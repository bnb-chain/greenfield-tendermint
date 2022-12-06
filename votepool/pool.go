package votepool

import (
	"context"
	"errors"
	"time"

	lru "github.com/hashicorp/golang-lru"

	"github.com/tendermint/tendermint/libs/log"

	"github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/types"
)

const (

	// The number of cached votes (i.e., keys) to quickly filter out when adding votes.
	cacheVoteSize = 1024

	// Vote will assign the expired at time when adding to the Pool.
	voteKeepAliveAfter = time.Second * 30

	// Votes in the Pool will be pruned periodically to remove useless ones.
	pruneVoteInterval = 3 * time.Second

	// Defines the channel size for event bus subscription.
	eventBusSubscribeCap = 200

	// The event type of adding new votes to the Pool successfully.
	eventBusVotePoolUpdates = "votePoolUpdates"
)

// voteStore stores one type of votes.
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
	eventHashStr := string(vote.EventHash[:])
	pubKeyStr := string(vote.PubKey[:])
	s.mtx.Lock()
	defer s.mtx.Unlock()

	subM, ok := s.voteMap[eventHashStr]
	if ok {
		if _, ok := subM[pubKeyStr]; ok {
			return nil
		}
		subM[string(vote.PubKey[:])] = vote
		s.queue.Insert(vote)
		return nil
	}

	subM = make(map[string]*Vote)
	subM[pubKeyStr] = vote
	s.voteMap[eventHashStr] = subM
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

	if expires, err := s.queue.PopUntil(current); err == nil {
		for _, expire := range expires {
			keys = append(keys, expire.Key())
			delete(s.voteMap[string(expire.EventHash[:])], string(expire.PubKey[:]))
		}
	}
	return keys
}

// Pool implements VotePool to store different types of votes.
// Meanwhile, it will check the signature and source signer of a vote, only votes from validators will be saved.
type Pool struct {
	logger log.Logger

	stores map[EventType]*voteStore // each event type will have a store
	ticker *time.Ticker             // prune ticker

	blsVerifier       *BlsSignatureVerifier  // verify a vote's signature
	validatorVerifier *FromValidatorVerifier // verify a vote is from a validator

	cache *lru.Cache // to cache recent added votes' keys

	eventBus *types.EventBus // to subscribe validator update events and publish new added vote events
}

// NewVotePool creates a Pool, the init validators should be supplied.
func NewVotePool(logger log.Logger, validators []*types.Validator, eventBus *types.EventBus) (*Pool, error) {
	// only used for cross chain votes currently.
	eventTypes := []EventType{ToBscCrossChainEvent, FromBscCrossChainEvent}

	ticker := time.NewTicker(pruneVoteInterval)
	stores := make(map[EventType]*voteStore, len(eventTypes))
	for _, et := range eventTypes {
		store := newVoteStore(et)
		stores[et] = store
	}

	cache, err := lru.New(cacheVoteSize)
	if err != nil {
		panic(err)
	}

	// set the initial validators
	validatorVerifier := NewFromValidatorVerifier()
	validatorVerifier.initValidators(validators)
	votePool := &Pool{
		logger:            logger,
		stores:            stores,
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

// AddVote implements VotePool.
func (p *Pool) AddVote(vote *Vote) error {
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
		p.logger.Error("Cannot publish vote pool event", "err", err.Error())
	}
	p.cache.Add(vote.Key(), struct{}{})
	return nil
}

// GetVotesByEventHash implements VotePool.
func (p *Pool) GetVotesByEventHash(eventType EventType, eventHash []byte) ([]*Vote, error) {
	store, ok := p.stores[eventType]
	if !ok {
		return nil, errors.New("unsupported event type")
	}
	return store.getVotesByEventHash(eventType, eventHash)
}

// GetVotesByEventType implements VotePool.
func (p *Pool) GetVotesByEventType(eventType EventType) ([]*Vote, error) {
	store, ok := p.stores[eventType]
	if !ok {
		return nil, errors.New("unsupported event type")
	}
	return store.getVotesByEventType(eventType)
}

// FlushVotes implements VotePool.
func (p *Pool) FlushVotes() {
	for _, singleVotePool := range p.stores {
		singleVotePool.flushVotes()
	}
	p.cache.Purge()
}

// validatorUpdateRoutine will sync validator updates.
func (p *Pool) validatorUpdateRoutine() {
	sub, err := p.eventBus.Subscribe(context.Background(), "VotePoolService", types.EventQueryValidatorSetUpdates, eventBusSubscribeCap)
	if err != nil {
		panic(err)
	}
	for {
		select {
		case validatorData := <-sub.Out():
			changes := validatorData.Data().(types.EventDataValidatorSetUpdates)
			p.validatorVerifier.updateValidators(changes.ValidatorUpdates)
			p.logger.Info("Validators updated", "changes", changes.ValidatorUpdates)
		case <-sub.Cancelled():
			return
		}
	}
}

// prune will prune votes at the given intervals.
func (p *Pool) pruneRoutine() {
	for range p.ticker.C {
		for _, s := range p.stores {
			keys := s.pruneVotes()
			for _, key := range keys {
				p.cache.Remove(key)
			}
		}
	}
}
