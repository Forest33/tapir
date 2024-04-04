// Package main Tapir server main package
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/forest33/tapir/adapter/ack"
	rest "github.com/forest33/tapir/adapter/http"
	"github.com/forest33/tapir/adapter/merger"
	"github.com/forest33/tapir/adapter/packet"
	"github.com/forest33/tapir/adapter/retry"
	"github.com/forest33/tapir/adapter/server"
	"github.com/forest33/tapir/adapter/wiface"
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/business/usecase"
	"github.com/forest33/tapir/pkg/automaxprocs"
	"github.com/forest33/tapir/pkg/command"
	"github.com/forest33/tapir/pkg/config"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/profiler"
)

var (
	cfg        = &entity.ServerConfig{}
	cfgHandler *config.Config
	zlog       *logger.Logger
	ctx        context.Context
	cancel     context.CancelFunc

	cmd           *command.Executor
	ifaceAdapter  entity.InterfaceAdapter
	serverAdapter entity.NetworkServer
	mergerAdapter entity.StreamMerger

	serverUseCase *usecase.ServerUseCase
)

func init() {
	var err error
	cfgHandler, err = config.New(entity.DefaultServerConfigFileName, "", cfg)
	if err != nil {
		log.Fatalf("failed to parse config file: %v", err)
	}

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

	if cfg.Runtime.GoMaxProcs != 0 {
		runtime.GOMAXPROCS(cfg.Runtime.GoMaxProcs)
	} else {
		automaxprocs.Init(zlog)
	}

	ctx, cancel = context.WithCancel(context.Background())
}

func main() {
	defer shutdown()

	if len(os.Args[1:]) > 0 {
		parseCommandLine()
		return
	}

	initAdapters()
	initUseCases()

	if *cfg.Profiler.Enabled {
		profiler.Start(&profiler.Config{
			Host: cfg.Profiler.Host,
			Port: cfg.Profiler.Port,
		}, zlog)
	}

	if err := serverUseCase.Start(); err != nil {
		zlog.Fatalf("failed to start server: %v", err)
	}

	initRestServer()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func initAdapters() {
	var err error

	cmd, err = command.New(cfg.System)
	if err != nil {
		zlog.Fatalf("failed to create command executor: %v", err)
	}

	retryFactory := func(ctx context.Context, log *logger.Logger, retrySender entity.RetrySender, keepaliveSender entity.KeepaliveSender,
		disconnect entity.DisconnectHandler, conn *entity.Connection) entity.NetworkRetry {
		return retry.New(ctx, log, &retry.Config{
			MaxTimeout:        time.Duration(cfg.Retry.MaxTimout) * time.Second,
			KeepaliveTimeout:  time.Duration(cfg.Network.KeepaliveTimeout) * time.Second,
			KeepaliveInterval: time.Duration(cfg.Network.KeepaliveInterval) * time.Second,
			KeepaliveProbes:   cfg.Network.KeepaliveProbes,
			BackoffFactor:     cfg.Retry.BackoffFactor,
			Tracing:           cfg.Tracing.Retry,
		}, retrySender, keepaliveSender, disconnect, conn)
	}

	ackFactory := func(ctx context.Context, log *logger.Logger, ackSender entity.AckSender, conn *entity.Connection, sessionID uint32) entity.NetworkAck {
		return ack.New(ctx, log, &ack.Config{
			MaxSize:                 usecase.GetMaxAcknowledgementSize(cfg.Tunnel),
			WaitingTimePercentOfRTO: cfg.Ack.WaitingTimePercentOfRTO,
			EndpointLifeTime:        cfg.Ack.EndpointLifeTime,
			Tracing:                 cfg.Tracing.Ack,
		}, ackSender, conn, sessionID)
	}

	sockPacketDecoder := packet.New(&packet.Config{
		Tracing:          cfg.Tracing.Socket,
		EndpointHashType: packet.EndpointHashDestinationAddress,
	})

	serverAdapter, err = server.NewV1(ctx, zlog, &server.Config{
		Codec:            usecase.GetCodec(zlog, cfg.Tunnel.MTU, cfg.Tunnel.Encryption, cfg.Network.ObfuscateData),
		PrimaryEncryptor: usecase.GetEncryptor(cfg.Authentication.Key, cfg.Tunnel.Encryption),
		MTU:              cfg.Tunnel.MTU,
		WriteBufferSize:  cfg.Network.WriteBufferSize,
		ReadBufferSize:   cfg.Network.ReadBufferSize,
		MultipathTCP:     *cfg.Network.MultipathTCP,
		MaxSessionsCount: len(cfg.Users),
		Tracing:          cfg.Tracing.Socket,
	}, retryFactory, ackFactory, sockPacketDecoder)
	if err != nil {
		zlog.Fatalf("failed to create server: %v", err)
	}

	ifacePacketDecoder := packet.New(&packet.Config{
		Tracing:          cfg.Tracing.Interface,
		EndpointHashType: packet.EndpointHashSourceAddress,
	})

	ifaceAdapter, _ = wiface.New(zlog, &wiface.Config{
		Tunnel:      cfg.Tunnel,
		Tracing:     cfg.Tracing.Interface,
		EndpointTTL: int64(cfg.StreamMerger.StreamTTL) * 2,
	}, cmd, ifacePacketDecoder)

	mergerAdapter, err = merger.New(ctx, zlog, &merger.Config{
		ThreadingBy:         cfg.StreamMerger.ThreadingBy,
		WaitingListMaxSize:  cfg.StreamMerger.WaitingListMaxSize,
		WaitingListMaxTTL:   cfg.StreamMerger.WaitingListMaxTTL,
		StreamCheckInterval: cfg.StreamMerger.StreamCheckInterval,
		StreamTTL:           cfg.StreamMerger.StreamTTL,
		StreamCount:         len(cfg.Users),
		Tracing:             cfg.Tracing.StreamMerger,
	})
	if err != nil {
		zlog.Fatalf("failed to create stream merger: %v", err)
	}
}

func initUseCases() {
	var err error

	serverUseCase, err = usecase.NewServerUseCase(ctx, zlog, cfg, cfgHandler, mergerAdapter, serverAdapter, ifaceAdapter)
	if err != nil {
		zlog.Fatalf("failed to create server: %v", err)
	}
}

func initRestServer() {
	if !*cfg.Rest.Enabled {
		return
	}
	srv, err := rest.New(&rest.Config{
		Host: cfg.Rest.Host,
		Port: cfg.Rest.Port,
	}, zlog, serverUseCase)
	if err != nil {
		zlog.Fatalf("failed to start HTTP server: %v", err)
	}
	srv.Start()
}

func shutdown() {
	cancel()
}
