// Package main warthog main package
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"

	client_grpc_server "github.com/forest33/tapir/adapter/grpc/server"
	"github.com/forest33/tapir/business/usecase"
	"github.com/forest33/tapir/pkg/automaxprocs"
	"github.com/forest33/tapir/pkg/profiler"
	"github.com/forest33/tapir/pkg/resources"
	"github.com/forest33/tapir/pkg/structs"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/command"
	"github.com/forest33/tapir/pkg/config"
	"github.com/forest33/tapir/pkg/logger"
)

var (
	zlog       *logger.Logger
	cfg        = &entity.ClientConfig{}
	cfgHandler *config.Config
	ctx        context.Context
	cancel     context.CancelFunc

	cmd           *command.Executor
	clientAdapter entity.NetworkClient
	ifaceAdapter  entity.InterfaceAdapter
	mergerAdapter entity.StreamMerger
	connName      *string
)

func init() {
	connName = flag.String("connection", "", "name of connection")
	connImport := flag.String("import", "", "import connection file")
	configFileDir := flag.String("config", "", "directory of config file")
	flag.Parse()

	homeDir := resources.CreateApplicationDir()

	var err error
	cfgHandler, err = config.New(
		entity.DefaultClientConfigFileName,
		structs.If(*configFileDir == "", homeDir, *configFileDir),
		cfg)
	if err != nil {
		log.Fatalf("failed to parse config file: %v", err)
	}

	cfg.Normalize()

	zlog = logger.New(logger.Config{
		Level:             cfg.Logger.Level,
		TimeFieldFormat:   cfg.Logger.TimeFieldFormat,
		PrettyPrint:       *cfg.Logger.PrettyPrint,
		DisableSampling:   *cfg.Logger.DisableSampling,
		RedirectStdLogger: *cfg.Logger.RedirectStdLogger,
		ErrorStack:        *cfg.Logger.ErrorStack,
		ShowCaller:        *cfg.Logger.ShowCaller,
		FileName:          structs.If(cfg.Logger.FileName != "", cfg.Logger.FileName, filepath.Join(homeDir, entity.ClientLogFile)),
	})

	if *connImport != "" {
		if err := importConnection(*connImport); err != nil {
			zlog.Fatalf("failed to import connection: %v", err)
		}
	}

	if cfg.Runtime.GoMaxProcs != 0 {
		runtime.GOMAXPROCS(cfg.Runtime.GoMaxProcs)
	} else {
		automaxprocs.Init(zlog)
	}

	ctx, cancel = context.WithCancel(context.Background())

	if *cfg.Profiler.Enabled {
		profiler.Start(&profiler.Config{
			Host: cfg.Profiler.Host,
			Port: cfg.Profiler.Port,
		}, zlog)
	}
}

func main() {
	defer shutdown()

	var (
		quit = make(chan os.Signal, 1)
		err  error
	)

	cmd, err = command.New(cfg.System)
	if err != nil {
		zlog.Fatalf("failed to create command executor: %v", err)
	}

	connManagerUseCase := usecase.NewConnectionManagerUseCase(
		ctx,
		zlog,
		cfg,
		createClientConnection,
		func() { quit <- syscall.SIGINT },
		cfgHandler)
	if err != nil {
		zlog.Fatalf("failed to create client IPC: %v", err)
	}

	_, err = client_grpc_server.New(cfg, zlog, connManagerUseCase)
	if err != nil {
		zlog.Fatalf("failed to create GRPC server: %v", err)
	}

	if *connName != "" {
		connIdx := slices.IndexFunc(cfg.Connections, func(c *entity.ClientConnection) bool { return c.Name == *connName })
		if connIdx == -1 {
			zlog.Fatalf("connection with name \"%s\" not exists", *connName)
		}
		err := connManagerUseCase.Connect(connIdx)
		if err != nil {
			zlog.Fatal(err)
		}
	}

	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func importConnection(file string) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	conn := &entity.ClientConnection{}
	if err := conn.Unmarshal(data); err != nil {
		return err
	}

	cfg.Connections = append(cfg.Connections, conn)
	cfgHandler.Save()

	zlog.Info().Msg("connection successfully imported")

	return nil
}

func shutdown() {
	cfgHandler.Save()
	cancel()
}
