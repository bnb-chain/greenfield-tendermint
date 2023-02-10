package bls12381_test

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/crypto/bls12381"

	"github.com/tendermint/tendermint/crypto"
)

func TestSignAndValidateBls12381(t *testing.T) {
	privKey := bls12381.GenPrivKey()
	pubKey := privKey.PubKey()

	msg := crypto.CRandBytes(32)
	sig, err := privKey.Sign(msg)
	require.Nil(t, err)

	// Test the signature
	assert.True(t, pubKey.VerifySignature(msg, sig))

	// Mutate the signature, just one bit.
	sig[7] ^= byte(0x01)

	assert.False(t, pubKey.VerifySignature(msg, sig))
}

func TestBls12381(t *testing.T) {
	privBytes, err := base64.StdEncoding.DecodeString("G9gkMFBeK41WeO7Fyjrs49wD5X2hEyY6CNEE/VN5jKE=")
	if err != nil {
		panic(err)
	}
	pubBytes, err := base64.StdEncoding.DecodeString("syD7qHdpRNj8ri35A+RyFL1ERPKsT8V8NNn4wNTEZDH2ZPJ0RHBLdgUveqZGJRci")
	if err != nil {
		panic(err)
	}

	privKey := bls12381.PrivKey(privBytes)
	pubKey := bls12381.PubKey(pubBytes)

	fmt.Println(privKey)
	fmt.Println(pubKey)

	msg, err := hex.DecodeString("2b08011101000000000000002a0b08f2fc969f061090d9c3203211746573742d636861696e2d5468324f3071")
	if err != nil {
		panic(err)
	}
	sig, err := hex.DecodeString("9312c6bfdffed60aee8b88839365e84d37c04200e07fb4cf5fa4dc87966dc1e8fcfc194d443e84cf47d2c8cae4bbbe7e19a971cc53244e7b714bb05cef8a61b35be7a8e912aa991a406330475e691003fb9b42783e87945d2b18f5a0181e42fd")
	if err != nil {
		panic(err)
	}

	valid := pubKey.VerifySignature(msg, sig)
	fmt.Println(valid)
}
