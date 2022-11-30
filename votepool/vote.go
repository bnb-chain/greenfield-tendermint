package votepool

import (
	"errors"
	"time"
)

// Vote stands for votes from differently relayers/validators, to agree/disagree on something (e.g., cross chain).
type Vote struct {
	PubKey    []byte    `json:"pub_key"`   //bls public key
	Signature []byte    `json:"signature"` //bls signature
	EvenType  EventType `json:"event_type"`
	EventHash string    `json:"event_hash"` //0x-prefixed hash

	expireAt time.Time
}

func NewVote(pubKey, signature []byte, eventType uint8, eventHash string) *Vote {
	vote := Vote{
		PubKey:    pubKey,
		Signature: signature,
		EvenType:  EventType(eventType),
		EventHash: eventHash,
	}
	return &vote
}

// Key is used as an identifier of a vote, it is usually used as the key of map of cache.
func (v *Vote) Key() string {
	return v.EventHash + string(v.PubKey[:])
}

// ValidateBasic does basic validation of vote.
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
