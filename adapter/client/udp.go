package client

import (
	"net"
	"time"

	"github.com/forest33/tapir/business/entity"
)

func (c *Client) ReceiverUDP(conn *entity.Connection, sessionID uint32) {
	var (
		err  error
		msg  *entity.Message
		addr net.Addr
		enc  entity.Encryptor
		n    int
	)

	conn.SessionID = sessionID
	conn.Retry = c.retryFactory(c.ctx, c.originalLog, c.send, c.sendKeepalive, c.disconnect, conn)
	conn.Ack = c.ackFactory(c.ctx, c.originalLog, c.sendAsyncUDP, conn, sessionID)
	conn.Proto = entity.ProtoUDP

	c.log.Info().
		Uint32("session_id", sessionID).
		Str("local", conn.UDPConn.LocalAddr().String()).
		Str("remote", conn.UDPConn.RemoteAddr().String()).
		Msg("connection started")

	go func() {
		defer func() {
			conn.Retry.Stop()
			conn.Ack.Stop()
			c.log.Info().
				Uint32("session_id", sessionID).
				Str("local", conn.UDPConn.LocalAddr().String()).
				Str("remote", conn.UDPConn.RemoteAddr().String()).
				Msg("connection finished")
		}()

		for {
			msg, addr, n, err = c.readUDP(conn.UDPConn)
			if err != nil {
				if entity.IsErrorInterruptingNetwork(err) {
					return
				}
				continue
			}

			conn.Addr = addr

			c.addSessionStatistic(msg.SessionID, &entity.Statistic{
				IncomingBytes:  uint64(n),
				IncomingFrames: 1,
			})

			if msg.Type == entity.MessageTypeKeepalive {
				if !msg.IsACK {
					conn.Retry.Keepalive()
				} else {
					conn.Retry.Ack(nil)
				}
				continue
			}

			if msg.IsUserData() {
				enc, err = c.getUserEncryptor(conn)
				if err != nil {
					c.log.Error().Err(err).Uint32("session_id", conn.SessionID).Msg("failed to get connection encryptor")
					return
				} else if enc == nil {
					continue
				}
			} else {
				enc = c.cfg.PrimaryEncryptor
			}

			if msg.IsPayload() {
				msg.Payload, err = enc.Decrypt(msg.Payload)
				if err != nil {
					c.log.Error().Err(err).Msg("failed to decrypt payload")
					continue
				}

				if err := c.cfg.Codec.UnmarshalPayload(msg); err != nil {
					c.log.Error().Err(err).Msg("failed to unmarshal payload")
					continue
				}

				if msg.IsUserData() {
					msg.PacketInfo, err = c.packetDecoder.Decode(msg.Payload.([]byte))
					if err != nil {
						continue
					}
				}
			}

			c.receiveMessageLog(msg.Payload, n, msg)

			if msg.IsACK {
				conn.Retry.Ack(msg.Payload.(*entity.MessageAcknowledgement))
				continue
			} else if msg.IsSendACK() {
				conn.Ack.Push(msg.ID, msg.GetEndpoint())
			}

			if err = c.receiver(msg, conn); err != nil {
				if msg.Type != entity.MessageTypeData {
					return
				}
			}
		}
	}()
}

func (c *Client) readUDP(conn *net.UDPConn) (*entity.Message, net.Addr, int, error) {
	var (
		headerLength    = c.cfg.PrimaryEncryptor.GetLength(entity.HeaderSize)
		buf             = make([]byte, c.cfg.PrimaryEncryptor.GetLength(c.cfg.MTU)+headerLength)
		decryptedHeader []byte
	)

	n, addr, err := conn.ReadFrom(buf)
	if err != nil {
		return nil, nil, 0, err
	}
	if n < headerLength {
		return nil, nil, 0, nil
	}

	decryptedHeader, err = c.cfg.PrimaryEncryptor.Decrypt(buf[:headerLength])
	if err != nil {
		c.log.Error().Err(err).Msg("failed to decrypt header")
		return nil, nil, 0, nil
	}

	msg := &entity.Message{}
	if err = c.cfg.Codec.UnmarshalHeader(decryptedHeader, msg); err != nil {
		c.log.Error().Err(err).Msg("failed to unmarshal header")
		return nil, nil, 0, nil
	}

	if int(msg.PayloadLength)+headerLength != n {
		c.log.Error().Err(err).
			Uint16("payload_size", msg.PayloadLength).
			Int("message_size", len(buf)).
			Msg("wrong message length")
		return nil, nil, 0, nil
	}

	if msg.PayloadLength > 0 {
		msg.Payload = buf[headerLength : headerLength+int(msg.PayloadLength)]
	}

	if msg.IsACK {
		if msg.Type == entity.MessageTypeKeepalive {
			c.receiveMessageLog(buf[headerLength:headerLength+int(msg.PayloadLength)], n, msg)
			return msg, addr, 0, nil
		}
	}

	return msg, addr, n, nil
}

func (c *Client) sendAsyncUDP(msg *entity.Message, userEncryptor entity.Encryptor, conn *entity.Connection, retry entity.NetworkRetry) error {
	encodedHeader, encodedPayload, err := c.cfg.Codec.Marshal(msg)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encode message")
		return err
	}

	var cypherPayload []byte
	if len(encodedPayload) != 0 {
		if userEncryptor != nil {
			cypherPayload, err = userEncryptor.Encrypt(encodedPayload)
		} else {
			cypherPayload, err = c.cfg.PrimaryEncryptor.Encrypt(encodedPayload)
		}
		if err != nil {
			c.log.Error().Err(err).Msg("failed to encrypt payload")
			return err
		}
	}

	cypherHeader, err := c.cfg.PrimaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encrypt header")
		return err
	}

	data := append(cypherHeader, cypherPayload...)

	err = c.send(data, msg, conn)
	if err != nil {
		return err
	}

	if msg.IsSendACK() {
		msg.Payload = data
		retry.Push(msg)
	}

	return nil
}

func (c *Client) sendSyncUDP(msg *entity.Message, conn *entity.Connection, timeout time.Duration) (*entity.Message, *entity.Connection, error) {
	encodedHeader, encodedPayload, err := c.cfg.Codec.Marshal(msg)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encode message")
		return nil, nil, err
	}

	var cypherPayload []byte
	if len(encodedPayload) != 0 {
		cypherPayload, err = c.cfg.PrimaryEncryptor.Encrypt(encodedPayload)
		if err != nil {
			c.log.Error().Err(err).Msg("failed to encrypt payload")
			return nil, nil, err
		}
	}

	cypherHeader, err := c.cfg.PrimaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encrypt header")
		return nil, nil, err
	}

	data := append(cypherHeader, cypherPayload...)

	if err := conn.UDPConn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return nil, nil, err
	}

	err = c.send(data, msg, conn)
	if err != nil {
		return nil, nil, err
	}

	if err := conn.UDPConn.SetWriteDeadline(time.Time{}); err != nil {
		return nil, nil, err
	}
	if err := conn.UDPConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, nil, err
	}

	reply, addr, n, err := c.readUDP(conn.UDPConn)
	if err != nil {
		return nil, nil, err
	}

	c.receiveMessageLog(msg.Payload, n, msg)

	if err := conn.UDPConn.SetReadDeadline(time.Time{}); err != nil {
		return nil, nil, err
	}

	conn.Addr = addr

	return reply, conn, nil
}

func (c *Client) send(data []byte, msg *entity.Message, conn *entity.Connection) error {
	var (
		sent, n int
		err     error
	)

	for sent < len(data) {
		n, err = conn.UDPConn.Write(data[sent:])
		if err != nil { // TODO check error level
			return err
		}
		sent += n
	}

	c.sendMessageLog(data[entity.HeaderSize:], sent, msg)

	return nil
}

func (c *Client) sendKeepalive(conn *entity.Connection, ack bool) {
	msg := &entity.Message{
		SessionID: conn.SessionID,
		Type:      entity.MessageTypeKeepalive,
		IsACK:     ack,
	}

	encodedHeader, _, err := c.cfg.Codec.Marshal(msg)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encode keepalive message")
		return
	}

	cypherHeader, err := c.cfg.PrimaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encrypt keepalive header")
		return
	}

	err = c.send(cypherHeader, msg, conn)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to send keepalive")
	}
}
