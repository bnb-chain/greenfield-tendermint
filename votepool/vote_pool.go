package votepool

import (
	"errors"
	"time"

	"github.com/tendermint/tendermint/libs/sync"
	sm "github.com/tendermint/tendermint/state"
)

// Pool is used for pooling cross chain, challenge votes from different validators/relayers.
// Votes in the pool will be pruned based on Vote's expiredTime.
type Pool interface {
	// AddVote will add a vote to the pool. Different validations can be conducted before adding.
	AddVote(vote *Vote) error

	// GetVotesByEventHash will filter votes by event hash and event type.
	GetVotesByEventHash(eventType EventType, eventHash string) ([]*Vote, error)

	// GetVotesByEventType will filter votes by event type.
	GetVotesByEventType(eventType EventType) ([]*Vote, error)

	// FlushVotes will clear all votes in the pool, no matter what kinds of events.
	FlushVotes()

	// SubscribeVotes will provide a channel to consume new added votes.
	SubscribeVotes() <-chan *Vote
}

const PruneVoteInterval = 5 * time.Second

// singlePool stores one kind of votes.
type singlePool struct {
	eventType EventType                   // event type
	mtx       *sync.RWMutex               // mutex to protect voteMap
	voteMap   map[string]map[string]*Vote // map: eventHash -> pubKey -> Vote
	ch        chan *Vote                  // channel for subscription

	queue  *VoteQueue // priority to prune votes
	ticker *time.Ticker
}

// newSinglePool creates a basic vote pool.
func newSinglePool(eventType EventType, ch chan *Vote) *singlePool {
	ticker := time.NewTicker(PruneVoteInterval)

	p := &singlePool{
		eventType: eventType,
		mtx:       &sync.RWMutex{},
		voteMap:   make(map[string]map[string]*Vote),
		ch:        ch,
		queue:     NewVoteQueue(),
		ticker:    ticker,
	}
	go p.prune()
	return p
}

// AddVote implements Pool.
// Be noted: no validation is conducted in this layer.
func (s *singlePool) AddVote(vote *Vote) error {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	subM, ok := s.voteMap[vote.EventHash]
	if ok {
		if _, ok := subM[string(vote.PubKey[:])]; ok {
			return errors.New("vote already exists")
		}
		subM[string(vote.PubKey[:])] = vote
		s.queue.Insert(vote)
		return nil
	}

	subM = make(map[string]*Vote)
	subM[string(vote.PubKey[:])] = vote
	s.voteMap[vote.EventHash] = subM
	s.queue.Insert(vote)
	go func() {
		s.ch <- vote
	}()
	return nil
}

// GetVotesByEventHash implements Pool.
func (s *singlePool) GetVotesByEventHash(eventType EventType, eventHash string) ([]*Vote, error) {
	if eventType != s.eventType {
		return nil, errors.New("invalid event type")
	}
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	votes := make([]*Vote, 0)
	if subM, ok := s.voteMap[eventHash]; ok {
		for _, v := range subM {
			votes = append(votes, v)
		}
	}
	return votes, nil
}

// GetVotesByEventType implements Pool.
func (s *singlePool) GetVotesByEventType(eventType EventType) ([]*Vote, error) {
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

// FlushVotes implements Pool.
func (s *singlePool) FlushVotes() {
	s.mtx.Lock()
	defer s.mtx.Unlock()

	s.voteMap = make(map[string]map[string]*Vote)
}

// SubscribeVotes implements Pool.
func (s *singlePool) SubscribeVotes() <-chan *Vote {
	return s.ch
}

// prune will prune votes at the given intervals.
func (s *singlePool) prune() {
	for range s.ticker.C {
		current := &Vote{expireAt: time.Now()}
		s.mtx.Lock()
		expires, err := s.queue.PopUntil(current)
		if err == nil {
			for _, expire := range expires {
				delete(s.voteMap[expire.EventHash], string(expire.PubKey[:]))
			}

		}
		s.mtx.Unlock()
	}
}

// pool provides a vote pool to store different kinds of vote.
// Meanwhile, it will check the source signer of the votes, currently only votes from validators will be saved.
type pool struct {
	singlePools map[EventType]*singlePool
	ch          chan *Vote
	verifiers   []Verifier
}

// NewMultiVotePool creates a Pool for usage, which is only used for cross chain votes currently.
func NewMultiVotePool(stateDB sm.Store) (Pool, error) {
	eventTypes := make([]EventType, 0)
	eventTypes = append(eventTypes, ToBscCrossChainEvent, FromBscCrossChainEvent)

	ch := make(chan *Vote)
	m := make(map[EventType]*singlePool, len(eventTypes))
	for _, et := range eventTypes {
		singleVp := newSinglePool(et, ch)
		m[et] = singleVp
	}
	votePool := &pool{
		singlePools: m,
		ch:          ch,
		verifiers:   make([]Verifier, 0),
	}

	blsVerifier := &BlsSignatureVerifier{}
	fromValVerifier, err := NewFromValidatorVerifier(stateDB)
	if err != nil {
		return nil, err
	}
	votePool.AddVerifier(blsVerifier)
	votePool.AddVerifier(fromValVerifier)

	return votePool, nil
}

// AddVote implements Pool.
func (m *pool) AddVote(vote *Vote) error {
	err := vote.ValidateBasic()
	if err != nil {
		return err
	}
	singleVotePool, ok := m.singlePools[vote.EvenType]
	if !ok {
		return errors.New("unsupported event type")
	}

	//for _, verifier := range m.verifiers {
	//	if err = verifier.Validate(*vote); err != nil {
	//		return err
	//	}
	//}

	return singleVotePool.AddVote(vote)
}

// GetVotesByEventHash implements Pool.
func (m *pool) GetVotesByEventHash(eventType EventType, eventHash string) ([]*Vote, error) {
	singleVotePool, ok := m.singlePools[eventType]
	if !ok {
		return nil, errors.New("unsupported event type")
	}
	return singleVotePool.GetVotesByEventHash(eventType, eventHash)
}

// GetVotesByEventType implements Pool.
func (m *pool) GetVotesByEventType(eventType EventType) ([]*Vote, error) {
	singleVotePool, ok := m.singlePools[eventType]
	if !ok {
		return nil, errors.New("unsupported event type")
	}
	return singleVotePool.GetVotesByEventType(eventType)
}

// FlushVotes implements Pool.
func (m *pool) FlushVotes() {
	for _, singleVotePool := range m.singlePools {
		singleVotePool.FlushVotes()
	}
}

// SubscribeVotes implements Pool.
func (m *pool) SubscribeVotes() <-chan *Vote {
	return m.ch
}

// AddVerifier will append a Vote verifier.
func (m *pool) AddVerifier(v Verifier) {
	m.verifiers = append(m.verifiers, v)
}

// ClearVerifier will clear all verifiers for the Pool.
func (m *pool) ClearVerifier() {
	m.verifiers = make([]Verifier, 0)
}
