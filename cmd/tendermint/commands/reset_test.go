package commands

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	cfg "github.com/tendermint/tendermint/config"
	"github.com/tendermint/tendermint/privval"
)

func Test_ResetAll(t *testing.T) {
	config := cfg.TestConfig()
	dir := t.TempDir()
	config.SetRoot(dir)
	cfg.EnsureRoot(dir)
	require.NoError(t, initFilesWithConfig(config))
	pv := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorBlsKeyFile(),
		config.PrivValidatorStateFile(), config.PrivValidatorRelayerFile())
	pv.LastSignState.Height = 10
	pv.Save()
	require.NoError(t, resetAll(config.DBDir(), config.P2P.AddrBookFile(), config.PrivValidatorKeyFile(),
		config.PrivValidatorBlsKeyFile(), config.PrivValidatorRelayerFile(), config.PrivValidatorStateFile(), logger))
	require.DirExists(t, config.DBDir())
	require.NoFileExists(t, filepath.Join(config.DBDir(), "block.db"))
	require.NoFileExists(t, filepath.Join(config.DBDir(), "state.db"))
	require.NoFileExists(t, filepath.Join(config.DBDir(), "evidence.db"))
	require.NoFileExists(t, filepath.Join(config.DBDir(), "tx_index.db"))
	require.FileExists(t, config.PrivValidatorStateFile())
	pv = privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorBlsKeyFile(),
		config.PrivValidatorStateFile(), config.PrivValidatorRelayerFile())
	require.Equal(t, int64(0), pv.LastSignState.Height)
}

func Test_ResetState(t *testing.T) {
	config := cfg.TestConfig()
	dir := t.TempDir()
	config.SetRoot(dir)
	cfg.EnsureRoot(dir)
	require.NoError(t, initFilesWithConfig(config))
	pv := privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorBlsKeyFile(),
		config.PrivValidatorStateFile(), config.PrivValidatorRelayerFile())
	pv.LastSignState.Height = 10
	pv.Save()
	require.NoError(t, resetState(config.DBDir(), logger))
	require.DirExists(t, config.DBDir())
	require.NoFileExists(t, filepath.Join(config.DBDir(), "block.db"))
	require.NoFileExists(t, filepath.Join(config.DBDir(), "state.db"))
	require.NoFileExists(t, filepath.Join(config.DBDir(), "evidence.db"))
	require.NoFileExists(t, filepath.Join(config.DBDir(), "tx_index.db"))
	require.FileExists(t, config.PrivValidatorStateFile())
	pv = privval.LoadFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorBlsKeyFile(),
		config.PrivValidatorStateFile(), config.PrivValidatorRelayerFile())
	// private validator state should still be in tact.
	require.Equal(t, int64(10), pv.LastSignState.Height)
}
