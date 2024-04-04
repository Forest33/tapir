package entity

import (
	"net"
	"testing"
)

type connTestData struct {
	in  Connection
	out ConnectionKey
}

func BenchmarkConnectionKeyOld(b *testing.B) {
	data := getNetworkTestData()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := range data {
			k := data[i].in.KeyOld()
			_ = k
		}
	}
}

func BenchmarkConnectionKey(b *testing.B) {
	data := getNetworkTestData()
	b.ResetTimer()
	for n := 0; n < b.N; n++ {
		for i := range data {
			k := data[i].in.Key()
			_ = k
		}
	}
}

func TestConnectionKey(t *testing.T) {
	data := getNetworkTestData()
	for i := range data {
		k := data[i].in.Key()
		if k != data[i].out {
			t.Errorf("wrong key")
		}
	}
}

func getNetworkTestData() []connTestData {
	addrTCP, _ := net.ResolveTCPAddr("tcp", "216.58.211.238:443")
	addrUDP, _ := net.ResolveUDPAddr("udp", "8.8.8.8:53")
	tcp, _ := net.DialTCP("tcp", nil, addrTCP)
	udp, _ := net.DialUDP("udp", nil, addrUDP)

	return []connTestData{
		{
			in: Connection{
				TCPConn: tcp,
				Port:    1977,
				Proto:   ProtoTCP,
			},
			out: ConnectionKey{0x01, 0xb9, 0x07, 0xd8, 0x3a, 0xd3, 0xee, 0xbb, 0x01},
		},
		{
			in: Connection{
				UDPConn: udp,
				Addr:    addrUDP,
				Port:    1977,
				Proto:   ProtoUDP,
			},
			out: ConnectionKey{0x02, 0xb9, 0x07, 0x08, 0x08, 0x08, 0x08, 0x35, 0x00},
		},
	}
}
