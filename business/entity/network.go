package entity

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/panjf2000/gnet/v2"

	"github.com/forest33/tapir/pkg/logger"
)

const (
	HeaderSize        = 12
	connectionKeySize = 9
)

const (
	ProtoTCP Protocol = iota + 1
	ProtoUDP
)

const (
	PortSelectionStrategyRandom PortSelectionStrategy = iota + 1
	PortSelectionStrategyHash
)

const (
	CompressionNone CompressionType = iota
	CompressionLZ4
	CompressionLZO
	CompressionZSTD
)

type CompressionType uint8

func GetCompressionType(name string) CompressionType {
	switch name {
	case CompressionNameNone:
		return CompressionNone
	case CompressionNameLZ4:
		return CompressionLZ4
	case CompressionNameLZO:
		return CompressionLZO
	case CompressionNameZSTD:
		return CompressionZSTD
	default:
		return CompressionNone
	}
}

func (c CompressionType) Byte() byte {
	return byte(c)
}

type CompressionLevel uint8

func (c CompressionLevel) Byte() byte {
	return byte(c)
}

type PortSelectionStrategy uint8

func GetPortSelectionStrategy(name string) PortSelectionStrategy {
	switch name {
	case PortSelectionStrategyNameRandom:
		return PortSelectionStrategyRandom
	case PortSelectionStrategyNameHash:
		return PortSelectionStrategyHash
	default:
		return PortSelectionStrategyRandom
	}
}

type ReceiverHandler func(*Message, *Connection) error
type DisconnectHandler func(*Connection, error)
type ResetHandler func(uint32, *Connection)
type EncryptorGetter func(*Connection) (Encryptor, error)
type GetRTOFunc func() time.Duration
type RetrySender func([]byte, *Message, *Connection) error
type KeepaliveSender func(*Connection, bool)
type AckSender func(*Message, Encryptor, *Connection, NetworkRetry) error
type StatisticHandler func(uint32, *Statistic)

type NetworkRetry interface {
	Push(*Message)
	Ack(*MessageAcknowledgement)
	Keepalive()
	GetRTO() time.Duration
	Stop()
}

type NetworkRetryFactory func(context.Context, *logger.Logger, RetrySender, KeepaliveSender, DisconnectHandler, *Connection) NetworkRetry

type NetworkAck interface {
	Push(uint32, PacketEndpoint)
	Stop()
}

type NetworkAckFactory func(context.Context, *logger.Logger, AckSender, *Connection, uint32) NetworkAck

type StreamMerger interface {
	CreateStream(sessionID uint32) error
	DeleteStream(sessionID uint32)
	Push(msg *Message, conn *Connection) error
	SetReceiverHandler(f ReceiverHandler)
	SetDisconnectHandler(h DisconnectHandler)
	SetResetHandler(h ResetHandler)
}

type Protocol uint8

func (p Protocol) String() string {
	switch p {
	case ProtoTCP:
		return "tcp4"
	case ProtoUDP:
		return "udp4"
	default:
		return "unknown"
	}
}

type NetworkMessage interface {
	Encode()
}

type IfIP struct {
	ServerLocal  net.IP
	ServerRemote net.IP
	ClientLocal  net.IP
	ClientRemote net.IP
}

type Connection struct {
	TCPConn          *net.TCPConn
	UDPConn          *net.UDPConn
	GNetConn         gnet.Conn
	Proto            Protocol
	Addr             net.Addr
	Port             uint16
	Retry            NetworkRetry
	Ack              NetworkAck
	SessionID        uint32
	CreatedAt        int64
	CompressionType  CompressionType
	CompressionLevel CompressionLevel
}

type ConnectionKey [connectionKeySize]byte

// KeyOld
// Deprecated:
func (c *Connection) KeyOld() ConnectionKey {
	var (
		remote string
		key    = make([]byte, 0, connectionKeySize)
		lPort  = make([]byte, 2)
		rPort  = make([]byte, 2)
		cKey   = ConnectionKey{}
	)

	if c.TCPConn != nil {
		remote = c.TCPConn.RemoteAddr().String()
		key = append(key, byte(c.Proto))
	} else if c.UDPConn != nil || c.GNetConn != nil {
		remote = c.Addr.String()
		key = append(key, byte(c.Proto))
	} else if c.GNetConn != nil {
		remote = c.Addr.String()
		key = append(key, byte(c.Proto))
	} else {
		panic(fmt.Sprintf("wrong connection: %+v", c))
	}

	if c.Port == 0 {
		return cKey
	}

	rAddr, err := netip.ParseAddrPort(remote)
	if err != nil {
		panic(fmt.Sprintf("error parse IP address: %v", err))
	}

	binary.LittleEndian.PutUint16(lPort, c.Port)
	binary.LittleEndian.PutUint16(rPort, rAddr.Port())

	rIP := rAddr.Addr().As4()
	key = append(key, lPort...)
	key = append(key, rIP[:]...)
	key = append(key, rPort...)

	copy(cKey[:], key)

	return cKey
}

// Key generates a unique ConnectionKey for this Connection.
//
// It extracts the local and remote IP and port information from the Connection's
// TCPConn, UDPConn, or GNetConn field based on the connection type.
//
// It concatenates:
//
// - 1 byte for connection protocol (TCP or UDP)
// - 2 bytes for local port of connection
// - 4 bytes for remote IPv4 address
// - 2 bytes for remote port
//
// If the port is 0, it returns an empty ConnectionKey.
//
// The concatenated byte array is then copied into the returned ConnectionKey.
func (c *Connection) Key() ConnectionKey {
	if c.Port == 0 {
		return ConnectionKey{}
	}

	var (
		ip    net.IP
		port  uint16
		key   = make([]byte, 0, connectionKeySize)
		lPort = make([]byte, 2)
		rPort = make([]byte, 2)
		cKey  = ConnectionKey{}
	)

	if c.TCPConn != nil {
		addr := c.TCPConn.RemoteAddr().(*net.TCPAddr)
		ip = addr.IP.To4()
		port = uint16(addr.Port)
		key = append(key, byte(c.Proto))
	} else if c.UDPConn != nil {
		addr := c.Addr.(*net.UDPAddr)
		ip = addr.IP.To4()
		port = uint16(addr.Port)
		key = append(key, byte(c.Proto))
	} else if c.GNetConn != nil {
		switch addr := c.GNetConn.RemoteAddr().(type) {
		case *net.UDPAddr:
			ip = addr.IP.To4()
			port = uint16(addr.Port)
		case *net.TCPAddr:
			ip = addr.IP.To4()
			port = uint16(addr.Port)
		default:
			panic(fmt.Sprintf("wrong address type: %+v", c))
		}
		key = append(key, byte(c.Proto))
	} else {
		panic(fmt.Sprintf("wrong connection: %+v", c))
	}

	binary.LittleEndian.PutUint16(lPort, c.Port)
	binary.LittleEndian.PutUint16(rPort, port)

	key = append(key, lPort...)
	key = append(key, ip...)
	key = append(key, rPort...)

	copy(cKey[:], key)

	return cKey
}

func (c *Connection) Protocol() Protocol {
	if c.TCPConn != nil {
		return ProtoTCP
	}
	return ProtoUDP
}

func (c *Connection) Close() error {
	if c.TCPConn != nil {
		return c.TCPConn.Close()
	} else if c.UDPConn != nil {
		return c.UDPConn.Close()
	}
	return c.GNetConn.Close()
}

type Statistic struct {
	IncomingBytes      uint64
	OutgoingBytes      uint64
	IncomingFrames     uint64
	OutgoingFrames     uint64
	IncomingRateBytes  float64
	OutgoingRateBytes  float64
	IncomingRateFrames float64
	OutgoingRateFrames float64
}
