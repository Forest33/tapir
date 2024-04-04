package client

import (
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/util/hash"
)

func (c *Client) sendMessageLog(data []byte, size int, msg *entity.Message) {
	if !c.cfg.SocketTracing {
		return
	}

	ev := c.log.Debug()
	if msg != nil {
		ev = ev.Uint32("id", msg.ID).
			Uint64("endpoint", msg.GetEndpoint().Uint64()).
			Uint32("session_id", msg.SessionID).
			Str("type", msg.Type.String()).
			Bool("ack", msg.IsACK).
			Uint16("payload_size", msg.PayloadLength).
			Uint8("compression", uint8(msg.CompressionType))
		if msg.IsACK && msg.Payload != nil {
			ev = ev.Int("ack_size", msg.Payload.(*entity.MessageAcknowledgement).GetMessagesCount()).
				Interface("ack_id", msg.Payload.(*entity.MessageAcknowledgement).Get())
		}
	}
	ev = ev.Int("size", size)
	if data != nil {
		ev = ev.Str("hash", hash.MD5(data))
	}
	ev.Msg("sent to socket")
}

func (c *Client) receiveMessageLog(data interface{}, size int, msg *entity.Message) {
	if !c.cfg.SocketTracing {
		return
	}

	ev := c.log.Debug().Uint32("id", msg.ID).
		Uint64("endpoint", msg.GetEndpoint().Uint64()).
		Int("size", size).
		Uint16("payload_size", msg.PayloadLength).
		Str("type", msg.Type.String()).
		Bool("ack", msg.IsACK).
		Uint8("compression", uint8(msg.CompressionType))
	if msg.IsACK && msg.Payload != nil {
		ev = ev.Int("ack_size", msg.Payload.(*entity.MessageAcknowledgement).GetMessagesCount()).
			Interface("ack_id", msg.Payload.(*entity.MessageAcknowledgement).Get())
	}
	if data != nil {
		if d, ok := data.([]byte); ok {
			ev = ev.Str("hash", hash.MD5(d))
		}
	}
	ev.Msg("received from socket")
}
