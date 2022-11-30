package votepool

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	blsCommon "github.com/prysmaticlabs/prysm/crypto/bls/common"
	"github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/types"
)

// Verifier will validate Votes by different policies.
type Verifier interface {
	Validate(vote Vote) error
}

// FromValidatorVerifier will check whether the Vote is from a valid validator.
type FromValidatorVerifier struct {
	mtx        *sync.RWMutex
	validators map[string]*types.Validator
}

func NewFromValidatorVerifier() *FromValidatorVerifier {
	f := &FromValidatorVerifier{
		validators: make(map[string]*types.Validator),
		mtx:        &sync.RWMutex{},
	}
	return f
}

func (f *FromValidatorVerifier) initValidators(validators []*types.Validator) {
	for _, val := range validators {
		if len(val.BlsPubKey) > 0 {
			f.validators[string(val.BlsPubKey[:])] = val
		}
	}
}

func (f *FromValidatorVerifier) updateValidators(changes []*types.Validator) {
	f.mtx.Lock()
	f.mtx.Unlock()

	vals := make([]*types.Validator, 0)
	for k, val := range f.validators {
		vals = append(vals, val)
		delete(f.validators, k)
	}
	valSet := &types.ValidatorSet{Validators: vals}
	if err := valSet.UpdateWithChangeSet(changes); err != nil {
		for _, val := range valSet.Validators {
			if len(val.BlsPubKey) > 0 {
				f.validators[string(val.BlsPubKey[:])] = val
			}
		}
	}
}

// Validate implements Verifier.
func (f *FromValidatorVerifier) Validate(vote Vote) error {
	f.mtx.RLock()
	defer f.mtx.RUnlock()

	if _, ok := f.validators[string(vote.PubKey[:])]; ok {
		return nil
	}
	return errors.New("vote is not from validators")
}

// BlsSignatureVerifier will check whether the Vote is correctly bls signed.
type BlsSignatureVerifier struct {
}

// Validate implements Verifier.
func (b *BlsSignatureVerifier) Validate(vote Vote) error {
	hash := common.HexToHash(vote.EventHash)
	valid := verifySignature(hash.Bytes(), vote.PubKey, vote.Signature)
	if !valid {
		return errors.New("invalid signature")
	}
	return nil
}

func verifySignature(msg []byte, pubKey, sig []byte) bool {
	blsPubKey, err := blst.PublicKeyFromBytes(pubKey)
	if err != nil {
		return false
	}
	sigs := make([][]byte, 1)
	msgs := make([][32]byte, 1)
	pubKeys := make([]blsCommon.PublicKey, 1)
	sigs[0] = sig
	copy(msgs[0][:], msg)
	pubKeys[0] = blsPubKey

	valid, err := blst.VerifyMultipleSignatures(sigs, msgs, pubKeys)
	if err != nil {
		return false
	}
	return valid
}
