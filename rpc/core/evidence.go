package core

import (
	"errors"
	"fmt"

	ctypes "github.com/bnb-chain/greenfield-tendermint/rpc/core/types"
	rpctypes "github.com/bnb-chain/greenfield-tendermint/rpc/jsonrpc/types"
	"github.com/bnb-chain/greenfield-tendermint/types"
)

// BroadcastEvidence broadcasts evidence of the misbehavior.
// More: https://docs.tendermint.com/v0.34/rpc/#/Info/broadcast_evidence
func BroadcastEvidence(ctx *rpctypes.Context, ev types.Evidence) (*ctypes.ResultBroadcastEvidence, error) {
	if ev == nil {
		return nil, errors.New("no evidence was provided")
	}

	if err := ev.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("evidence.ValidateBasic failed: %w", err)
	}

	if err := env.EvidencePool.AddEvidence(ev); err != nil {
		return nil, fmt.Errorf("failed to add evidence: %w", err)
	}
	return &ctypes.ResultBroadcastEvidence{Hash: ev.Hash()}, nil
}
