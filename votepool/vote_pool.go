package votepool

import (
	"context"
	"errors"
	"fmt"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/libs/sync"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

const (

	// The number of cached votes (i.e., keys) to quickly filter out when adding votes.
	cacheVoteSize = 1024

	// Vote will assign the expired at time when adding to Pool.
	voteKeepAliveAfter = time.Second * 30

	// Votes in the Pool will be pruned periodically to remove useless ones.
	pruneVoteInterval = 5 * time.Second

	// Defines the channel size for event bus subscription.
	eventBusSubscribeCap = 200

	// The event type of adding new votes to the Pool successfully.
	eventBusVotePoolUpdates = "votePoolUpdates"
)

// Pool is used for pooling cross chain, challenge votes from different validators/relayers.
// Votes in the pool will be pruned based on Vote's expiredTime.
type Pool interface {
	// AddVote will add a vote to the pool. Different validations can be conducted before adding.
	AddVote(vote *Vote) error

	// GetVotesByEventHash will filter votes by event hash and event type.
	GetVotesByEventHash(eventType EventType, eventHash []byte) ([]*Vote, error)

	// GetVotesByEventType will filter votes by event type.
	GetVotesByEventType(eventType EventType) ([]*Vote, error)

	// FlushVotes will clear all votes in the pool, no matter what kinds of events.
	FlushVotes()
}

// voteStore stores one kind of votes.
type voteStore struct {
	eventType EventType                   // event type
	mtx       *sync.RWMutex               // mutex for concurrency access of voteMap and others
	voteMap   map[string]map[string]*Vote // map: eventHash -> pubKey -> Vote

	queue *VoteQueue // priority queue for prune votes

}

// newVoteStore creates a vote store to store votes.
func newVoteStore(eventType EventType) *voteStore {

	s := &voteStore{
		eventType: eventType,
		mtx:       &sync.RWMutex{},
		voteMap:   make(map[string]map[string]*Vote),
		queue:     NewVoteQueue(),
	}
	return s
}

// Be noted: no validation is conducted in this layer.
func (s *voteStore) addVote(vote *Vote) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	subM, ok := s.voteMap[string(vote.EventHash[:])]
	if ok {
		if _, ok := subM[string(vote.PubKey[:])]; ok {
			return nil
		}
		subM[string(vote.PubKey[:])] = vote
		s.queue.Insert(vote)
		return nil
	}

	subM = make(map[string]*Vote)
	subM[string(vote.PubKey[:])] = vote
	s.voteMap[string(vote.EventHash[:])] = subM
	s.queue.Insert(vote)
	return nil
}

func (s *voteStore) getVotesByEventHash(eventType EventType, eventHash []byte) ([]*Vote, error) {
	if eventType != s.eventType {
		return nil, errors.New("invalid event type")
	}
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	votes := make([]*Vote, 0)
	if subM, ok := s.voteMap[string(eventHash[:])]; ok {
		for _, v := range subM {
			votes = append(votes, v)
		}
	}
	return votes, nil
}

func (s *voteStore) getVotesByEventType(eventType EventType) ([]*Vote, error) {
	if eventType != s.eventType {
		return nil, errors.New("invalid event type")
	}
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	votes := make([]*Vote, 0)
	for _, subM := range s.voteMap {
		for _, v := range subM {
			votes = append(votes, v)
		}
	}
	return votes, nil
}

func (s *voteStore) flushVotes() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.voteMap = make(map[string]map[string]*Vote)
}

func (s *voteStore) pruneVotes() []string {
	keys := make([]string, 0)
	current := &Vote{expireAt: time.Now()}
	s.mtx.Lock()
	defer s.mtx.Unlock()

	expires, err := s.queue.PopUntil(current)
	if err == nil {
		for _, expire := range expires {
			keys = append(keys, expire.Key())
			delete(s.voteMap[string(expire.EventHash[:])], string(expire.PubKey[:]))
		}
	}
	return keys
}

// pool provides a vote pool to store different kinds of vote.
// Meanwhile, it will check the source signer of the votes, currently only votes from validators will be saved.
type pool struct {
	service.BaseService

	stores map[EventType]*voteStore // each event type will have a store
	ticker *time.Ticker             // prune ticker

	blsVerifier       *BlsSignatureVerifier  // verify a vote's signature
	validatorVerifier *FromValidatorVerifier // verify a vote is from a validator

	cache *lru.Cache // to cache recent added votes' keys

	eventBus *types.EventBus // to subscribe validator update events and publish new added vote events
}

// NewVotePool creates a Pool for usage, which is only used for cross chain votes currently.
func NewVotePool(stateDB sm.Store, eventBus *types.EventBus) (*pool, error) {
	// get the initial validators
	state, err := stateDB.Load()
	if err != nil {
		return nil, fmt.Errorf("cannot load state: %w", err)
	}
	validatorVerifier := NewFromValidatorVerifier()
	validatorVerifier.initValidators(state.Validators.Validators)

	eventTypes := make([]EventType, 0)
	eventTypes = append(eventTypes, ToBscCrossChainEvent, FromBscCrossChainEvent)

	ticker := time.NewTicker(pruneVoteInterval)
	m := make(map[EventType]*voteStore, len(eventTypes))
	for _, et := range eventTypes {
		store := newVoteStore(et)
		m[et] = store
	}

	cache, err := lru.New(cacheVoteSize)
	if err != nil {
		panic(err)
	}
	votePool := &pool{
		stores:            m,
		ticker:            ticker,
		cache:             cache,
		eventBus:          eventBus,
		blsVerifier:       &BlsSignatureVerifier{},
		validatorVerifier: validatorVerifier,
	}
	go votePool.validatorUpdateRoutine()
	go votePool.pruneRoutine()
	return votePool, nil
}

// AddVote implements Pool.
func (p *pool) AddVote(vote *Vote) error {
	err := vote.ValidateBasic()
	if err != nil {
		return err
	}
	store, ok := p.stores[vote.EvenType]
	if !ok {
		return errors.New("unsupported event type")
	}

	if ok := p.cache.Contains(vote.Key()); ok {
		return nil
	}

	if err := p.validatorVerifier.Validate(*vote); err != nil {
		return err
	}
	if err := p.blsVerifier.Validate(*vote); err != nil {
		return err
	}

	vote.expireAt = time.Now().Add(voteKeepAliveAfter)
	err = store.addVote(vote)
	if err != nil {
		return err
	}
	err = p.eventBus.Publish(eventBusVotePoolUpdates, *vote)
	if err != nil {
		p.Logger.Error("Cannot publish vote to vote pool", "err", err.Error())
	}
	p.cache.Add(vote.Key(), struct{}{})
	return nil
}

// GetVotesByEventHash implements Pool.
func (p *pool) GetVotesByEventHash(eventType EventType, eventHash []byte) ([]*Vote, error) {
	store, ok := p.stores[eventType]
	if !ok {
		return nil, errors.New("unsupported event type")
	}
	return store.getVotesByEventHash(eventType, eventHash)
}

// GetVotesByEventType implements Pool.
func (p *pool) GetVotesByEventType(eventType EventType) ([]*Vote, error) {
	store, ok := p.stores[eventType]
	if !ok {
		return nil, errors.New("unsupported event type")
	}
	return store.getVotesByEventType(eventType)
}

// FlushVotes implements Pool.
func (p *pool) FlushVotes() {
	for _, singleVotePool := range p.stores {
		singleVotePool.flushVotes()
	}
	p.cache.Purge()
}

// validatorUpdateRoutine will sync validator updates.
func (p *pool) validatorUpdateRoutine() {
	sub, err := p.eventBus.Subscribe(context.Background(), "VotePoolService", types.EventQueryValidatorSetUpdates, eventBusSubscribeCap)
	if err != nil {
		panic(err)
	}
	for {
		select {
		case validatorData := <-sub.Out():
			changes := validatorData.Data().(types.EventDataValidatorSetUpdates)
			p.validatorVerifier.updateValidators(changes.ValidatorUpdates)
		case <-sub.Cancelled():
			return
		}
	}
}

// prune will prune votes at the given intervals.
func (p *pool) pruneRoutine() {
	for range p.ticker.C {
		for _, s := range p.stores {
			keys := s.pruneVotes()
			for _, key := range keys {
				p.cache.Remove(key)
			}
		}
	}
}
