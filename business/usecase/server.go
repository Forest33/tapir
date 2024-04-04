// Package usecase provides business logic.
package usecase

import (
	"context"
	"crypto/rand"
	"sync"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/codec"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
	"github.com/forest33/tapir/pkg/util/hash"
)

// ServerUseCase object capable of interacting with ServerUseCase
type ServerUseCase struct {
	ctx                   context.Context
	log                   *logger.Logger
	cfg                   *entity.ServerConfig
	cfgHandler            configHandler
	codec                 codec.Codec
	encryptor             entity.Encryptor
	merger                entity.StreamMerger
	srv                   entity.NetworkServer
	iface                 entity.InterfaceAdapter
	users                 map[string]*entity.User
	connections           map[entity.ConnectionKey]*serverConn
	interfaces            map[string]*ServerInterfaceInfo
	sessions              map[uint32]*ServerSessionInfo
	client2session        map[string]uint32
	statCh                chan *sessionStatisticRequest
	connMux               sync.RWMutex
	sessMux               sync.RWMutex
	portSelectionStrategy entity.PortSelectionStrategy
	compressionType       entity.CompressionType
}

type ServerInterfaceInfo struct {
	handler     *entity.Interface
	Connections []*entity.Connection
	SessionID   uint32
}

type ServerSessionInfo struct {
	IfName   string
	UserName string
	ClientID string
	Stat     *entity.Statistic
}

type sessionStatisticRequest struct {
	ID   uint32
	stat *entity.Statistic
}

// NewServerUseCase creates a new ServerUseCase
func NewServerUseCase(ctx context.Context, log *logger.Logger, cfg *entity.ServerConfig, cfgHandler configHandler,
	merger entity.StreamMerger, srv entity.NetworkServer, iface entity.InterfaceAdapter) (*ServerUseCase, error) {

	uc := &ServerUseCase{
		ctx:                   ctx,
		log:                   log.Duplicate(log.With().Str("layer", "ucsrv").Logger()),
		cfg:                   cfg,
		cfgHandler:            cfgHandler,
		codec:                 GetCodec(log, cfg.Tunnel.MTU, cfg.Tunnel.Encryption, cfg.Network.ObfuscateData),
		encryptor:             GetEncryptor(cfg.Authentication.Key, cfg.Tunnel.Encryption),
		merger:                merger,
		srv:                   srv,
		iface:                 iface,
		connections:           make(map[entity.ConnectionKey]*serverConn, cfg.Network.MaxPorts()),
		interfaces:            make(map[string]*ServerInterfaceInfo, len(cfg.Users)),
		sessions:              make(map[uint32]*ServerSessionInfo, len(cfg.Users)),
		client2session:        make(map[string]uint32, len(cfg.Users)),
		statCh:                make(chan *sessionStatisticRequest, len(cfg.Users)*cfg.Network.MaxPorts()),
		portSelectionStrategy: entity.GetPortSelectionStrategy(cfg.Network.PortSelectionStrategy),
		compressionType:       entity.GetCompressionType(cfg.Network.Compression),
	}

	return uc, nil
}

func (uc *ServerUseCase) Start() error {
	if err := uc.cfg.Validate(); err != nil {
		return err
	}

	if err := uc.cfgHandler.AddObserver(uc.onConfigChanged); err != nil {
		uc.log.Error().Err(err).Msg("failed to create config file observer")
		return err
	}

	uc.initUsers()
	uc.sessionStat()

	if uc.cfg.Network.UseStreamMerger() {
		uc.merger.SetReceiverHandler(uc.socketReceiver)
		uc.merger.SetDisconnectHandler(uc.disconnect)
		uc.merger.SetResetHandler(uc.reset)
		uc.srv.SetReceiverHandler(uc.merger.Push)
		uc.srv.SetDisconnectHandler(uc.disconnect)
		uc.srv.SetEncryptorGetter(uc.getConnectionEncryptor)
	} else {
		uc.srv.SetReceiverHandler(uc.socketReceiver)
		uc.srv.SetDisconnectHandler(uc.disconnect)
		uc.srv.SetEncryptorGetter(uc.getConnectionEncryptor)
	}

	uc.srv.SetStatisticHandler(uc.addSessionStat)

	var err error
	for port := uc.cfg.Network.PortMin; port <= uc.cfg.Network.PortMax; port++ {
		if *uc.cfg.Network.UseTCP {
			err = uc.srv.Run(uc.cfg.Network.Host, port, entity.ProtoTCP)
			if err != nil {
				uc.log.Error().Err(err).
					Str("host", uc.cfg.Network.Host).
					Uint16("port", port).
					Str("protocol", entity.ProtoTCP.String()).
					Msg("failed to create server")
			}
		}

		if *uc.cfg.Network.UseUDP {
			err = uc.srv.Run(uc.cfg.Network.Host, port, entity.ProtoUDP)
			if err != nil {
				uc.log.Error().Err(err).
					Str("host", uc.cfg.Network.Host).
					Uint16("port", port).
					Str("protocol", entity.ProtoUDP.String()).
					Msg("failed to create server")
			}
		}
	}

	uc.log.Info().
		Uint16("portMin", uc.cfg.Network.PortMin).
		Uint16("portMax", uc.cfg.Network.PortMax).
		Bool("tcp", *uc.cfg.Network.UseTCP).
		Bool("udp", *uc.cfg.Network.UseUDP).
		Bool("mptcp", *uc.cfg.Network.MultipathTCP).
		Int("mtu", uc.cfg.Tunnel.MTU).
		Msg("server started")

	return nil
}

func (uc *ServerUseCase) initUsers() {
	uc.users = structs.SliceToMap(uc.cfg.Users, func(u *entity.User) string { return u.Name })
}

func (uc *ServerUseCase) socketReceiver(msg *entity.Message, conn *entity.Connection) error {
	resp, _ := uc.command(msg, conn)
	if resp == nil {
		return nil
	}

	return uc.srv.Send(resp, conn)
}

func (uc *ServerUseCase) command(msg *entity.Message, conn *entity.Connection) (resp *entity.Message, err error) {
	defer func() {
		if err != nil {
			if msg.Type != entity.MessageTypeData {
				resp = &entity.Message{
					SessionID: conn.SessionID,
					Type:      msg.Type,
					Error:     entity.GetMessageError(err),
				}
			}
			uc.log.Error().Err(err).
				Uint32("session_id", conn.SessionID).
				Str("protocol", conn.Protocol().String()).
				Msg("incoming message error")
		}
	}()

	switch msg.Type {
	case entity.MessageTypeAuthentication:
		resp, err = uc.commandAuthentication(msg, conn)
	case entity.MessageTypeHandshake:
		resp, err = uc.commandHandshake(msg, conn)
	case entity.MessageTypeData:
		resp, err = uc.commandData(msg, conn)
	case entity.MessageTypeReset:
		err = uc.commandReset(msg, conn)
	default:
		err = entity.ErrUnknownCommand
	}

	return
}

func (uc *ServerUseCase) commandAuthentication(msg *entity.Message, conn *entity.Connection) (*entity.Message, error) {
	m, ok := entity.MessageTypePayload[msg.Type]
	if !ok {
		return nil, entity.ErrWrongMessagePayload
	}

	if err := mapstructure.Decode(msg.Payload, &m); err != nil {
		return nil, err
	}

	req := m.(*entity.MessageAuthenticationRequest)
	if user, ok := uc.users[req.Name]; !ok || user.Password != req.Password {
		uc.log.Error().Err(entity.ErrUnauthorized).Str("name", req.Name).Msg("incorrect name or password")
		return nil, entity.ErrUnauthorized
	}

	if msg.SessionID == 0 {
		_ = uc.dropSessionByClientID(req.ClientID)
		conn.SessionID = uc.createSession(req.ClientID, req.Name)
	} else {
		if err := uc.checkSession(msg.SessionID, req.ClientID, req.Name); err != nil {
			uc.log.Error().Err(entity.ErrUnauthorized).Uint32("session_id", msg.SessionID).Str("client_id", req.ClientID).Msg("incorrect session ID")
			return nil, entity.ErrUnauthorized
		}
		conn.SessionID = msg.SessionID
	}

	if uc.cfg.Network.UseStreamMerger() {
		if err := uc.merger.CreateStream(conn.SessionID); err != nil {
			uc.log.Error().Err(err).Msg("failed to create stream")
			return nil, entity.ErrInternalError
		}
	}

	ic, err := uc.createInterface(conn.SessionID)
	if err != nil {
		uc.log.Error().Err(err).
			Str("name", req.Name).
			Str("client_id", req.ClientID).
			Uint32("session_id", conn.SessionID).
			Msg("failed to create network interface")
		return nil, entity.ErrInternalError
	}

	ifName, _ := ic.handler.Name() // sometimes panic!
	uc.addConnection(conn, &serverConn{
		ifName:           ifName,
		sessionID:        conn.SessionID,
		port:             conn.Port,
		protocol:         conn.Protocol(),
		compressionType:  req.CompressionType,
		compressionLevel: req.CompressionLevel,
	})

	uc.log.Info().
		Uint32("session_id", conn.SessionID).
		Str("client_id", req.ClientID).
		Str("name", req.Name).
		Str("addr", conn.Addr.String()).
		Msg("authentication successful")

	return &entity.Message{
		SessionID: conn.SessionID,
		Type:      msg.Type,
		Payload: &entity.MessageAuthenticationResponse{
			SessionID: conn.SessionID,
			LocalIP:   ic.handler.IP.ClientLocal,
			RemoteIP:  ic.handler.IP.ClientRemote,
		},
	}, nil
}

func (uc *ServerUseCase) commandHandshake(msg *entity.Message, conn *entity.Connection) (*entity.Message, error) {
	m, ok := entity.MessageTypePayload[msg.Type]
	if !ok {
		return nil, entity.ErrWrongMessagePayload
	}

	if err := mapstructure.Decode(msg.Payload, &m); err != nil {
		return nil, err
	}

	req := m.(*entity.MessageHandshake)

	sc, ok := uc.getConnection(conn)
	if !ok {
		return nil, entity.ErrConnectionNotExists
	}

	privateKey, err := GetECDHCurve().GenerateKey(rand.Reader)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate ECDSA private key")
	}

	publicKey, err := GetECDHCurve().NewPublicKey(req.Key)
	if err != nil {
		return nil, errors.Wrap(err, "failed to check public key")
	}

	shared, err := privateKey.ECDH(publicKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create shared key")
	}

	uc.setConnectionEncryptor(conn, GetEncryptor(string(shared), uc.cfg.Tunnel.Encryption))
	if err := uc.addInterfaceConnection(sc, conn); err != nil {
		return nil, errors.Wrap(err, "failed to add interface connection")
	}

	uc.log.Info().
		Uint32("session_id", conn.SessionID).
		Str("addr", conn.Addr.String()).
		Str("proto", conn.Protocol().String()).
		Str("key", hash.MD5(shared)).
		Msg("handshake successful")

	return &entity.Message{
		SessionID: conn.SessionID,
		Type:      msg.Type,
		Payload: &entity.MessageHandshake{
			Key: privateKey.PublicKey().Bytes(),
		},
	}, nil
}

func (uc *ServerUseCase) commandData(msg *entity.Message, conn *entity.Connection) (*entity.Message, error) {
	sc, ok := uc.getConnection(conn)
	if !ok {
		uc.log.Error().Err(entity.ErrConnectionNotExists).
			Uint32("session_id", msg.SessionID).
			Str("addr", conn.Addr.String()).
			Msg(entity.ErrConnectionNotExists.Error())
		return nil, entity.ErrConnectionNotExists
	}

	ifc, ok := uc.getInterfaceByName(sc.ifName)
	if !ok {
		uc.log.Error().Msg("no interface")
		return nil, entity.ErrInterfaceNotExists
	}

	ifName, _ := ifc.Name()

	err := uc.iface.Write(ifc, msg.Payload)
	if err != nil {
		uc.log.Error().Err(err).
			Uint32("id", msg.ID).
			Uint32("session_id", msg.SessionID).
			Msg("failed to write to interface")
	}

	uc.iface.SendLog(msg, ifName)

	return nil, err
}

func (uc *ServerUseCase) commandReset(msg *entity.Message, conn *entity.Connection) error {
	sc, ok := uc.getConnection(conn)
	if !ok {
		uc.log.Error().Err(entity.ErrConnectionNotExists).
			Uint32("session_id", msg.SessionID).
			Str("addr", conn.Addr.String()).
			Msg(entity.ErrConnectionNotExists.Error())
		return entity.ErrConnectionNotExists
	}

	return uc.dropSessionByID(sc.sessionID)
}

func (uc *ServerUseCase) reset(sessionID uint32, conn *entity.Connection) {
	req := &entity.Message{
		Type:      entity.MessageTypeReset,
		SessionID: sessionID,
	}

	if err := uc.srv.Send(req, conn); err != nil {
		uc.log.Error().Err(err).Uint32("session_id", sessionID).Msg("failed to send reset")
	}
}

func (uc *ServerUseCase) GetSessions() map[uint32]*ServerSessionInfo {
	uc.connMux.RLock()
	defer uc.connMux.RUnlock()
	return uc.sessions
}

func (uc *ServerUseCase) GetInterfaces() map[string]*ServerInterfaceInfo {
	uc.connMux.RLock()
	defer uc.connMux.RUnlock()
	return uc.interfaces
}

func (uc *ServerUseCase) onConfigChanged(cfg interface{}) {
	uc.cfg = cfg.(*entity.ServerConfig)
	uc.initUsers()
}
