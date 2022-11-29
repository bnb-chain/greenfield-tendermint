package types

import (
	"fmt"

	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/ed25519"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/crypto/secp256k1"
)

func Ed25519ValidatorUpdate(pk []byte, power int64, blsPk []byte) ValidatorUpdate {
	pke := ed25519.PubKey(pk)
	pkp, err := cryptoenc.PubKeyToProto(pke)
	if err != nil {
		panic(err)
	}

	pkBls := bls12381.PubKey(blsPk)
	pkpBls, err := cryptoenc.BlsPubKeyToProto(pkBls)
	if err != nil {
		panic(err)
	}

	return ValidatorUpdate{
		// Address:
		PubKey:    pkp,
		BlsPubKey: &pkpBls,
		Power:     power,
	}
}

func Secp256k1ValidatorUpdate(pk []byte, power int64, blsPk []byte) ValidatorUpdate {
	pke := secp256k1.PubKey(pk)
	pkp, err := cryptoenc.PubKeyToProto(pke)
	if err != nil {
		panic(err)
	}

	pkBls := bls12381.PubKey(blsPk)
	pkpBls, err := cryptoenc.BlsPubKeyToProto(pkBls)
	if err != nil {
		panic(err)
	}

	return ValidatorUpdate{
		// Address:
		PubKey:    pkp,
		BlsPubKey: &pkpBls,
		Power:     power,
	}
}

func UpdateValidator(pk []byte, power int64, keyType string, blsPk []byte) ValidatorUpdate {
	switch keyType {
	case "", ed25519.KeyType:
		return Ed25519ValidatorUpdate(pk, power, blsPk)
	case secp256k1.KeyType:
		return Secp256k1ValidatorUpdate(pk, power, blsPk)
	default:
		panic(fmt.Sprintf("key type %s not supported", keyType))
	}
}
