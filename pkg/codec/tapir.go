package codec

import (
	"encoding/binary"
	"math"
	"math/rand"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/compression"
	"github.com/forest33/tapir/pkg/logger"
)

const (
	errorSize                   = 1
	acknowledgementEndpointSize = 9

	authenticationRequestParams          = 5
	authenticationResponseParams         = 2
	handshakeParams                      = 1
	authenticationResponseMinPayloadSize = 14

	headerPositionType      = 0
	headerPositionFlags     = 1
	headerPositionID        = 2
	headerPositionSessionID = 6
	headerPositionLength    = 10

	authRequestIndexClientID         = 0
	authRequestIndexName             = 1
	authRequestIndexPassword         = 2
	authRequestIndexCompressionType  = 3
	authRequestIndexCompressionLevel = 4
	authResponseIndexLocalIP         = 0
	authResponseIndexRemoteIP        = 1
	handshakeIndexKey                = 0
)

const (
	flagNoFlags = 0
	flagError   = 1 << (iota - 1)
	flagRequest
	flagACK
	flagCompressionLZ4
	flagCompressionLZO
	flagCompressionZSTD
)

var (
	byteOrder = binary.BigEndian
)

type Tapir struct {
	log                     *logger.Logger
	cfg                     *Config
	cmp                     *compression.Compressor
	maxEncryptedPayloadSize uint16
}

func NewTapirCodec(log *logger.Logger, cfg *Config) *Tapir {
	return &Tapir{
		log:                     log,
		cfg:                     cfg,
		cmp:                     compression.New(&compression.Config{PayloadSize: cfg.PayloadSize}),
		maxEncryptedPayloadSize: uint16(cfg.GetLength(cfg.PayloadSize)),
	}
}

func (c *Tapir) Marshal(m *entity.Message) ([]byte, []byte, error) {
	var (
		header  = make([]byte, entity.HeaderSize)
		payload []byte
		flags   uint8
		err     error
	)

	header[headerPositionType] = uint8(m.Type)

	if m.Error > 0 {
		flags += flagError
		payload = []byte{byte(m.Error)}
		byteOrder.PutUint16(header[headerPositionLength:], uint16(c.cfg.GetLength(errorSize)))
	}
	if m.IsACK {
		flags += flagACK
	}

	byteOrder.PutUint32(header[headerPositionID:], m.ID)
	byteOrder.PutUint32(header[headerPositionSessionID:], m.SessionID)

	if m.Error == 0 {
		switch m.Type {
		case entity.MessageTypeAuthentication:
			if req, ok := m.Payload.(*entity.MessageAuthenticationRequest); ok {
				payload, err = c.marshalBytes([]byte(req.ClientID), []byte(req.Name), []byte(req.Password), []byte{req.CompressionType.Byte()}, []byte{req.CompressionLevel.Byte()})
				if err != nil {
					return nil, nil, err
				}
				flags += flagRequest
			} else if resp, ok := m.Payload.(*entity.MessageAuthenticationResponse); ok {
				payload = make([]byte, 0, 32)
				sessionID := make([]byte, 4)
				byteOrder.PutUint32(sessionID, resp.SessionID)
				payload = append(payload, sessionID...)
				ips, err := c.marshalBytes(resp.LocalIP, resp.RemoteIP)
				if err != nil {
					return nil, nil, err
				}
				payload = append(payload, ips...)
			} else {
				return nil, nil, entity.ErrWrongMessagePayload
			}
			payload = c.addFakeData(payload)
			byteOrder.PutUint16(header[headerPositionLength:], uint16(c.cfg.GetLength(len(payload))))
		case entity.MessageTypeHandshake:
			if req, ok := m.Payload.(*entity.MessageHandshake); ok {
				payload, err = c.marshalBytes(req.Key)
				if err != nil {
					return nil, nil, err
				}
				payload = c.addFakeData(payload)
				byteOrder.PutUint16(header[headerPositionLength:], uint16(c.cfg.GetLength(len(payload))))
			} else {
				return nil, nil, entity.ErrWrongMessagePayload
			}
		case entity.MessageTypeData:
			var ok bool
			if !m.IsACK {
				if payload, ok = m.Payload.([]byte); ok {
					if m.CompressionType == entity.CompressionLZ4 {
						if payload, ok = c.cmp.CompressLZ4(payload); ok {
							flags += flagCompressionLZ4
						}
					} else if m.CompressionType == entity.CompressionLZO {
						if payload, ok = c.cmp.CompressLZO(payload); ok {
							flags += flagCompressionLZO
						}
					} else if m.CompressionType == entity.CompressionZSTD {
						if payload, ok = c.cmp.CompressZSTD(payload, m.CompressionLevel); ok {
							flags += flagCompressionZSTD
						}
					}
					byteOrder.PutUint16(header[headerPositionLength:], uint16(c.cfg.GetLength(len(payload))))
				} else {
					return nil, nil, entity.ErrWrongMessagePayload
				}
			} else {
				if ack, ok := m.Payload.(*entity.MessageAcknowledgement); ok {
					payload, err = c.MarshalAcknowledgement(ack)
					if err != nil {
						return nil, nil, err
					}
					byteOrder.PutUint16(header[headerPositionLength:], uint16(c.cfg.GetLength(len(payload))))
				} else {
					return nil, nil, entity.ErrWrongMessagePayload
				}
			}
		}
	}

	header[headerPositionFlags] = flags

	return header, payload, nil
}

func (c *Tapir) UnmarshalHeader(data []byte, m *entity.Message) error {
	if len(data) != entity.HeaderSize {
		return entity.ErrWrongMessageHeaderSize
	}

	m.Type = entity.MessageType(data[headerPositionType])
	if !m.Type.Valid() {
		return entity.ErrUnknownCommand
	}

	if data[headerPositionFlags] != flagNoFlags {
		if data[headerPositionFlags]&flagError == flagError {
			m.IsError = true
		}
		if data[headerPositionFlags]&flagRequest == flagRequest {
			m.IsRequest = true
		}
		if data[headerPositionFlags]&flagACK == flagACK {
			m.IsACK = true
		}
		if data[headerPositionFlags]&flagCompressionLZ4 == flagCompressionLZ4 {
			m.CompressionType = entity.CompressionLZ4
		} else if data[headerPositionFlags]&flagCompressionLZO == flagCompressionLZO {
			m.CompressionType = entity.CompressionLZO
		} else if data[headerPositionFlags]&flagCompressionZSTD == flagCompressionZSTD {
			m.CompressionType = entity.CompressionZSTD
		}
	}

	m.ID = byteOrder.Uint32(data[headerPositionID:])
	m.SessionID = byteOrder.Uint32(data[headerPositionSessionID:])

	m.PayloadLength = byteOrder.Uint16(data[headerPositionLength:])
	if m.PayloadLength > c.maxEncryptedPayloadSize {
		c.log.Error().
			Uint16("size", m.PayloadLength).
			Uint16("max_size", c.maxEncryptedPayloadSize).
			Msg(entity.ErrWrongMessagePayloadSize.Error())
		return entity.ErrWrongMessagePayloadSize
	}

	return nil
}

func (c *Tapir) UnmarshalPayload(m *entity.Message) error {
	if m.PayloadLength == 0 {
		return nil
	}

	if m.IsError {
		if m.PayloadLength < errorSize || len(m.Payload.([]byte)) < errorSize {
			return entity.ErrWrongMessagePayload
		}
		m.Error = entity.MessageError(m.Payload.([]byte)[0])
		return nil
	}

	switch m.Type {
	case entity.MessageTypeAuthentication:
		if m.IsRequest {
			fields, err := c.unmarshalBytes(m.Payload.([]byte), 5)
			if err != nil {
				return err
			}
			if len(fields) != authenticationRequestParams {
				return entity.ErrWrongMessagePayload
			}
			m.Payload = &entity.MessageAuthenticationRequest{
				ClientID:         string(fields[authRequestIndexClientID]),
				Name:             string(fields[authRequestIndexName]),
				Password:         string(fields[authRequestIndexPassword]),
				CompressionType:  entity.CompressionType(fields[authRequestIndexCompressionType][0]),
				CompressionLevel: entity.CompressionLevel(fields[authRequestIndexCompressionLevel][0]),
			}
		} else {
			if len(m.Payload.([]byte)) < authenticationResponseMinPayloadSize {
				return entity.ErrWrongMessagePayload
			}
			fields, err := c.unmarshalBytes(m.Payload.([]byte)[4:], 2)
			if err != nil {
				return err
			}
			if len(fields) != authenticationResponseParams {
				return entity.ErrWrongMessagePayload
			}

			m.Payload = &entity.MessageAuthenticationResponse{
				SessionID: byteOrder.Uint32(m.Payload.([]byte)[:4]),
				LocalIP:   fields[authResponseIndexLocalIP],
				RemoteIP:  fields[authResponseIndexRemoteIP],
			}
		}
	case entity.MessageTypeHandshake:
		fields, err := c.unmarshalBytes(m.Payload.([]byte), 1)
		if err != nil {
			return err
		}
		if len(fields) != handshakeParams {
			return entity.ErrWrongMessagePayload
		}
		m.Payload = &entity.MessageHandshake{
			Key: fields[handshakeIndexKey],
		}
	case entity.MessageTypeData:
		var err error
		payload := m.Payload.([]byte)
		if !m.IsACK {
			if m.CompressionType == entity.CompressionLZ4 {
				if payload, err = c.cmp.DecompressLZ4(payload); err != nil {
					return err
				}
			} else if m.CompressionType == entity.CompressionLZO {
				if payload, err = c.cmp.DecompressLZO(payload); err != nil {
					return err
				}
			} else if m.CompressionType == entity.CompressionZSTD {
				if payload, err = c.cmp.DecompressZSTD(payload); err != nil {
					return err
				}
			}
			m.Payload = payload
		} else {
			if m.Payload, err = c.UnmarshalAcknowledgement(payload); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Tapir) marshalBytes(data ...[]byte) ([]byte, error) {
	res := make([]byte, 0, math.MaxUint8)
	for _, v := range data {
		if len(v)+1 > math.MaxUint8 {
			return nil, entity.ErrMaxBytesSize
		}
		mv := append([]byte{byte(len(v))}, v...)
		res = append(res, mv...)
	}
	return res, nil
}

func (c *Tapir) unmarshalBytes(data []byte, n int) ([][]byte, error) {
	if len(data) == 0 {
		return nil, nil
	}
	res := make([][]byte, 0, 8)
	for len(data) > 0 {
		l := int(data[0])
		if l > len(data)-1 {
			return nil, entity.ErrWrongBytesSize
		}
		v := data[1 : l+1]
		res = append(res, v)
		if len(res) == n {
			return res, nil
		}
		data = data[l+1:]
	}
	return res, nil
}

func (c *Tapir) addFakeData(data []byte) []byte {
	if !c.cfg.ObfuscateData {
		return data
	}
	l := rand.Intn(c.cfg.PayloadSize - len(data))
	fake := make([]byte, l)
	for i := 0; i < l; i++ {
		fake[i] = byte(rand.Intn(math.MaxUint8))
	}
	return append(data, fake...)
}

func (c *Tapir) MarshalAcknowledgement(ack *entity.MessageAcknowledgement) ([]byte, error) {
	res := make([]byte, ack.Size())
	i := 0
	for k, v := range ack.Get() {
		byteOrder.PutUint64(res[i:], k.Uint64())
		res[i+8] = byte(len(v))
		i += acknowledgementEndpointSize
		for _, e := range v {
			byteOrder.PutUint32(res[i:], e)
			i += 4
		}
	}
	return res, nil
}

func (c *Tapir) UnmarshalAcknowledgement(data []byte) (*entity.MessageAcknowledgement, error) {
	l := len(data)
	if l < 13 {
		return nil, entity.ErrWrongMessageAcknowledgement
	}
	res := make(entity.MessageAcknowledgementData, 2)
	for i := 0; i < l; {
		if i+acknowledgementEndpointSize >= l {
			return nil, entity.ErrWrongMessageAcknowledgement
		}
		eLen := int(data[i+8])
		ids := make([]uint32, 0, eLen)
		endpoint := entity.PacketEndpoint(byteOrder.Uint64(data[i : i+8]))
		i += acknowledgementEndpointSize
		eMax := eLen*4 + i
		if eMax > l {
			return nil, entity.ErrWrongMessageAcknowledgement
		}
		for ; i < eMax; i += 4 {
			ids = append(ids, byteOrder.Uint32(data[i:]))
		}
		res[endpoint] = ids
	}
	return entity.NewMessageAcknowledgement(res), nil
}
