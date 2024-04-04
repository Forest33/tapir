package main

import (
	_ "embed"
	"os"
	"path/filepath"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/util/hash"
)

var (
	// AppName application name
	AppName string
	// AppVersion application version
	AppVersion string
	// AppURL application homepage
	AppURL = "https://github.com/forest33/tapir"
	// BuiltAt build date
	BuiltAt string
	// VersionAstilectron Astilectron version
	VersionAstilectron string
	// VersionElectron Electron version
	VersionElectron string
	// UseBootstrap true if using bootstrap
	UseBootstrap = "false"

	//go:embed client
	ClientBinData []byte
	ClientBinHash string
)

func prepareClient() {
	if len(ClientBinData) == 0 || ClientBinHash == "" {
		return
	}

	clientBinPath := filepath.Join(homeDir, entity.GetClientBinaryName())
	if _, err := os.Stat(clientBinPath); os.IsNotExist(err) {
		zlog.Info().Str("hash", ClientBinHash).Msg("creating client binary")
		if err := os.WriteFile(clientBinPath, ClientBinData, 0755); err != nil {
			zlog.Fatalf("failed to write client binary: %v", err)
		}
		return
	}

	curClientBin, err := os.ReadFile(clientBinPath)
	if err != nil {
		zlog.Fatalf("failed to read client binary: %v", err)
	}

	curHash := hash.MD5(curClientBin)
	if curHash != ClientBinHash {
		zlog.Info().
			Str("old", curHash).
			Str("new", ClientBinHash).
			Msg("updating client binary")
		if err := os.WriteFile(clientBinPath, ClientBinData, 0755); err != nil {
			zlog.Fatalf("failed to write client binary: %v", err)
		}
	}
}
