package types

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/tendermint/tendermint/crypto"
	ce "github.com/tendermint/tendermint/crypto/encoding"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
)

// Volatile state for each Validator
// NOTE: The ProposerPriority is not included in Validator.Hash();
// make sure to update that method if changes are made here
type Validator struct {
	Address          Address       `json:"address"`
	PubKey           crypto.PubKey `json:"pub_key"`
	BlsPubKey        crypto.PubKey `json:"bls_pub_key"`
	Relayer          string        `json:"relayer"`
	VotingPower      int64         `json:"voting_power"`
	ProposerPriority int64         `json:"proposer_priority"`
}

// NewValidator returns a new validator with the given pubkey and voting power.
func NewValidator(pubKey crypto.PubKey, blsPubKey crypto.PubKey, votingPower int64, relayer string) *Validator {
	return &Validator{
		Address:          pubKey.Address(),
		PubKey:           pubKey,
		BlsPubKey:        blsPubKey,
		Relayer:          relayer,
		VotingPower:      votingPower,
		ProposerPriority: 0,
	}
}

// ValidateBasic performs basic validation.
func (v *Validator) ValidateBasic() error {
	if v == nil {
		return errors.New("nil validator")
	}
	if v.PubKey == nil {
		return errors.New("validator does not have a public key")
	}
	if v.BlsPubKey == nil {
		return errors.New("validator does not have a bls public key")
	}

	if v.VotingPower < 0 {
		return errors.New("validator has negative voting power")
	}

	if len(v.Address) != crypto.AddressSize {
		return fmt.Errorf("validator address is the wrong size: %v", v.Address)
	}

	return nil
}

// Creates a new copy of the validator so we can mutate ProposerPriority.
// Panics if the validator is nil.
func (v *Validator) Copy() *Validator {
	vCopy := *v
	return &vCopy
}

// Returns the one with higher ProposerPriority.
func (v *Validator) CompareProposerPriority(other *Validator) *Validator {
	if v == nil {
		return other
	}
	switch {
	case v.ProposerPriority > other.ProposerPriority:
		return v
	case v.ProposerPriority < other.ProposerPriority:
		return other
	default:
		result := bytes.Compare(v.Address, other.Address)
		switch {
		case result < 0:
			return v
		case result > 0:
			return other
		default:
			panic("Cannot compare identical validators")
		}
	}
}

// String returns a string representation of String.
//
// 1. address
// 2. public key
// 3. bls public key
// 4. voting power
// 5. proposer priority
// 6. relayer address
func (v *Validator) String() string {
	if v == nil {
		return "nil-Validator"
	}
	return fmt.Sprintf("Validator{%v %v %v VP:%v A:%v %v}",
		v.Address,
		v.PubKey,
		v.BlsPubKey,
		v.VotingPower,
		v.ProposerPriority,
		v.Relayer)
}

// ValidatorListString returns a prettified validator list for logging purposes.
func ValidatorListString(vals []*Validator) string {
	chunks := make([]string, len(vals))
	for i, val := range vals {
		chunks[i] = fmt.Sprintf("%s:%d", val.Address, val.VotingPower)
	}

	return strings.Join(chunks, ",")
}

// Bytes computes the unique encoding of a validator with a given voting power.
// These are the bytes that gets hashed in consensus. It excludes address
// as its redundant with the pubkey. This also excludes ProposerPriority
// which changes every round.
func (v *Validator) Bytes() []byte {
	pk, err := ce.PubKeyToProto(v.PubKey)
	if err != nil {
		panic(err)
	}

	blsPk, err := ce.BlsPubKeyToProto(v.BlsPubKey)
	if err != nil {
		panic(err)
	}

	pbv := tmproto.SimpleValidator{
		PubKey:      &pk,
		BlsPubKey:   &blsPk,
		Relayer:     v.Relayer,
		VotingPower: v.VotingPower,
	}

	bz, err := pbv.Marshal()
	if err != nil {
		panic(err)
	}
	return bz
}

// ToProto converts Valiator to protobuf
func (v *Validator) ToProto() (*tmproto.Validator, error) {
	if v == nil {
		return nil, errors.New("nil validator")
	}

	pk, err := ce.PubKeyToProto(v.PubKey)
	if err != nil {
		return nil, err
	}

	blsPk, err := ce.BlsPubKeyToProto(v.BlsPubKey)
	if err != nil {
		return nil, err
	}

	vp := tmproto.Validator{
		Address:          v.Address,
		PubKey:           pk,
		BlsPubKey:        blsPk,
		Relayer:          v.Relayer,
		VotingPower:      v.VotingPower,
		ProposerPriority: v.ProposerPriority,
	}

	return &vp, nil
}

// FromProto sets a protobuf Validator to the given pointer.
// It returns an error if the public key is invalid.
func ValidatorFromProto(vp *tmproto.Validator) (*Validator, error) {
	if vp == nil {
		return nil, errors.New("nil validator")
	}

	pk, err := ce.PubKeyFromProto(vp.PubKey)
	if err != nil {
		return nil, err
	}
	blsPk, err := ce.BlsPubKeyFromProto(vp.BlsPubKey)
	if err != nil {
		return nil, err
	}
	v := new(Validator)
	v.Address = vp.GetAddress()
	v.PubKey = pk
	v.BlsPubKey = blsPk
	v.Relayer = vp.GetRelayer()
	v.VotingPower = vp.GetVotingPower()
	v.ProposerPriority = vp.GetProposerPriority()

	return v, nil
}

//----------------------------------------
// RandValidator

// RandValidator returns a randomized validator, useful for testing.
// UNSTABLE
func RandValidator(randPower bool, minPower int64) (*Validator, PrivValidator) {
	privVal := NewMockPV()
	votePower := minPower
	if randPower {
		votePower += int64(tmrand.Uint32())
	}
	pubKey, err := privVal.GetPubKey()
	if err != nil {
		panic(fmt.Errorf("could not retrieve pubkey %w", err))
	}

	blsPubKey, err := privVal.GetBlsPubKey()
	if err != nil {
		panic(fmt.Errorf("could not generate bls secret key %w", err))
	}

	relayer := privVal.Relayer

	val := NewValidator(pubKey, blsPubKey, votePower, relayer)
	return val, privVal
}
