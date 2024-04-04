package server

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

type V1 struct {
	ctx                 context.Context
	log                 *logger.Logger
	originalLog         *logger.Logger
	cfg                 *Config
	receiver            entity.ReceiverHandler
	disconnect          entity.DisconnectHandler
	getUserEncryptor    entity.EncryptorGetter
	retryFactory        entity.NetworkRetryFactory
	ackFactory          entity.NetworkAckFactory
	packetDecoder       entity.PacketDecoder
	addSessionStatistic entity.StatisticHandler
	droppedSessions     map[uint32]struct{}
	dsMux               sync.Mutex
}

func NewV1(ctx context.Context, log *logger.Logger, cfg *Config, retryFactory entity.NetworkRetryFactory, ackFactory entity.NetworkAckFactory, packetDecoder entity.PacketDecoder) (entity.NetworkServer, error) {
	var err error

	once.Do(func() {
		if err = cfg.validate(); err == nil {
			instance = &V1{
				ctx:             ctx,
				log:             log.Duplicate(log.With().Str("layer", "srv").Logger()),
				originalLog:     log,
				cfg:             cfg,
				retryFactory:    retryFactory,
				ackFactory:      ackFactory,
				packetDecoder:   packetDecoder,
				droppedSessions: make(map[uint32]struct{}),
			}
		}
	})

	return instance, err
}

func (s *V1) Run(host string, port uint16, proto entity.Protocol) error {
	switch proto {
	case entity.ProtoTCP:
		lc := &net.ListenConfig{}
		lc.SetMultipathTCP(s.cfg.MultipathTCP)
		lst, err := lc.Listen(s.ctx, proto.String(), fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return err
		}
		s.listenerTCP(lst.(*net.TCPListener))
	case entity.ProtoUDP:
		conn, err := net.ListenUDP(proto.String(), &net.UDPAddr{IP: net.ParseIP(host), Port: int(port)})
		if err != nil {
			return err
		}

		if err := conn.SetReadBuffer(s.cfg.WriteBufferSize); err != nil {
			return err
		}
		if err := conn.SetWriteBuffer(s.cfg.ReadBufferSize); err != nil {
			return err
		}

		s.receiverUDP(conn)
	default:
		return fmt.Errorf("unknown protocol %s", proto)
	}

	return nil
}

func (s *V1) Send(msg *entity.Message, conn *entity.Connection) error {
	var (
		userEncryptor entity.Encryptor
		err           error
	)

	if msg.IsUserData() {
		userEncryptor, err = s.getUserEncryptor(conn)
		if err != nil {
			s.log.Error().Err(err).Uint32("session_id", conn.SessionID).Msg("failed to get connection encryptor")
			return err
		}
	}

	if conn.TCPConn != nil {
		return s.sendTCP(msg, userEncryptor, conn.TCPConn)
	}

	return s.sendUDP(msg, userEncryptor, conn, conn.Retry)
}

func (s *V1) SetReceiverHandler(f entity.ReceiverHandler) {
	s.receiver = f
}

func (s *V1) SetDisconnectHandler(f entity.DisconnectHandler) {
	s.disconnect = f
}

func (s *V1) SetEncryptorGetter(f entity.EncryptorGetter) {
	s.getUserEncryptor = f
}

func (s *V1) SetStatisticHandler(f entity.StatisticHandler) {
	s.addSessionStatistic = f
}

func (s *V1) DropSession(sessionID uint32) {
	s.dsMux.Lock()
	defer s.dsMux.Unlock()
	s.droppedSessions[sessionID] = struct{}{}
}
