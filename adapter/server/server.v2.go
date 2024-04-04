package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
)

type V2 struct {
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

type server struct {
	srv               *V2
	conn              *entity.Connection
	connectionControl map[uint32]connControl
	curSessionsCount  int
	maxSessionsCount  int
}

func NewV2(ctx context.Context, log *logger.Logger, cfg *Config, retryFactory entity.NetworkRetryFactory, ackFactory entity.NetworkAckFactory, packetDecoder entity.PacketDecoder) (entity.NetworkServer, error) {
	var err error

	once.Do(func() {
		if err = cfg.validate(); err == nil {
			instance = &V2{
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

func (s *V2) Run(host string, port uint16, proto entity.Protocol) error {
	srv := &server{
		srv:              s,
		maxSessionsCount: s.cfg.MaxSessionsCount,
		conn: &entity.Connection{
			Proto: proto,
			Port:  port,
		},
	}

	host = structs.If(host != "", host, "0.0.0.0")

	go func() {
		switch proto {
		case entity.ProtoTCP:
			err := gnet.Run(srv, fmt.Sprintf("tcp://%s:%d", host, port),
				gnet.WithMulticore(true),
				gnet.WithReuseAddr(true),
				gnet.WithReusePort(true))
			if err != nil {
				s.log.Fatalf("failed to start server: %v", err)
			}
		case entity.ProtoUDP:
			srv.connectionControl = make(map[uint32]connControl, 10)
			err := gnet.Run(srv, fmt.Sprintf("udp://%s:%d", host, port),
				gnet.WithMulticore(true),
				gnet.WithReuseAddr(true),
				gnet.WithReusePort(true))
			if err != nil {
				s.log.Fatalf("failed to start server: %v", err)
			}
		default:
			s.log.Fatalf("unknown protocol %s", proto)
		}
	}()

	return nil
}

func (s *V2) Send(msg *entity.Message, conn *entity.Connection) error {
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

	encodedHeader, encodedPayload, err := s.cfg.Codec.Marshal(msg)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to encode message")
		return err
	}

	cypherHeader, err := s.cfg.PrimaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to encrypt header")
		return err
	}

	var cypherPayload []byte
	if len(encodedPayload) != 0 {
		if userEncryptor != nil {
			cypherPayload, err = userEncryptor.Encrypt(encodedPayload)
		} else {
			cypherPayload, err = s.cfg.PrimaryEncryptor.Encrypt(encodedPayload)
		}
		if err != nil {
			s.log.Error().Err(err).Msg("failed to encrypt payload")
			return err
		}
	}

	data := append(cypherHeader, cypherPayload...)

	if err := s.send(data, msg, conn); err != nil {
		return err
	}

	return nil
}

func (s *V2) send(data []byte, msg *entity.Message, conn *entity.Connection) error {
	var (
		sent, n int
		err     error
	)

	for sent < len(data) {
		n, err = conn.GNetConn.Write(data[sent:])
		if err != nil {
			return err
		}
		sent += n
	}

	s.sendMessageLog(data[entity.HeaderSize:], sent, msg, "sent to socket")

	return nil
}

func (s *V2) sendKeepalive(conn *entity.Connection, ack bool) {
	msg := &entity.Message{
		SessionID: conn.SessionID,
		Type:      entity.MessageTypeKeepalive,
		IsACK:     ack,
	}

	encodedHeader, _, err := s.cfg.Codec.Marshal(msg)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to encode keepalive message")
		return
	}

	cypherHeader, err := s.cfg.PrimaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to encrypt keepalive header")
		return
	}

	err = s.send(cypherHeader, msg, conn)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to send keepalive")
	}
}

func (s *V2) sendUDP(msg *entity.Message, userEncryptor entity.Encryptor, conn *entity.Connection, retry entity.NetworkRetry) error {
	encodedHeader, encodedPayload, err := s.cfg.Codec.Marshal(msg)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to encode message")
		return err
	}

	var cypherPayload []byte
	if len(encodedPayload) != 0 {
		if userEncryptor != nil {
			cypherPayload, err = userEncryptor.Encrypt(encodedPayload)
		} else {
			cypherPayload, err = s.cfg.PrimaryEncryptor.Encrypt(encodedPayload)
		}
		if err != nil {
			s.log.Error().Err(err).Msg("failed to encrypt payload")
			return err
		}
	}

	cypherHeader, err := s.cfg.PrimaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to encrypt header")
		return err
	}

	data := append(cypherHeader, cypherPayload...)

	err = s.send(data, msg, conn)
	if err != nil {
		return err
	}

	if msg.IsSendACK() {
		msg.Payload = data
		retry.Push(msg)
	}

	return nil
}

func (srv *server) OnTraffic(conn gnet.Conn) (action gnet.Action) {
	var (
		s               = srv.srv
		headerLength    = s.cfg.PrimaryEncryptor.GetLength(entity.HeaderSize)
		header          []byte
		decryptedHeader []byte
		enc             entity.Encryptor
		err             error
	)

	for {
		header, err = conn.Peek(headerLength)
		if errors.Is(err, io.ErrShortBuffer) {
			break
		} else if err != nil {
			if entity.IsErrorInterruptingNetwork(err) {
				return gnet.Close
			}
			s.log.Error().Err(err).Msg("failed to read header from socket")
			break
		} else if len(header) < headerLength {
			break
		}

		decryptedHeader, err = s.cfg.PrimaryEncryptor.Decrypt(header)
		if err != nil {
			s.log.Error().Err(err).
				Int("header_size", len(header)).
				Str("key", s.cfg.PrimaryEncryptor.GetKey()).
				Str("header", fmt.Sprintf("% x", header)).
				Msg("failed to decrypt header")
			return gnet.Close
		}

		msg := &entity.Message{}
		if err = s.cfg.Codec.UnmarshalHeader(decryptedHeader, msg); err != nil {
			s.log.Error().Err(err).
				Int("encrypt_header_size", len(header)).
				Int("header_size", len(decryptedHeader)).
				Str("key", s.cfg.PrimaryEncryptor.GetKey()).
				Str("header", fmt.Sprintf("% x", decryptedHeader)).
				Msg("failed to unmarshal header")
			return gnet.Close
		}

		packetLength := headerLength
		if msg.PayloadLength > 0 {
			packetLength += int(msg.PayloadLength)
			if l := conn.InboundBuffered(); l < packetLength {
				break
			}

			buf, err := conn.Peek(packetLength)
			if errors.Is(err, io.ErrShortBuffer) {
				break
			} else if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				s.log.Error().Err(err).Msg("failed to read payload from socket")
				break
			}
			msg.Payload = buf[headerLength:]
		}

		if _, err := conn.Discard(packetLength); err != nil {
			s.log.Error().Err(err).Msg("failed to discard buffer")
		}

		s.addSessionStatistic(msg.SessionID, &entity.Statistic{
			OutgoingBytes:  uint64(headerLength + int(msg.PayloadLength)),
			OutgoingFrames: 1,
		})

		var port int
		if srv.conn.Proto == entity.ProtoTCP {
			port = conn.LocalAddr().(*net.TCPAddr).Port
		} else {
			port = conn.LocalAddr().(*net.UDPAddr).Port
		}

		connection := &entity.Connection{
			GNetConn:  conn,
			Addr:      conn.RemoteAddr(),
			Port:      uint16(port),
			Proto:     srv.conn.Proto,
			SessionID: msg.SessionID,
		}

		if msg.IsUserData() {
			enc, err = s.getUserEncryptor(connection)
			if err != nil {
				s.log.Error().Err(err).Uint32("session_id", connection.SessionID).Msg("failed to get connection encryptor")
				return gnet.Close
			} else if enc == nil {
				break
			}
		} else {
			enc = s.cfg.PrimaryEncryptor
		}

		if msg.IsPayload() {
			msg.Payload, err = enc.Decrypt(msg.Payload)
			if err != nil {
				s.log.Error().Err(err).Msg("failed to decrypt payload")
				break
			}

			if err := s.cfg.Codec.UnmarshalPayload(msg); err != nil {
				s.log.Error().Err(err).Msg("failed to unmarshal payload")
				break
			}

			if msg.IsUserData() {
				msg.PacketInfo, err = s.packetDecoder.Decode(msg.Payload.([]byte))
				if err != nil {
					break
				}
			}
		}

		s.receiveMessageLog(msg.Payload, headerLength+int(msg.PayloadLength), msg)

		if srv.conn.Proto == entity.ProtoUDP {
			if msg.SessionID != 0 {
				if cc, ok := srv.connectionControl[msg.SessionID]; !ok {
					connection.Retry = s.retryFactory(s.ctx, s.originalLog, s.send, s.sendKeepalive, s.disconnect, connection)
					connection.Ack = s.ackFactory(s.ctx, s.originalLog, s.sendUDP, connection, msg.SessionID)
					srv.connectionControl[msg.SessionID] = connControl{
						retry: connection.Retry,
						ack:   connection.Ack,
					}
					srv.curSessionsCount++
				} else {
					connection.Retry = cc.retry
					connection.Ack = cc.ack
				}
			}

			if msg.Type == entity.MessageTypeKeepalive {
				if !msg.IsACK {
					connection.Retry.Keepalive()
				} else {
					connection.Retry.Ack(nil)
				}
				break
			}

			if msg.IsACK {
				connection.Retry.Ack(msg.Payload.(*entity.MessageAcknowledgement))
				break
			} else if msg.IsSendACK() {
				connection.Ack.Push(msg.ID, msg.GetEndpoint()) // panic: runtime error: invalid memory address or nil pointer dereference
			}
		}

		if err = s.receiver(msg, connection); err != nil {
			if msg.Type != entity.MessageTypeData {
				return gnet.Close
			}
		}
	}

	if srv.conn.Proto == entity.ProtoUDP && srv.curSessionsCount > srv.maxSessionsCount {
		s.dsMux.Lock()
		for sessionID := range s.droppedSessions {
			delete(srv.connectionControl, sessionID)
			delete(s.droppedSessions, sessionID)
			srv.curSessionsCount--
		}
		s.dsMux.Unlock()
		if srv.curSessionsCount > srv.maxSessionsCount {
			srv.maxSessionsCount++
			s.log.Debug().Int("max_sessions_count", srv.maxSessionsCount).Msg("increasing max sessions count")
		}
	}

	return gnet.None
}

func (srv *server) OnBoot(eng gnet.Engine) (action gnet.Action) {
	return gnet.None
}

func (srv *server) OnShutdown(eng gnet.Engine) {
}

func (srv *server) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	return nil, gnet.None
}

func (srv *server) OnClose(conn gnet.Conn, err error) (action gnet.Action) {
	srv.srv.disconnect(&entity.Connection{
		GNetConn: conn,
		Addr:     conn.RemoteAddr(),
		Proto:    srv.conn.Proto,
		Port:     srv.conn.Port,
	}, err)
	return gnet.Close
}

func (srv *server) OnTick() (delay time.Duration, action gnet.Action) {
	return
}

func (s *V2) SetReceiverHandler(f entity.ReceiverHandler) {
	s.receiver = f
}

func (s *V2) SetDisconnectHandler(f entity.DisconnectHandler) {
	s.disconnect = f
}

func (s *V2) SetEncryptorGetter(f entity.EncryptorGetter) {
	s.getUserEncryptor = f
}

func (s *V2) SetStatisticHandler(f entity.StatisticHandler) {
	s.addSessionStatistic = f
}

func (s *V2) DropSession(sessionID uint32) {
	s.dsMux.Lock()
	defer s.dsMux.Unlock()
	s.droppedSessions[sessionID] = struct{}{}
}
