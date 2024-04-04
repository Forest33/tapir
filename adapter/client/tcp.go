package client

import (
	"errors"
	"io"
	"net"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/util/hash"
)

func (c *Client) ReceiverTCP(conn *entity.Connection, sessionID uint32) {
	var (
		err      error
		msg      *entity.Message
		enc      entity.Encryptor
		received int
	)

	conn.SessionID = sessionID
	conn.Proto = entity.ProtoTCP

	go func() {
		defer func() {
			c.log.Info().
				Uint32("session_id", sessionID).
				Str("local", conn.TCPConn.LocalAddr().String()).
				Str("remote", conn.TCPConn.RemoteAddr().String()).
				Msg("connection finished")
		}()

		for {
			msg, received, err = c.readTCP(conn.TCPConn)
			if err != nil {
				if entity.IsErrorInterruptingNetwork(err) {
					return
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
					c.log.Error().Err(err).
						Str("key", hash.MD5([]byte(enc.GetKey()))).
						Msg("failed to decrypt payload")
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

			c.addSessionStatistic(msg.SessionID, &entity.Statistic{
				IncomingBytes:  uint64(received),
				IncomingFrames: 1,
			})

			c.receiveMessageLog(msg.Payload, received, msg)

			if err = c.receiver(msg, conn); err != nil {
				return
			}
		}
	}()
}

func (c *Client) readTCP(conn *net.TCPConn) (*entity.Message, int, error) {
	var (
		headerLength    = c.cfg.PrimaryEncryptor.GetLength(entity.HeaderSize)
		header          = make([]byte, headerLength)
		receivedHeader  int
		receivedPayload int
		decryptedHeader []byte
		err             error
	)

	for {
		n, err := conn.Read(header[receivedHeader:])
		if err != nil {
			if entity.IsErrorInterruptingNetwork(err) {
				return nil, 0, err
			}
			c.log.Error().Err(err).Msg("failed to read header from socket")
			continue
		}
		if receivedHeader+n < headerLength {
			receivedHeader += n
			continue
		}
		break
	}

	decryptedHeader, err = c.cfg.PrimaryEncryptor.Decrypt(header)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to decrypt header")
		return nil, 0, nil
	}

	msg := &entity.Message{}
	if err = c.cfg.Codec.UnmarshalHeader(decryptedHeader, msg); err != nil {
		c.log.Error().Err(err).Msg("failed to unmarshal header")
		return nil, 0, nil
	}

	if msg.PayloadLength > 0 {
		msg.Payload = make([]byte, msg.PayloadLength)
		for receivedPayload < int(msg.PayloadLength) {
			n, err := conn.Read(msg.Payload.([]byte)[receivedPayload:])
			if err != nil {
				if errors.Is(err, io.EOF) {
					return nil, 0, err
				}
				c.log.Error().Err(err).Msg("failed to read payload from socket")
				continue
			}
			receivedPayload += n
		}
	}

	return msg, headerLength + receivedPayload, nil
}

func (c *Client) sendAsyncTCP(msg *entity.Message, userEncryptor entity.Encryptor, conn *net.TCPConn) error {
	encodedHeader, encodedPayload, err := c.cfg.Codec.Marshal(msg)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encode message")
		return err
	}

	cypherHeader, err := c.cfg.PrimaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encrypt header")
		return err
	}

	var cypherPayload []byte
	if userEncryptor != nil {
		cypherPayload, err = userEncryptor.Encrypt(encodedPayload)
	} else {
		cypherPayload, err = c.cfg.PrimaryEncryptor.Encrypt(encodedPayload)
	}
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encrypt payload")
		return err
	}

	var (
		data    = append(cypherHeader, cypherPayload...)
		sent, n int
	)

	for sent < len(data) {
		n, err = conn.Write(data[sent:])
		if err != nil {
			return err
		}
		sent += n
	}

	c.sendMessageLog(nil, sent, msg)

	return err
}

func (c *Client) sendSyncTCP(msg *entity.Message, conn *entity.Connection, timeout time.Duration) (*entity.Message, *entity.Connection, error) {
	encodedHeader, encodedPayload, err := c.cfg.Codec.Marshal(msg)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encode message")
		return nil, nil, err
	}

	cypherHeader, err := c.cfg.PrimaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encrypt header")
		return nil, nil, err
	}

	cypherPayload, err := c.cfg.PrimaryEncryptor.Encrypt(encodedPayload)
	if err != nil {
		c.log.Error().Err(err).Msg("failed to encrypt payload")
		return nil, nil, err
	}

	var (
		data    = append(cypherHeader, cypherPayload...)
		sent, n int
	)

	for sent < len(data) {
		n, err = conn.TCPConn.Write(data[sent:])
		if err != nil {
			return nil, nil, err
		}
		sent += n
	}

	c.sendMessageLog(nil, sent, msg)

	if err := conn.TCPConn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, nil, err
	}

	reply, n, err := c.readTCP(conn.TCPConn)
	if err != nil {
		return nil, nil, err
	}

	c.receiveMessageLog(msg.Payload, n, msg)

	if err := conn.TCPConn.SetReadDeadline(time.Time{}); err != nil {
		return nil, nil, err
	}

	conn.Addr = conn.TCPConn.RemoteAddr()

	return reply, conn, nil
}
