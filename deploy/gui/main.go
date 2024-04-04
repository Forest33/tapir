// Package main warthog main package
package main

import (
	"context"
	"log"
	"runtime"

	"github.com/asticode/go-astilectron"

	grpc_client "github.com/forest33/tapir/adapter/grpc/client"
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/business/usecase"
	"github.com/forest33/tapir/pkg/command"
	"github.com/forest33/tapir/pkg/config"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/resources"
)

var (
	zlog *logger.Logger
)

var (
	cfg        = &entity.ClientConfig{}
	cfgHandler *config.Config
	ctx        context.Context
	cancel     context.CancelFunc
	homeDir    string

	guiUseCase *usecase.GUIUseCase
	ast        *astilectron.Astilectron
	window     *astilectron.Window
	tray       *astilectron.Tray
)

const (
	applicationName = "Tapir"
)

func init() {
	homeDir = resources.CreateApplicationDir()

	var err error
	cfgHandler, err = config.New(entity.DefaultClientConfigFileName, homeDir, cfg)
	if err != nil {
		log.Fatalf("failed to parse config file: %v", err)
	}

	cfg.Normalize()

	//cfg.Logger.FileName = `C:\\Projects\tapir.log`
	//cfg.Logger.Level = "debug"

	zlog = logger.New(logger.Config{
		Level:             cfg.Logger.Level,
		TimeFieldFormat:   cfg.Logger.TimeFieldFormat,
		PrettyPrint:       *cfg.Logger.PrettyPrint,
		DisableSampling:   *cfg.Logger.DisableSampling,
		RedirectStdLogger: *cfg.Logger.RedirectStdLogger,
		ErrorStack:        *cfg.Logger.ErrorStack,
		ShowCaller:        *cfg.Logger.ShowCaller,
		FileName:          cfg.Logger.FileName,
	})

	resources.Init(cfg, zlog)

	if cfg.Runtime.GoMaxProcs == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		runtime.GOMAXPROCS(cfg.Runtime.GoMaxProcs)
	}

	ctx, cancel = context.WithCancel(context.Background())
}

func main() {
	defer shutdown()

	prepareClient()

	cmd, err := command.New(cfg.System)
	if err != nil {
		zlog.Fatalf("failed to create command executor: %v", err)
	}

	ipcClient, err := grpc_client.New(cfg.IPC, zlog)
	if err != nil {
		zlog.Fatalf("failed to create gRPC client: %v", err)
	}

	guiUseCase, err = usecase.NewGUIUseCase(ctx, zlog, cfg, cfgHandler, ipcClient, cmd, homeDir)
	if err != nil {
		zlog.Fatalf("failed to create UI: %v", err)
	}

	if UseBootstrap == "true" {
		withBootstrap()
	} else {
		withoutBootstrap()
	}
}

func shutdown() {
	guiUseCase.Shutdown()
	cancel()
}
