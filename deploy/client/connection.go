package main

import (
	"context"
	"time"

	"github.com/pkg/errors"

	"github.com/forest33/tapir/adapter/ack"
	"github.com/forest33/tapir/adapter/client"
	"github.com/forest33/tapir/adapter/merger"
	"github.com/forest33/tapir/adapter/packet"
	"github.com/forest33/tapir/adapter/retry"
	"github.com/forest33/tapir/adapter/wiface"
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/business/usecase"
	"github.com/forest33/tapir/pkg/logger"
)

func createClientConnection(clientConn *entity.ClientConnection, statHandler entity.StatisticHandler) (entity.ClientConnectionHandler, error) {
	var err error

	retryFactory := func(ctx context.Context, log *logger.Logger, retrySender entity.RetrySender, keepaliveSender entity.KeepaliveSender,
		disconnect entity.DisconnectHandler, conn *entity.Connection) entity.NetworkRetry {
		return retry.New(ctx, log, &retry.Config{
			MaxTimeout:        time.Duration(cfg.Retry.MaxTimout) * time.Second,
			KeepaliveTimeout:  time.Duration(clientConn.Server.KeepaliveTimeout) * time.Second,
			KeepaliveInterval: time.Duration(clientConn.Server.KeepaliveInterval) * time.Second,
			KeepaliveProbes:   clientConn.Server.KeepaliveProbes,
			BackoffFactor:     cfg.Retry.BackoffFactor,
			Tracing:           cfg.Tracing.Retry,
		}, retrySender, keepaliveSender, disconnect, conn)
	}

	ackFactory := func(ctx context.Context, log *logger.Logger, ackSender entity.AckSender, conn *entity.Connection, sessionID uint32) entity.NetworkAck {
		return ack.New(ctx, log, &ack.Config{
			MaxSize:                 usecase.GetMaxAcknowledgementSize(clientConn.Tunnel),
			WaitingTimePercentOfRTO: cfg.Ack.WaitingTimePercentOfRTO,
			EndpointLifeTime:        cfg.Ack.EndpointLifeTime,
			Tracing:                 cfg.Tracing.Ack,
		}, ackSender, conn, sessionID)
	}

	sockPacketDecoder := packet.New(&packet.Config{
		Tracing:          cfg.Tracing.Socket,
		EndpointHashType: packet.EndpointHashSourceAddress,
	})

	clientAdapter, err = client.New(ctx, zlog, &client.Config{
		Codec:             usecase.GetCodec(zlog, clientConn.Tunnel.MTU, clientConn.Tunnel.Encryption, clientConn.Server.ObfuscateData),
		PrimaryEncryptor:  usecase.GetEncryptor(clientConn.Authentication.Key, clientConn.Tunnel.Encryption),
		MTU:               clientConn.Tunnel.MTU,
		WriteBufferSize:   clientConn.Server.WriteBufferSize,
		ReadBufferSize:    clientConn.Server.ReadBufferSize,
		MultipathTCP:      *clientConn.Server.MultipathTCP,
		KeepaliveInterval: time.Duration(clientConn.Server.KeepaliveInterval) * time.Second,
		SocketTracing:     cfg.Tracing.Socket,
	}, retryFactory, ackFactory, sockPacketDecoder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create client")
	}

	ifacePacketDecoder := packet.New(&packet.Config{
		Tracing:          cfg.Tracing.Interface,
		EndpointHashType: packet.EndpointHashDestinationAddress,
	})

	ifaceAdapter, err = wiface.New(zlog, &wiface.Config{
		Tunnel:      clientConn.Tunnel,
		Tracing:     cfg.Tracing.Interface,
		EndpointTTL: int64(cfg.StreamMerger.StreamTTL) * 2,
		ServerHost:  clientConn.Server.Host,
	}, cmd, ifacePacketDecoder)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create network interface manager")
	}

	mergerAdapter, err = merger.New(ctx, zlog, &merger.Config{
		ThreadingBy:         cfg.StreamMerger.ThreadingBy,
		WaitingListMaxSize:  cfg.StreamMerger.WaitingListMaxSize,
		WaitingListMaxTTL:   cfg.StreamMerger.WaitingListMaxTTL,
		StreamCheckInterval: cfg.StreamMerger.StreamCheckInterval,
		StreamTTL:           cfg.StreamMerger.StreamTTL,
		StreamCount:         1,
		Tracing:             cfg.Tracing.StreamMerger,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to create stream merger")
	}

	return usecase.NewClientUseCase(ctx, zlog, cfg, clientConn, mergerAdapter, clientAdapter, ifaceAdapter, statHandler)
}
