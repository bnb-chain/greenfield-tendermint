package votepool

import (
	"errors"

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

// singlePool stores one kind of votes.
type singlePool struct {
	eventType EventType                   // event type
	mtx       *sync.RWMutex               // mutex to protect voteMap
	voteMap   map[string]map[string]*Vote // map: eventHash -> pubKey -> Vote
	ch        chan *Vote                  // channel for subscription
}

// newSinglePool creates a basic vote pool.
func newSinglePool(eventType EventType, ch chan *Vote) *singlePool {
	return &singlePool{
		eventType: eventType,
		mtx:       &sync.RWMutex{},
		voteMap:   make(map[string]map[string]*Vote),
		ch:        ch,
	}
}

// AddVote implements Pool.
// Be noted: no validation is conducted in this layer.
func (s *singlePool) AddVote(vote *Vote) error {

	s.mtx.Lock()
	defer s.mtx.Unlock()

	subM, ok := s.voteMap[vote.EventHash]
	if ok {
		if _, ok := subM[string(vote.PubKey)]; ok {
			return errors.New("vote already exists")
		}
		subM[string(vote.PubKey)] = vote
		return nil
	}

	subM = make(map[string]*Vote)
	subM[string(vote.PubKey)] = vote
	s.voteMap[vote.EventHash] = subM
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
	s.mtx.RLock()
	defer s.mtx.RUnlock()

	s.voteMap = make(map[string]map[string]*Vote)
}

// SubscribeVotes implements Pool.
func (s *singlePool) SubscribeVotes() <-chan *Vote {
	return s.ch
}

// MultiPool provides a vote pool to store different kinds of vote.
// Meanwhile, it will check the source signer of the votes, currently only votes from validators will be saved.
type MultiPool struct {
	singleVps map[EventType]*singlePool
	ch        chan *Vote
	verifiers []Verifier
}

// NewMultiVotePool creates a Pool for usage, which is only used for cross chain votes currently.
func NewMultiVotePool(stateDB sm.Store) (Pool, error) {
	eventTypes := make([]EventType, 0)
	eventTypes = append(eventTypes, CrossChainEvent)

	ch := make(chan *Vote)
	m := make(map[EventType]*singlePool, len(eventTypes))
	for _, et := range eventTypes {
		singleVp := newSinglePool(et, ch)
		m[et] = singleVp
	}
	votePool := &MultiPool{
		singleVps: m,
		ch:        ch,
		verifiers: make([]Verifier, 0),
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
func (m *MultiPool) AddVote(vote *Vote) error {
	err := vote.ValidateBasic()
	if err != nil {
		return err
	}
	singleVotePool, ok := m.singleVps[vote.EvenType]
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
func (m *MultiPool) GetVotesByEventHash(eventType EventType, eventHash string) ([]*Vote, error) {
	singleVotePool, ok := m.singleVps[eventType]
	if !ok {
		return nil, errors.New("unsupported event type")
	}
	return singleVotePool.GetVotesByEventHash(eventType, eventHash)
}

// GetVotesByEventType implements Pool.
func (m *MultiPool) GetVotesByEventType(eventType EventType) ([]*Vote, error) {
	singleVotePool, ok := m.singleVps[eventType]
	if !ok {
		return nil, errors.New("unsupported event type")
	}
	return singleVotePool.GetVotesByEventType(eventType)
}

// FlushVotes implements Pool.
func (m *MultiPool) FlushVotes() {
	for _, singleVotePool := range m.singleVps {
		singleVotePool.FlushVotes()
	}
}

// SubscribeVotes implements Pool.
func (m *MultiPool) SubscribeVotes() <-chan *Vote {
	return m.ch
}

// AddVerifier will append a Vote verifier.
func (m *MultiPool) AddVerifier(v Verifier) {
	m.verifiers = append(m.verifiers, v)
}

// ClearVerifier will clear all verifiers for the Pool.
func (m *MultiPool) ClearVerifier() {
	m.verifiers = make([]Verifier, 0)
}
