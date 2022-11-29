package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	cfg "github.com/tendermint/tendermint/config"
	tmos "github.com/tendermint/tendermint/libs/os"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/privval"
	"github.com/tendermint/tendermint/types"
	tmtime "github.com/tendermint/tendermint/types/time"
)

// InitFilesCmd initialises a fresh Tendermint Core instance.
var InitFilesCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tendermint",
	RunE:  initFiles,
}

func initFiles(cmd *cobra.Command, args []string) error {
	return initFilesWithConfig(config)
}

func initFilesWithConfig(config *cfg.Config) error {
	// private validator
	privValKeyFile := config.PrivValidatorKeyFile()
	privValBlsKeyFile := config.PrivValidatorBlsKeyFile()
	privValRelayerFile := config.PrivValidatorRelayerFile()
	privValStateFile := config.PrivValidatorStateFile()
	var pv *privval.FilePV
	if tmos.FileExists(privValKeyFile) {
		pv = privval.LoadFilePV(privValKeyFile, privValBlsKeyFile, privValStateFile, privValRelayerFile)
		logger.Info("Found private validator", "keyFile", privValKeyFile,
			"blsKeyFile", privValBlsKeyFile, "relayerFile", privValRelayerFile, "stateFile", privValStateFile)
	} else {
		pv = privval.GenFilePV(privValKeyFile, privValBlsKeyFile, privValStateFile, privValRelayerFile)
		pv.Save()
		logger.Info("Generated private validator", "keyFile", privValKeyFile,
			"blsKeyFile", privValBlsKeyFile, "relayerFile", privValRelayerFile, "stateFile", privValStateFile)
	}

	nodeKeyFile := config.NodeKeyFile()
	if tmos.FileExists(nodeKeyFile) {
		logger.Info("Found node key", "path", nodeKeyFile)
	} else {
		if _, err := p2p.LoadOrGenNodeKey(nodeKeyFile); err != nil {
			return err
		}
		logger.Info("Generated node key", "path", nodeKeyFile)
	}

	// genesis file
	genFile := config.GenesisFile()
	if tmos.FileExists(genFile) {
		logger.Info("Found genesis file", "path", genFile)
	} else {
		genDoc := types.GenesisDoc{
			ChainID:         fmt.Sprintf("test-chain-%v", tmrand.Str(6)),
			GenesisTime:     tmtime.Now(),
			ConsensusParams: types.DefaultConsensusParams(),
		}
		pubKey, err := pv.GetPubKey()
		if err != nil {
			return fmt.Errorf("can't get pubkey: %w", err)
		}
		blsPubKey, err := pv.GetBlsPubKey()
		if err != nil {
			return fmt.Errorf("can't get bls pubkey: %w", err)
		}
		relayer, err := pv.GetRelayer()
		if err != nil {
			return fmt.Errorf("can't get relayer: %w", err)
		}
		genDoc.Validators = []types.GenesisValidator{{
			Address:   pubKey.Address(),
			PubKey:    pubKey,
			BlsPubKey: blsPubKey,
			Relayer:   relayer,
			Power:     10,
		}}

		if err := genDoc.SaveAs(genFile); err != nil {
			return err
		}
		logger.Info("Generated genesis file", "path", genFile)
	}

	return nil
}
