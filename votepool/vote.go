package votepool

import (
	"errors"
	"time"
)

// Vote stands for votes from differently relayers/validators, to agree/disagree on something (e.g., cross chain).
type Vote struct {
	PubKey    []byte    `json:"pub_key"`    //bls public key
	Signature []byte    `json:"signature"`  //bls signature
	EvenType  EventType `json:"event_type"` //event type of the vote
	EventHash []byte    `json:"event_hash"` //the hash to sign, here []byte is used, so that vote pool will not care about the meaning of the data

	expireAt time.Time
}

func NewVote(pubKey, signature []byte, eventType uint8, eventHash []byte) *Vote {
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
	return string(v.EventHash[:]) + string(v.PubKey[:])
}

// ValidateBasic does basic validation of vote.
func (v *Vote) ValidateBasic() error {
	if len(v.EventHash) != 32 {
		return errors.New("invalid event hash")
	}
	if v.EvenType != ToBscCrossChainEvent && v.EvenType != FromBscCrossChainEvent {
		return errors.New("invalid event type")
	}
	if len(v.PubKey) != 48 {
		return errors.New("invalid public key")
	}
	if len(v.Signature) != 96 {
		return errors.New("invalid signature")
	}
	if !v.expireAt.IsZero() && v.expireAt.Before(time.Now()) {
		return errors.New("vote is expired")
	}
	return nil
}
