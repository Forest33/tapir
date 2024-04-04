package wiface

import (
	"github.com/rs/zerolog"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/util/hash"
)

func (i *Iface) SendLog(msg *entity.Message, ifName string) {
	if !i.cfg.Tracing {
		return
	}

	pi, err := i.packetDecoder.Decode(msg.Payload.([]byte))
	if err != nil {
		i.log.Error().Err(err).
			Uint32("id", msg.ID).
			Int("size", len(msg.Payload.([]byte))).
			Msg("failed to decode packet")
	}

	ev := i.log.Debug().
		Uint32("id", msg.ID).
		Uint64("endpoint", msg.GetEndpoint().Uint64()).
		Int("size", len(msg.Payload.([]byte))).
		Uint8("compression", uint8(msg.CompressionType)).
		Str("if", ifName)
	ev = i.packetLog(pi, ev)
	ev = ev.Str("hash", hash.MD5(msg.Payload.([]byte)))
	ev.Msg("sent to interface")
}

func (i *Iface) ReceiveLog(msg *entity.Message) {
	if !i.cfg.Tracing {
		return
	}

	ev := i.log.Debug().
		Uint32("id", msg.ID).
		Uint64("endpoint", msg.GetEndpoint().Uint64()).
		Int("size", len(msg.Payload.([]byte))).
		Str("if", msg.PacketInfo.IfName)
	ev = i.packetLog(msg.PacketInfo, ev)
	ev = ev.Str("hash", hash.MD5(msg.Payload.([]byte)))
	ev.Msg("received from interface")
}

func (i *Iface) packetLog(pi *entity.NetworkPacketInfo, ev *zerolog.Event) *zerolog.Event {
	if pi == nil {
		return ev
	}

	if pi.IP != nil {
		ev = ev.Interface("IP", pi.IP)
		if pi.IP.Error != nil {
			ev = ev.Err(pi.IP.Error)
		}
	}
	if pi.TCP != nil {
		ev = ev.Interface("TCP", pi.TCP)
		if pi.TCP.Error != nil {
			ev = ev.Err(pi.TCP.Error)
		}
	}
	if pi.UDP != nil {
		ev = ev.Interface("UDP", pi.UDP)
		if pi.UDP.Error != nil {
			ev = ev.Err(pi.UDP.Error)
		}
	}
	if pi.TLS != nil {
		ev = ev.Interface("TLS", pi.TLS)
		if pi.TLS.Error != nil {
			ev = ev.Err(pi.TLS.Error)
		}
	}
	if pi.ICMPv4 != nil {
		ev = ev.Interface("ICMPv4", pi.ICMPv4)
		if pi.ICMPv4.Error != nil {
			ev = ev.Err(pi.ICMPv4.Error)
		}
	}

	return ev
}
