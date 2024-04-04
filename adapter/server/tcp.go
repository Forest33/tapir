package server

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/util/hash"
)

func (s *V1) listenerTCP(lst *net.TCPListener) {
	go func() {
		for {
			conn, err := lst.AcceptTCP()
			if err != nil {
				s.log.Error().Err(err).Msg("failed to accept")
				continue
			}

			if err := conn.SetReadBuffer(s.cfg.WriteBufferSize); err != nil {
				s.log.Error().Err(err).Msg("failed to set write buffer size")
				return
			}
			if err := conn.SetWriteBuffer(s.cfg.ReadBufferSize); err != nil {
				s.log.Error().Err(err).Msg("failed to set read buffer size")
				return
			}

			isMultipathTCP, _ := conn.MultipathTCP()
			s.log.Info().
				Str("addr", conn.RemoteAddr().String()).
				Bool("mptcp", isMultipathTCP).
				Msgf("connection accepted")

			s.receiverTCP(conn)
		}
	}()
}

func (s *V1) receiverTCP(conn *net.TCPConn) {
	var (
		err        error
		connection = &entity.Connection{
			TCPConn: conn,
			Addr:    conn.RemoteAddr(),
			Port:    uint16(conn.LocalAddr().(*net.TCPAddr).Port),
			Proto:   entity.ProtoTCP,
		}
	)

	go func() {
		defer func() {
			s.disconnect(connection, err)
			if err := conn.Close(); err != nil {
				s.log.Error().Err(err).Msgf("failed to close connection")
			} else {
				s.log.Debug().Str("addr", conn.RemoteAddr().String()).Msg("connection closed")
			}
		}()

		var (
			headerLength    = s.cfg.PrimaryEncryptor.GetLength(entity.HeaderSize)
			header          = make([]byte, headerLength)
			receivedHeader  int
			receivedPayload int
			decryptedHeader []byte
			enc             entity.Encryptor
		)

		for {
			n, err := conn.Read(header[receivedHeader:])
			if err != nil {
				if entity.IsErrorInterruptingNetwork(err) {
					return
				}
				s.log.Error().Err(err).Msg("failed to read header from socket")
				continue
			}
			if receivedHeader+n < headerLength {
				receivedHeader += n
				continue
			}

			receivedHeader = 0
			receivedPayload = 0

			decryptedHeader, err = s.cfg.PrimaryEncryptor.Decrypt(header)
			if err != nil {
				s.log.Error().Err(err).
					Int("header_size", len(header)).
					Str("key", s.cfg.PrimaryEncryptor.GetKey()).
					Str("header", fmt.Sprintf("% x", header)).
					Msg("failed to decrypt header")
				return
			}

			msg := &entity.Message{}
			if err = s.cfg.Codec.UnmarshalHeader(decryptedHeader, msg); err != nil {
				s.log.Error().Err(err).
					Int("encrypt_header_size", len(header)).
					Int("header_size", len(decryptedHeader)).
					Str("key", s.cfg.PrimaryEncryptor.GetKey()).
					Str("header", fmt.Sprintf("% x", decryptedHeader)).
					Msg("failed to unmarshal header")
				return
			}

			if msg.PayloadLength > 0 {
				msg.Payload = make([]byte, msg.PayloadLength)
				for receivedPayload < int(msg.PayloadLength) {
					n, err = conn.Read(msg.Payload.([]byte)[receivedPayload:])
					if err != nil {
						if errors.Is(err, io.EOF) {
							return
						}
						s.log.Error().Err(err).Msg("failed to read payload from socket")
						continue
					}
					receivedPayload += n
				}
			}

			s.addSessionStatistic(msg.SessionID, &entity.Statistic{
				OutgoingBytes:  uint64(receivedHeader + receivedPayload),
				OutgoingFrames: 1,
			})

			if msg.IsUserData() {
				enc, err = s.getUserEncryptor(connection)
				if err != nil {
					s.log.Error().Err(err).Uint32("session_id", connection.SessionID).Msg("failed to get connection encryptor")
					return
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
						Str("key", hash.MD5([]byte(enc.GetKey()))).
						Msg("failed to decrypt payload")
					continue
				}

				if err := s.cfg.Codec.UnmarshalPayload(msg); err != nil {
					s.log.Error().Err(err).Msg("failed to unmarshal payload")
					continue
				}

				if msg.IsUserData() {
					msg.PacketInfo, err = s.packetDecoder.Decode(msg.Payload.([]byte))
					if err != nil {
						continue
					}
				}
			}

			s.receiveMessageLog(msg.Payload, headerLength+receivedPayload, msg)

			if err = s.receiver(msg, connection); err != nil {
				return
			}
		}
	}()
}

func (s *V1) sendTCP(msg *entity.Message, userEncryptor entity.Encryptor, conn *net.TCPConn) error {
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

	s.sendMessageLog(nil, sent, msg, "sent to socket")

	//entity.MessagePool.Put(msg)

	return err
}
