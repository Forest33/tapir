//go:build !debug

package server

import (
	"fmt"
	"net"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/util/hash"
)

func (s *V1) receiverUDP(conn *net.UDPConn) {
	var (
		connectionControl = make(map[uint32]connControl, 10)
		decryptedHeader   []byte
		enc               entity.Encryptor
		maxSessionsCount  = s.cfg.MaxSessionsCount
		curSessionsCount  int
		connPort          = uint16(conn.LocalAddr().(*net.UDPAddr).Port)
	)

	go func() {
		var (
			headerLength = s.cfg.PrimaryEncryptor.GetLength(entity.HeaderSize)
			buf          = make([]byte, s.cfg.PrimaryEncryptor.GetLength(s.cfg.MTU)+headerLength)
		)

		for {
			n, addr, err := conn.ReadFrom(buf)
			if err != nil {
				if entity.IsErrorInterruptingNetwork(err) {
					return
				}
				s.log.Error().Err(err).Msg("failed to read header from socket")
				continue
			}
			if n < headerLength {
				continue
			}

			connection := &entity.Connection{
				UDPConn: conn,
				Addr:    addr,
				Port:    connPort,
				Proto:   entity.ProtoUDP,
			}

			decryptedHeader, err = s.cfg.PrimaryEncryptor.Decrypt(buf[:headerLength])
			if err != nil {
				s.log.Error().Err(err).
					Str("header", fmt.Sprintf("% x", buf[:headerLength])).
					Str("local", conn.LocalAddr().String()).
					Str("remote", addr.String()).
					Msg("failed to decrypt header")
				s.disconnect(connection, err)
				continue
			}

			msg := &entity.Message{}
			if err = s.cfg.Codec.UnmarshalHeader(decryptedHeader, msg); err != nil {
				s.log.Error().Err(err).
					Int("header_size", len(decryptedHeader)).
					Str("header", fmt.Sprintf("% x", decryptedHeader)).
					Str("local", conn.LocalAddr().String()).
					Str("remote", addr.String()).
					Msg("failed to unmarshal header")
				s.disconnect(connection, err)
				continue
			}

			if int(msg.PayloadLength)+headerLength != n {
				s.log.Error().Err(err).
					Uint16("payload_size", msg.PayloadLength).
					Int("message_size", len(buf)).
					Str("local", conn.LocalAddr().String()).
					Str("remote", addr.String()).
					Msg("wrong message length")
				continue
			}

			if msg.PayloadLength > 0 {
				msg.Payload = buf[headerLength : headerLength+int(msg.PayloadLength)]
			}

			s.addSessionStatistic(msg.SessionID, &entity.Statistic{
				OutgoingBytes:  uint64(n),
				OutgoingFrames: 1,
			})

			connection.SessionID = msg.SessionID

			if msg.IsUserData() {
				enc, err = s.getUserEncryptor(connection)
				if err != nil {
					s.log.Error().Err(err).
						Uint32("session_id", connection.SessionID).
						Str("local", conn.LocalAddr().String()).
						Str("remote", addr.String()).
						Msg("failed to get connection encryptor")
					continue
				} else if enc == nil {
					continue
				}
			} else {
				enc = s.cfg.PrimaryEncryptor
			}

			if msg.IsPayload() {
				msg.Payload, err = enc.Decrypt(msg.Payload)
				if err != nil {
					s.log.Error().Err(err).
						Str("local", conn.LocalAddr().String()).
						Str("remote", addr.String()).
						Str("key", hash.MD5([]byte(enc.GetKey()))).
						Msg("failed to decrypt payload")
					continue
				}

				if err := s.cfg.Codec.UnmarshalPayload(msg); err != nil {
					s.log.Error().Err(err).
						Str("local", conn.LocalAddr().String()).
						Str("remote", addr.String()).
						Msg("failed to unmarshal payload")
					continue
				}

				if msg.IsUserData() {
					msg.PacketInfo, err = s.packetDecoder.Decode(msg.Payload.([]byte))
					if err != nil {
						continue
					}
				}
			}

			if msg.SessionID != 0 {
				if cc, ok := connectionControl[msg.SessionID]; !ok {
					connection.Retry = s.retryFactory(s.ctx, s.originalLog, s.send, s.sendKeepalive, s.disconnect, connection)
					connection.Ack = s.ackFactory(s.ctx, s.originalLog, s.sendUDP, connection, msg.SessionID)
					connectionControl[msg.SessionID] = connControl{
						retry: connection.Retry,
						ack:   connection.Ack,
					}
					curSessionsCount++
				} else {
					connection.Retry = cc.retry
					connection.Ack = cc.ack
				}
			}

			s.receiveMessageLog(msg.Payload, n, msg)

			if msg.Type == entity.MessageTypeKeepalive {
				if !msg.IsACK {
					connection.Retry.Keepalive()
				} else {
					connection.Retry.Ack(nil)
				}
				continue
			}

			if msg.IsACK {
				connection.Retry.Ack(msg.Payload.(*entity.MessageAcknowledgement))
				continue
			} else if msg.IsSendACK() {
				connection.Ack.Push(msg.ID, msg.GetEndpoint()) // panic: runtime error: invalid memory address or nil pointer dereference
			}

			if err = s.receiver(msg, connection); err != nil {
				if msg.Type != entity.MessageTypeData {
					return
				}
			}

			if curSessionsCount > maxSessionsCount {
				s.dsMux.Lock()
				for sessionID := range s.droppedSessions {
					delete(connectionControl, sessionID)
					delete(s.droppedSessions, sessionID)
					curSessionsCount--
				}
				s.dsMux.Unlock()
				if curSessionsCount > maxSessionsCount {
					maxSessionsCount++
					s.log.Debug().Int("max_sessions_count", maxSessionsCount).Msg("increasing max sessions count")
				}
			}
		}
	}()
}

func (s *V1) sendUDP(msg *entity.Message, userEncryptor entity.Encryptor, conn *entity.Connection, retry entity.NetworkRetry) error {
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

//func (s *V1) send(data []byte, msg *entity.Message, conn *net.UDPConn, addr net.Addr) error {
//	var (
//		sent, n int
//		err     error
//	)
//
//	for sent < len(data) {
//		n, err = conn.WriteTo(data[sent:], addr)
//		if err != nil {
//			return err
//		}
//		sent += n
//	}
//
//	s.sendMessageLog(data[entity.HeaderSize:], sent, msg, "sent to socket")
//
//	return nil
//}

func (s *V1) send(data []byte, msg *entity.Message, conn *entity.Connection) error {
	var (
		sent, n int
		err     error
	)

	for sent < len(data) {
		n, err = conn.UDPConn.WriteTo(data[sent:], conn.Addr)
		if err != nil {
			return err
		}
		sent += n
	}

	s.sendMessageLog(data[entity.HeaderSize:], sent, msg, "sent to socket")

	return nil
}

func (s *V1) sendKeepalive(conn *entity.Connection, ack bool) {
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
