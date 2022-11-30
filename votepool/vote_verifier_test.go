package votepool

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/types"
)

func TestVoteFromValidatorVerifier(t *testing.T) {
	pubKey1 := ed25519.GenPrivKey().PubKey()
	blsPubKey1 := bls12381.GenPrivKey().PubKey()
	val1 := &types.Validator{Address: pubKey1.Address(), PubKey: pubKey1, BlsPubKey: blsPubKey1, VotingPower: 10}

	pubKey2 := ed25519.GenPrivKey().PubKey()
	blsPubKey2 := bls12381.GenPrivKey().PubKey()
	val2 := &types.Validator{Address: pubKey2.Address(), PubKey: pubKey2, BlsPubKey: blsPubKey2, VotingPower: 10}

	mockStore := mockStoreDB{validators: []*types.Validator{val1, val2}}
	verifier, err := NewFromValidatorVerifier(mockStore)
	require.NoError(t, err)

	voteFromVal1 := Vote{PubKey: blsPubKey1.Bytes()}
	err = verifier.Validate(voteFromVal1)
	require.NoError(t, err)

	voteFromOthers := Vote{PubKey: bls12381.GenPrivKey().PubKey().Bytes()}
	err = verifier.Validate(voteFromOthers)
	require.Error(t, err)

}

func TestVoteBlsVerifier(t *testing.T) {
	privKey := bls12381.GenPrivKey()
	pubKey := privKey.PubKey()
	eventHash := "0xeefacfed87736ae1d8e8640f6fd7951862997782e5e79842557923e2779d5d5a"
	sign, err := privKey.Sign(common.HexToHash(eventHash).Bytes())
	require.NoError(t, err)

	verifier := &BlsSignatureVerifier{}

	vote1 := Vote{
		PubKey:    pubKey.Bytes(),
		Signature: sign,
		EvenType:  0,
		EventHash: eventHash,
		expireAt:  time.Time{},
	}
	err = verifier.Validate(vote1)
	require.NoError(t, err)

	vote2 := Vote{
		PubKey:    pubKey.Bytes(),
		Signature: sign,
		EvenType:  0,
		EventHash: "0xb3989c2ba4b4b91b35162c137c154848f7261e16ce3f6d8c88f64cf06b737a3c",
		expireAt:  time.Time{},
	}
	err = verifier.Validate(vote2)
	require.Error(t, err)
}
