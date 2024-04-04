package server

import (
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/util/hash"
)

func (s *V1) sendMessageLog(data []byte, size int, msg *entity.Message, description string) {
	if !s.cfg.Tracing {
		return
	}

	ev := s.log.Debug()
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
		if msg.PacketInfo != nil && msg.PacketInfo.IP != nil {
			ev = ev.Str("src", msg.PacketInfo.IP.Src).Str("dst", msg.PacketInfo.IP.Dst)
		}
	}
	ev = ev.Int("size", size)
	if data != nil {
		ev = ev.Str("hash", hash.MD5(data))
	}
	ev.Msg(description)
}

func (s *V1) receiveMessageLog(data interface{}, size int, msg *entity.Message) {
	if !s.cfg.Tracing {
		return
	}

	ev := s.log.Debug().Uint32("id", msg.ID).
		Uint64("endpoint", msg.GetEndpoint().Uint64()).
		Int("size", size).
		Uint16("payload_size", msg.PayloadLength).
		Uint32("session_id", msg.SessionID).
		Str("type", msg.Type.String()).
		Bool("ack", msg.IsACK).
		Uint8("compression", uint8(msg.CompressionType))
	if msg.IsACK && msg.Payload != nil {
		ev = ev.Int("ack_size", msg.Payload.(*entity.MessageAcknowledgement).GetMessagesCount()).
			Interface("ack_id", msg.Payload.(*entity.MessageAcknowledgement).Get())
	}
	if msg.PacketInfo != nil && msg.PacketInfo.IP != nil {
		ev = ev.Str("src", msg.PacketInfo.IP.Src).Str("dst", msg.PacketInfo.IP.Dst)
	}
	if data != nil {
		if d, ok := data.([]byte); ok {
			ev = ev.Str("hash", hash.MD5(d))
		}
	}
	ev.Msg("received from socket")
}
