package bls12381

import (
	"bytes"
	"crypto/subtle"
	"fmt"

	"github.com/prysmaticlabs/prysm/crypto/bls/blst"
	"github.com/prysmaticlabs/prysm/crypto/bls/common"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	tmjson "github.com/tendermint/tendermint/libs/json"
)

const (
	PrivKeyName = "tendermint/PrivKeyBls12381"
	PubKeyName  = "tendermint/PubKeyBls12381"
	// PubKeySize is the size, in bytes, of public keys as used in this package.
	PubKeySize = 48
	// SignatureSize is the size, in bytes, of signatures as used in this package.
	SignatureSize = 96

	KeyType = "bls12"
)

func init() {
	tmjson.RegisterType(PubKey{}, PubKeyName)
	tmjson.RegisterType(PrivKey{}, PrivKeyName)
}

// PrivKey implements crypto.PrivKey.
type PrivKey []byte

// Bytes returns the privkey byte format.
func (privKey PrivKey) Bytes() []byte {
	return []byte(privKey)
}

// Sign produces a signature on the provided message.
func (privKey PrivKey) Sign(msg []byte) ([]byte, error) {
	pk, err := blst.SecretKeyFromBytes(privKey)
	if err != nil {
		return nil, err
	}
	signature := pk.Sign(msg)
	return signature.Marshal(), nil
}

// PubKey gets the corresponding public key from the private key.
func (privKey PrivKey) PubKey() crypto.PubKey {
	secretKey, err := blst.SecretKeyFromBytes(privKey)
	if err != nil {
		return nil
	}
	return PubKey(secretKey.PublicKey().Marshal())
}

// Equals - you probably don't need to use this.
// Runs in constant time based on length of the keys.
func (privKey PrivKey) Equals(other crypto.PrivKey) bool {
	if otherPk, ok := other.(PrivKey); ok {
		return subtle.ConstantTimeCompare(privKey[:], otherPk[:]) == 1
	}

	return false
}

func (privKey PrivKey) Type() string {
	return KeyType
}

// GenPrivKey generates a new bls secret key.
func GenPrivKey() PrivKey {
	pk, err := blst.RandKey()
	if err != nil {
		return nil
	}
	return pk.Marshal()
}

//-------------------------------------

var _ crypto.PubKey = PubKey{}

// PubKey implements crypto.PubKey for BLS.
type PubKey []byte

// Address is the SHA256-20 of the raw pubkey bytes.
func (pubKey PubKey) Address() crypto.Address {
	if len(pubKey) != PubKeySize {
		panic("pubkey is incorrect size")
	}
	return crypto.Address(tmhash.SumTruncated(pubKey))
}

// Bytes returns the PubKey byte format.
func (pubKey PubKey) Bytes() []byte {
	return []byte(pubKey)
}

func (pubKey PubKey) VerifySignature(msg []byte, sig []byte) bool {
	// make sure we use the same algorithm to sign
	if len(sig) != SignatureSize {
		return false
	}

	blsPubKey, err := blst.PublicKeyFromBytes(pubKey)
	if err != nil {
		return false
	}
	sigs := make([][]byte, 1)
	msgs := make([][32]byte, 1)
	pubKeys := make([]common.PublicKey, 1)
	sigs[0] = sig
	copy(msgs[0][:], msg)
	pubKeys[0] = blsPubKey

	valid, err := blst.VerifyMultipleSignatures(sigs, msgs, pubKeys)
	if err != nil {
		return false
	}
	return valid
}

func (pubKey PubKey) String() string {
	return fmt.Sprintf("PubKeyBLS{%X}", []byte(pubKey))
}

func (pubKey PubKey) Type() string {
	return KeyType
}

func (pubKey PubKey) Equals(other crypto.PubKey) bool {
	if otherPk, ok := other.(PubKey); ok {
		return bytes.Equal(pubKey[:], otherPk[:])
	}

	return false
}
