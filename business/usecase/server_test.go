package usecase

import (
	"context"
	"net"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/forest33/tapir/adapter/merger"
	"github.com/forest33/tapir/adapter/packet"
	"github.com/forest33/tapir/adapter/wiface"
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/command"
	"github.com/forest33/tapir/pkg/config"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
	"github.com/forest33/tapir/pkg/util/test"
)

const (
	packetCount = 10_000_000
)

const (
	defaultConfigFile = "tapir-server.yaml"
	clientID          = "ad73d333-d19e-55dd-9e33-2e9ae43e9178"
	userName          = "anton"
	sharedKey         = "Eqky5BVEX8Nrj9uN4c3PqBY9sfNPbnaP"
)

var (
	ctx  = context.Background()
	zlog *logger.Logger
	cfg  = &entity.ServerConfig{}

	cmd           *command.Executor
	ifaceAdapter  entity.InterfaceAdapter
	serverAdapter entity.NetworkServer
	mergerAdapter entity.StreamMerger
	uc            *ServerUseCase

	wg *sync.WaitGroup
)

func init() {
	cfgHandler, _ := config.New(defaultConfigFile, "", cfg)

	cfg.Tracing = &entity.TracingConfig{}
	cfg.Logger.FileName = ""

	if cfg.Runtime.GoMaxProcs == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		runtime.GOMAXPROCS(cfg.Runtime.GoMaxProcs)
	}

	zlog = logger.New(logger.Config{
		Level:             "info",
		TimeFieldFormat:   cfg.Logger.TimeFieldFormat,
		PrettyPrint:       *cfg.Logger.PrettyPrint,
		DisableSampling:   *cfg.Logger.DisableSampling,
		RedirectStdLogger: *cfg.Logger.RedirectStdLogger,
		ErrorStack:        *cfg.Logger.ErrorStack,
		ShowCaller:        *cfg.Logger.ShowCaller,
		FileName:          cfg.Logger.FileName,
	})

	var err error
	cmd, err = command.New(cfg.System)
	if err != nil {
		zlog.Fatalf("failed to create command executor: %v", err)
	}

	wg = &sync.WaitGroup{}
	wg.Add(1)
	serverAdapter = &MockNetworkServer{
		wg:               wg,
		maxMessagesCount: packetCount,
		codec:            GetCodec(zlog, cfg.Tunnel.MTU, "aes-256-ecb", structs.Ref(false)),
		primaryEncryptor: GetEncryptor(cfg.Authentication.Key, "aes-256-ecb"),
	}

	packetDecoder := packet.New(&packet.Config{Tracing: cfg.Tracing.Interface, EndpointHashType: packet.EndpointHashSourceAddress})

	ifaceAdapter, err = wiface.New(zlog, &wiface.Config{
		Tunnel:               cfg.Tunnel,
		Tracing:              cfg.Tracing.Interface,
		InterfaceCreatorFunc: wiface.CreateMockNetworkInterface,
		InterfaceStartupFunc: func(*entity.Interface, bool) error { return nil },
	}, cmd, packetDecoder)
	if err != nil {
		zlog.Fatalf("failed to create interface handler: %v", err)
	}

	mergerAdapter, err = merger.NewV2(ctx, zlog, &merger.Config{
		WaitingListMaxSize: cfg.StreamMerger.WaitingListMaxSize,
		StreamCount:        len(cfg.Users),
		Tracing:            cfg.Tracing.StreamMerger,
	})
	if err != nil {
		zlog.Fatalf("failed to create stream merger: %v", err)
	}

	uc, err = NewServerUseCase(ctx, zlog, cfg, cfgHandler, mergerAdapter, serverAdapter, ifaceAdapter)
	if err != nil {
		zlog.Fatalf("failed to create usecase: %v", err)
	}
}

func TestReceiveFromInterface(t *testing.T) {
	uc.merger.SetReceiverHandler(func(msg *entity.Message, conn *entity.Connection) error {
		return nil
	})
	uc.merger.SetDisconnectHandler(func(*entity.Connection, error) {})

	addr, err := net.ResolveUDPAddr("udp", "127.0.0.1:33333")
	if err != nil {
		uc.log.Fatalf("failed to create connection: %v", err)
	}

	udpConn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		uc.log.Fatalf("failed to create connection: %v", err)
	}

	conn := &entity.Connection{
		UDPConn:   udpConn,
		Addr:      addr,
		Port:      33333,
		SessionID: uc.createSession(clientID, userName),
	}

	if uc.cfg.Network.UseStreamMerger() {
		if err := uc.merger.CreateStream(conn.SessionID); err != nil {
			uc.log.Fatalf("failed to create stream: %v", err)
		}
	}

	ic, err := uc.createInterface(conn.SessionID)
	if err != nil {
		uc.log.Fatalf("failed to create network interface: %v", err)
	}

	if err := uc.merger.CreateStream(conn.SessionID); err != nil {
		uc.log.Fatalf("failed to create stream: %v", err)
	}

	ifName, _ := ic.handler.Name()
	sc := &serverConn{
		ifName:           ifName,
		sessionID:        conn.SessionID,
		port:             conn.Port,
		protocol:         conn.Protocol(),
		compressionType:  entity.CompressionNone,
		compressionLevel: 0,
	}

	uc.addConnection(conn, sc)

	uc.setConnectionEncryptor(conn, GetEncryptor(sharedKey, "aes-256-ecb"))
	if err := uc.addInterfaceConnection(sc, conn); err != nil {
		uc.log.Fatalf("failed to add interface connection: %v", err)
	}

	startStat := test.MemUsage()
	start := time.Now()
	wg.Wait()
	zlog.Info().Str("duration", time.Since(start).String()).Msg("benchmark finished")
	endStat := test.MemUsage()
	zlog.Info().Uint64("mallocs", endStat.Mallocs-startStat.Mallocs).Uint64("frees", endStat.Frees-startStat.Frees).Msg("heap stats")
}
