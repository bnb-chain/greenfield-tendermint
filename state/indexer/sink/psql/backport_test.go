package psql

import (
	"github.com/bnb-chain/greenfield-tendermint/state/indexer"
	"github.com/bnb-chain/greenfield-tendermint/state/txindex"
)

var (
	_ indexer.BlockIndexer = BackportBlockIndexer{}
	_ txindex.TxIndexer    = BackportTxIndexer{}
)
