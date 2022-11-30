package votepool

import (
	tmstate "github.com/tendermint/tendermint/proto/tendermint/state"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	sm "github.com/tendermint/tendermint/state"
	"github.com/tendermint/tendermint/types"
)

type mockStoreDB struct {
	state      sm.State
	validators []*types.Validator
}

func (m mockStoreDB) LoadFromDBOrGenesisFile(s string) (sm.State, error) {
	panic("implement me")
}

func (m mockStoreDB) LoadFromDBOrGenesisDoc(doc *types.GenesisDoc) (sm.State, error) {
	panic("implement me")
}

func (m mockStoreDB) Load() (sm.State, error) {
	s := sm.State{}
	s.Validators = types.NewValidatorSet(m.validators)
	m.state = s
	return s, nil
}

func (m mockStoreDB) LoadValidators(i int64) (*types.ValidatorSet, error) {
	return m.state.Validators, nil
}

func (m mockStoreDB) LoadABCIResponses(i int64) (*tmstate.ABCIResponses, error) {
	panic("implement me")
}

func (m mockStoreDB) LoadLastABCIResponse(i int64) (*tmstate.ABCIResponses, error) {
	panic("implement me")
}

func (m mockStoreDB) LoadConsensusParams(i int64) (tmproto.ConsensusParams, error) {
	panic("implement me")
}

func (m mockStoreDB) Save(state sm.State) error {
	panic("implement me")
}

func (m mockStoreDB) SaveABCIResponses(i int64, responses *tmstate.ABCIResponses) error {
	panic("implement me")
}

func (m mockStoreDB) Bootstrap(state sm.State) error {
	panic("implement me")
}

func (m mockStoreDB) PruneStates(i int64, i2 int64) error {
	panic("implement me")
}

func (m mockStoreDB) Close() error {
	panic("implement me")
}
