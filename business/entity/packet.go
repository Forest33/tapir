package entity

type PacketDecoder interface {
	Decode(data []byte) (*NetworkPacketInfo, error)
}

type PacketEndpoint uint64

func (e PacketEndpoint) Uint64() uint64 {
	return uint64(e)
}

type NetworkPacketInfo struct {
	Endpoint PacketEndpoint
	Protocol IPProtocol
	IP       *IP
	TCP      *TCP
	UDP      *UDP
	TLS      *TLS
	ICMPv4   *ICMP
	IfName   string
}

type IP struct {
	Version  int
	Src      string
	Dst      string
	ID       uint16
	Protocol string
	Length   uint16
	Error    error
}

type TCP struct {
	Src    string
	Dst    string
	Seq    uint32
	Length int
	Error  error
}

type UDP struct {
	Src    string
	Dst    string
	Length uint16
	Error  error
}

type TLS struct {
	Length int
	Error  error
}

type ICMP struct {
	Seq    uint16
	Id     uint16
	Type   string
	Length int
	Error  error
}

type IPProtocol uint8

const (
	IPProtocolIPv6HopByHop    IPProtocol = 0
	IPProtocolICMPv4          IPProtocol = 1
	IPProtocolIGMP            IPProtocol = 2
	IPProtocolIPv4            IPProtocol = 4
	IPProtocolTCP             IPProtocol = 6
	IPProtocolUDP             IPProtocol = 17
	IPProtocolRUDP            IPProtocol = 27
	IPProtocolIPv6            IPProtocol = 41
	IPProtocolIPv6Routing     IPProtocol = 43
	IPProtocolIPv6Fragment    IPProtocol = 44
	IPProtocolGRE             IPProtocol = 47
	IPProtocolESP             IPProtocol = 50
	IPProtocolAH              IPProtocol = 51
	IPProtocolICMPv6          IPProtocol = 58
	IPProtocolNoNextHeader    IPProtocol = 59
	IPProtocolIPv6Destination IPProtocol = 60
	IPProtocolOSPF            IPProtocol = 89
	IPProtocolIPIP            IPProtocol = 94
	IPProtocolEtherIP         IPProtocol = 97
	IPProtocolVRRP            IPProtocol = 112
	IPProtocolSCTP            IPProtocol = 132
	IPProtocolUDPLite         IPProtocol = 136
	IPProtocolMPLSInIP        IPProtocol = 137
)
