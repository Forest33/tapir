//go:build amd64

package wiface

import (
	"errors"
	"os"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"

	"github.com/forest33/tapir/business/entity"
)

type MockNetworkInterface struct {
	name      string
	packets   [][]byte
	packetIdx int
	packetLen int
	run       bool
}

func (m *MockNetworkInterface) Name() (string, error) {
	return m.name, nil
}

func (*MockNetworkInterface) Close() error {
	return nil
}

func (m *MockNetworkInterface) Read(p []byte) (int, error) {
	if m.packetIdx == m.packetLen {
		m.packetIdx = 0
	}
	n := len(m.packets[m.packetIdx])
	copy(p, m.packets[m.packetIdx])
	m.packetIdx++
	return n, nil
}

func (*MockNetworkInterface) Write(p []byte) (n int, err error) {
	return 0, nil
}

func CreateMockNetworkInterface(mtu int) (entity.InterfaceHandler, error) {
	pcapPath, ok := os.LookupEnv("TAPIR_PCAP")
	if !ok {
		return nil, errors.New("environment variable TAPIR_PCAP is not set")
	}

	handle, err := pcap.OpenOffline(pcapPath)
	if err != nil {
		return nil, err
	}
	defer handle.Close()

	source := gopacket.NewPacketSource(handle, handle.LinkType())
	packets := make([][]byte, 0, 100)

	for packet := range source.Packets() {
		ipLayer := packet.Layer(layers.LayerTypeIPv4)
		if ipLayer != nil {
			ipPacket, _ := ipLayer.(*layers.IPv4)
			data := append(ipPacket.Contents, ipPacket.Payload...)
			if len(data) > mtu {
				continue
			}
			packets = append(packets, data)
		}
	}

	return &MockNetworkInterface{
		name:      "mock",
		packets:   packets,
		packetLen: len(packets),
	}, nil
}
