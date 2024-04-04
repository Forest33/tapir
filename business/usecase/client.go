package usecase

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/codec"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/util/hash"
)

// ClientUseCase object capable of interacting with ClientUseCase
type ClientUseCase struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	log                   *logger.Logger
	cfg                   *entity.ClientConfig
	codec                 codec.Codec
	encryptor             entity.Encryptor
	merger                entity.StreamMerger
	client                entity.NetworkClient
	iface                 entity.InterfaceAdapter
	addConnectionStat     entity.StatisticHandler
	conn                  *entity.ClientConnection
	connections           []*clientConn
	connectionsMap        map[entity.ConnectionKey]*clientConn
	interfaceConn         *clientInterfaceInfo
	connMux               sync.RWMutex
	isConnected           atomic.Bool
	isExit                atomic.Bool
	sessionID             uint32
	portSelectionStrategy entity.PortSelectionStrategy
	compressionType       entity.CompressionType
	compressionLevel      entity.CompressionLevel
}

type clientConn struct {
	idx       int
	conn      *entity.Connection
	encryptor entity.Encryptor
}

type clientInterfaceInfo struct {
	handler     *entity.Interface
	connections []*entity.Connection
}

// NewClientUseCase creates a new ClientUseCase
func NewClientUseCase(ctx context.Context, log *logger.Logger, cfg *entity.ClientConfig, conn *entity.ClientConnection,
	merger entity.StreamMerger, client entity.NetworkClient, iface entity.InterfaceAdapter, statHandler entity.StatisticHandler) (*ClientUseCase, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(ctx)

	uc := &ClientUseCase{
		ctx:                   ctx,
		cancel:                cancel,
		log:                   log.Duplicate(log.With().Str("layer", "uccli").Str("conn", conn.Name).Logger()),
		cfg:                   cfg,
		codec:                 GetCodec(log, conn.Tunnel.MTU, conn.Tunnel.Encryption, conn.Server.ObfuscateData),
		encryptor:             GetEncryptor(conn.Authentication.Key, conn.Tunnel.Encryption),
		merger:                merger,
		client:                client,
		iface:                 iface,
		addConnectionStat:     statHandler,
		conn:                  conn,
		connections:           make([]*clientConn, 0, conn.Server.MaxPorts()),
		connectionsMap:        make(map[entity.ConnectionKey]*clientConn, conn.Server.MaxPorts()),
		portSelectionStrategy: entity.GetPortSelectionStrategy(conn.Server.PortSelectionStrategy),
		compressionType:       entity.GetCompressionType(conn.Server.Compression),
		compressionLevel:      entity.CompressionLevel(conn.Server.CompressionLevel),
	}

	return uc, uc.init()
}

func (uc *ClientUseCase) init() error {
	uc.reset()

	if uc.conn.Server.UseStreamMerger() {
		uc.merger.SetReceiverHandler(uc.socketReceiver)
		uc.merger.SetDisconnectHandler(uc.disconnect)
		uc.merger.SetResetHandler(func(_ uint32, _ *entity.Connection) {})
		uc.client.SetReceiverHandler(uc.merger.Push)
		uc.client.SetDisconnectHandler(uc.disconnect)
		uc.client.SetEncryptorGetter(uc.getConnectionEncryptor)
	} else {
		uc.client.SetReceiverHandler(uc.socketReceiver)
		uc.client.SetDisconnectHandler(uc.disconnect)
		uc.client.SetEncryptorGetter(uc.getConnectionEncryptor)
	}

	uc.client.SetStatisticHandler(uc.addConnectionStat)

	uc.log.Info().
		Str("host", uc.conn.Server.Host).
		Uint16("portMin", uc.conn.Server.PortMin).
		Uint16("portMax", uc.conn.Server.PortMax).
		Bool("TCP", *uc.conn.Server.UseTCP).
		Bool("UDP", *uc.conn.Server.UseUDP).
		Msg("client started")

	return nil
}

func (uc *ClientUseCase) reset() {
	uc.sessionID = 0
	uc.isConnected.Store(false)
}

func (uc *ClientUseCase) Start() error {
	for port := uc.conn.Server.PortMin; port <= uc.conn.Server.PortMax; port++ {
		if *uc.conn.Server.UseTCP {
			err := uc.createConnection(port, entity.ProtoTCP)
			if err != nil {
				uc.log.Error().Err(err).
					Str("host", uc.conn.Server.Host).
					Uint16("port", port).
					Str("protocol", entity.ProtoTCP.String()).
					Msg("server connection error")
				return err
			}
		}
		if *uc.conn.Server.UseUDP {
			err := uc.createConnection(port, entity.ProtoUDP)
			if err != nil {
				uc.log.Error().Err(err).
					Str("host", uc.conn.Server.Host).
					Uint16("port", port).
					Str("protocol", entity.ProtoUDP.String()).
					Msg("server connection error")
				return err
			}
		}
	}

	return nil
}

func (uc *ClientUseCase) socketReceiver(msg *entity.Message, conn *entity.Connection) error {
	if msg.SessionID != uc.sessionID {
		uc.log.Error().Err(entity.ErrSessionNotExists).
			Uint32("message_session_id", msg.SessionID).
			Uint32("client_session_id", uc.sessionID).
			Msg("wrong session ID received")
		return nil
	}

	return uc.command(msg, conn)
}

func (uc *ClientUseCase) command(msg *entity.Message, conn *entity.Connection) (err error) {
	defer func() {
		if err != nil {
			uc.log.Error().Err(err).
				Uint32("id", msg.ID).
				Uint32("session_id", msg.SessionID).
				Str("protocol", conn.Protocol().String()).Msg("incoming message error")
		}
	}()

	switch msg.Type {
	case entity.MessageTypeData:
		err = uc.commandData(msg)
	case entity.MessageTypeReset:
		uc.commandReset()
	default:
		err = entity.ErrUnknownCommand
	}

	return
}

func (uc *ClientUseCase) commandAuthentication(cc *clientConn) error {
	req := &entity.Message{
		Type:      entity.MessageTypeAuthentication,
		SessionID: uc.sessionID,
		Payload: &entity.MessageAuthenticationRequest{
			ClientID:         uc.cfg.System.ClientID,
			Name:             uc.conn.User.Name,
			Password:         uc.conn.User.Password,
			CompressionType:  uc.compressionType,
			CompressionLevel: uc.compressionLevel,
		},
	}

	msg, conn, err := uc.client.SendSync(req, cc.conn, time.Duration(uc.conn.Server.AuthenticationTimeout)*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}

	return uc.responseAuthentication(msg, conn)
}

func (uc *ClientUseCase) responseAuthentication(msg *entity.Message, conn *entity.Connection) error {
	var err error
	msg.Payload, err = uc.encryptor.Decrypt(msg.Payload)
	if err != nil {
		uc.log.Error().Err(err).Msg("failed to decrypt payload")
		return err
	}

	if err := uc.codec.UnmarshalPayload(msg); err != nil {
		uc.log.Error().Err(err).Msg("failed to unmarshal payload")
		return err
	}

	if msg.Error != 0 {
		return msg.Error.Error()
	}

	resp := &entity.MessageAuthenticationResponse{}
	if err := mapstructure.Decode(msg.Payload, &resp); err != nil {
		return err
	}

	if uc.conn.Server.UseStreamMerger() {
		if err := uc.merger.CreateStream(resp.SessionID); err != nil {
			uc.log.Error().Err(err).Msg("failed to create stream")
			return entity.ErrInternalError
		}
	}

	if err := uc.createInterface(resp.LocalIP, resp.RemoteIP, conn); err != nil {
		uc.log.Error().Err(err).Msg("failed to create network interface")
		return err
	}

	uc.sessionID = resp.SessionID

	uc.log.Info().
		Uint32("session_id", uc.sessionID).
		Str("addr", conn.Addr.String()).
		Msg("authentication successful")

	return nil
}

func (uc *ClientUseCase) commandHandshake(cc *clientConn) error {
	privateKey, err := GetECDHCurve().GenerateKey(rand.Reader)
	if err != nil {
		return errors.Wrap(err, "failed to generate ECDSA private key")
	}

	req := &entity.Message{
		SessionID: uc.sessionID,
		Type:      entity.MessageTypeHandshake,
		Payload: &entity.MessageHandshake{
			Key: privateKey.PublicKey().Bytes(),
		},
	}

	msg, conn, err := uc.client.SendSync(req, cc.conn, time.Duration(uc.conn.Server.HandshakeTimeout)*time.Second)
	if err != nil {
		return errors.Wrap(err, "failed to send message")
	}

	return uc.responseHandshake(msg, conn, privateKey)
}

func (uc *ClientUseCase) responseHandshake(msg *entity.Message, conn *entity.Connection, privateKey *ecdh.PrivateKey) error {
	var err error
	msg.Payload, err = uc.encryptor.Decrypt(msg.Payload)
	if err != nil {
		uc.log.Error().Err(err).Msg("failed to decrypt payload")
		return err
	}

	if err := uc.codec.UnmarshalPayload(msg); err != nil {
		uc.log.Error().Err(err).Msg("failed to unmarshal payload")
		return err
	}

	if msg.Error != 0 {
		return msg.Error.Error()
	}

	resp := &entity.MessageHandshake{}
	if err := mapstructure.Decode(msg.Payload, &resp); err != nil {
		return err
	}

	publicKey, err := GetECDHCurve().NewPublicKey(resp.Key)
	if err != nil {
		uc.log.Error().Err(err).Msg("failed to check public key")
		return err
	}

	shared, err := privateKey.ECDH(publicKey)
	if err != nil {
		uc.log.Error().Err(err).Msg("failed to create shared key")
		return err
	}

	uc.setConnectionEncryptor(conn, GetEncryptor(string(shared), uc.conn.Tunnel.Encryption))

	uc.isConnected.Store(true)

	uc.log.Info().
		Uint32("session_id", uc.sessionID).
		Str("addr", conn.Addr.String()).
		Str("proto", conn.Protocol().String()).
		Str("key", hash.MD5(shared)).
		Msg("handshake successful")

	return nil
}

func (uc *ClientUseCase) commandData(msg *entity.Message) error {
	if !uc.isConnected.Load() {
		return nil
	}

	ifName, _ := uc.interfaceConn.handler.Name()

	err := uc.iface.Write(uc.interfaceConn.handler, msg.Payload)
	if err != nil {
		uc.log.Error().Err(err).
			Uint32("id", msg.ID).
			Uint32("session_id", msg.SessionID).
			Msg("failed to write to interface")
		return err
	}

	uc.iface.SendLog(msg, ifName)

	return nil
}

func (uc *ClientUseCase) commandReset() {
	if uc.isExit.Load() {
		return
	}

	uc.log.Info().Msg("received reset command, reconnecting...")
	uc.reset()
	uc.closeAndRemoveAllConnections()
	uc.closeInterface()
}

func (uc *ClientUseCase) sendReset() {
	if !uc.isConnected.Load() {
		return
	}

	cc, err := uc.getConnection(nil)
	if err != nil {
		uc.log.Error().Err(err).Msg("no connection")
		return
	}

	req := &entity.Message{
		Type:      entity.MessageTypeReset,
		SessionID: uc.sessionID,
	}

	if err := uc.client.SendAsync(req, cc.conn); err != nil {
		uc.log.Error().Err(err).Msg("failed to send reset message")
	}
}

func (uc *ClientUseCase) Stop() {
	if !uc.isConnected.Load() {
		return
	}
	uc.merger.DeleteStream(uc.sessionID)
	uc.closeInterface()
	uc.sendReset()
	uc.reset()
}

func (uc *ClientUseCase) Exit() {
	if !uc.isConnected.Load() {
		return
	}
	uc.isExit.Store(true)
	uc.sendReset()
	uc.cancel()
	uc.merger.DeleteStream(uc.sessionID)
	uc.closeAndRemoveAllConnections()
	uc.closeInterface()
	uc.reset()
}
