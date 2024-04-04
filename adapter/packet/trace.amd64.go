//go:build amd64 && !windows

package packet

import (
	_ "github.com/google/gopacket/layers"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"golang.org/x/sys/unix"

	"github.com/forest33/tapir/business/entity"
)

func decodeLayers(data []byte, pi *entity.NetworkPacketInfo) (*entity.NetworkPacketInfo, error) {
	var (
		ip4     layers.IPv4
		tcp     layers.TCP
		udp     layers.UDP
		icmp    layers.ICMPv4
		tls     layers.TLS
		payload gopacket.Payload
	)

	parser := gopacket.NewDecodingLayerParser(layers.LayerTypeIPv4, &ip4, &tcp, &udp, &tls, &icmp, &payload)
	parser.IgnoreUnsupported = true

	decodedLayers := make([]gopacket.LayerType, 0, 5)

	_ = parser.DecodeLayers(data, &decodedLayers)

	for _, typ := range decodedLayers {
		switch typ {
		case layers.LayerTypeIPv4:
			pi.IP = &entity.IP{
				Version:  unix.AF_INET,
				Src:      ip4.SrcIP.String(),
				Dst:      ip4.DstIP.String(),
				ID:       ip4.Id,
				Protocol: ip4.Protocol.String(),
				Length:   ip4.Length,
			}
		case layers.LayerTypeTCP:
			pi.TCP = &entity.TCP{
				Src:    tcp.SrcPort.String(),
				Dst:    tcp.DstPort.String(),
				Seq:    tcp.Seq,
				Length: len(tcp.Contents),
			}
		case layers.LayerTypeUDP:
			pi.UDP = &entity.UDP{
				Src:    udp.SrcPort.String(),
				Dst:    udp.DstPort.String(),
				Length: udp.Length,
			}
		case layers.LayerTypeTLS:
			pi.TLS = &entity.TLS{Length: len(tls.Contents)}
		case layers.LayerTypeICMPv4:
			pi.ICMPv4 = &entity.ICMP{
				Seq:    icmp.Seq,
				Id:     icmp.Id,
				Type:   icmp.TypeCode.String(),
				Length: len(icmp.Contents),
			}
			if len(data) != int(ip4.Length) {
				pi.ICMPv4.Error = entity.ErrWrongPacketLength
			}
		}
	}

	return pi, nil
}
