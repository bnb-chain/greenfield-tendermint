package core

import (
	abci "github.com/bnb-chain/greenfield-tendermint/abci/types"
	"github.com/bnb-chain/greenfield-tendermint/libs/bytes"
	"github.com/bnb-chain/greenfield-tendermint/proxy"
	ctypes "github.com/bnb-chain/greenfield-tendermint/rpc/core/types"
	rpctypes "github.com/bnb-chain/greenfield-tendermint/rpc/jsonrpc/types"
)

// ABCIQuery queries the application for some information.
// More: https://docs.tendermint.com/v0.34/rpc/#/ABCI/abci_query
func ABCIQuery(
	ctx *rpctypes.Context,
	path string,
	data bytes.HexBytes,
	height int64,
	prove bool,
) (*ctypes.ResultABCIQuery, error) {
	resQuery, err := env.ProxyAppQuery.QuerySync(abci.RequestQuery{
		Path:   path,
		Data:   data,
		Height: height,
		Prove:  prove,
	})
	if err != nil {
		return nil, err
	}

	return &ctypes.ResultABCIQuery{Response: *resQuery}, nil
}

// ABCIInfo gets some info about the application.
// More: https://docs.tendermint.com/v0.34/rpc/#/ABCI/abci_info
func ABCIInfo(ctx *rpctypes.Context) (*ctypes.ResultABCIInfo, error) {
	resInfo, err := env.ProxyAppQuery.InfoSync(proxy.RequestInfo)
	if err != nil {
		return nil, err
	}

	return &ctypes.ResultABCIInfo{Response: *resInfo}, nil
}
