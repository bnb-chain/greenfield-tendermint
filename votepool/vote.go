package votepool

import (
	"errors"
	"time"
)

const VoteToExpireTime = time.Second * 30

// Vote stands for votes from differently relayers/validators, to agree/disagree on something (e.g., cross chain).
type Vote struct {
	PubKey    []byte    `json:"pub_key"`
	Signature []byte    `json:"signature"`
	EvenType  EventType `json:"event_type"`
	EventHash string    `json:"event_hash"`

	expireAt time.Time
}

func NewVote(pubKey, signature []byte, eventType uint8, eventHash string) *Vote {
	vote := Vote{
		PubKey:    pubKey,
		Signature: signature,
		EvenType:  EventType(eventType),
		EventHash: eventHash,
		expireAt:  time.Now().Add(VoteToExpireTime),
	}
	return &vote
}

func (v *Vote) Key() string {
	return v.EventHash + string(v.PubKey[:])
}

func (v *Vote) ValidateBasic() error {
	//TODO: verify exact lengths
	if len(v.PubKey) == 0 {
		return errors.New("invalid public key")
	}
	if len(v.Signature) == 0 {
		return errors.New("invalid signature")
	}
	if len(v.EventHash) == 0 {
		return errors.New("invalid event hash")
	}
	if !v.expireAt.IsZero() && v.expireAt.Before(time.Now()) {
		return errors.New("vote is expired")
	}
	return nil
}
