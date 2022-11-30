package votepool

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/libs/sync"
	sm "github.com/tendermint/tendermint/state"
)

// Verifier will validate Votes by different policies.
type Verifier interface {
	Validate(vote Vote) error
}

// FromValidatorVerifier will check whether the Vote is from a valid validator.
type FromValidatorVerifier struct {
	state      sm.State
	mtx        *sync.RWMutex
	validators map[string]struct{}
}

func NewFromValidatorVerifier(stateDB sm.Store) (*FromValidatorVerifier, error) {
	state, err := stateDB.Load()
	if err != nil {
		return nil, fmt.Errorf("cannot load state: %w", err)
	}
	f := &FromValidatorVerifier{
		state: state,
		mtx:   &sync.RWMutex{},
	}
	f.updateValidators()

	return f, err
}

func (f *FromValidatorVerifier) updateValidators() {
	m := make(map[string]struct{})
	validators := f.state.Validators.Validators
	for _, v := range validators {
		m[v.BlsPubKey.Address().String()] = struct{}{}
	}
	f.validators = m
}

// Validate implements Verifier.
func (f *FromValidatorVerifier) Validate(vote Vote) error {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	if _, ok := f.validators[bls12381.PubKey(vote.PubKey).Address().String()]; ok {
		return nil
	}
	// if we cannot find the validator, will try to re-read the store.
	f.updateValidators()
	if _, ok := f.validators[bls12381.PubKey(vote.PubKey).Address().String()]; ok {
		return nil
	}
	return errors.New("vote is not from validators")
}

// BlsSignatureVerifier will check whether the Vote is correctly bls signed.
type BlsSignatureVerifier struct {
}

// Validate implements Verifier.
func (b *BlsSignatureVerifier) Validate(vote Vote) error {
	pubKey := bls12381.PubKey(vote.PubKey)
	hash := common.HexToHash(vote.EventHash)
	valid := pubKey.VerifySignature(hash.Bytes(), vote.Signature)
	if !valid {
		return errors.New("invalid signature")
	}
	return nil
}
