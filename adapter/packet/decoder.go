package packet

import (
	"encoding/binary"

	"github.com/forest33/tapir/business/entity"
)

const (
	IpPacketMinLength = 20
)

const (
	fnvBasis = 14695981039346656037
	fnvPrime = 1099511628211
)

const (
	endpointTypeIPv4 = iota + 1
	endpointTypeIPv6
)

const (
	EndpointHashSourceAddress EndpointHashType = iota + 1
	EndpointHashDestinationAddress
	EndpointHashFullAddress
)

type EndpointHashType uint8

type Decoder struct {
	cfg *Config
}

type Config struct {
	EndpointHashType EndpointHashType
	Tracing          bool
}

func New(cfg *Config) *Decoder {
	return &Decoder{cfg: cfg}
}

func (d *Decoder) Decode(data []byte) (*entity.NetworkPacketInfo, error) {

	if len(data) < IpPacketMinLength {
		return nil, entity.ErrWrongPacketLength
	}

	pi, err := d.decodeIP(data)
	if !d.cfg.Tracing {
		return pi, err
	}

	return decodeLayers(data, pi)
}

func (d *Decoder) decodeIP(data []byte) (*entity.NetworkPacketInfo, error) {
	switch data[0] >> 4 {
	case 4:
		return d.decodeIPv4(data)
	case 6:
		return d.decodeIPv6(data)
	default:
		return nil, entity.ErrWrongPacketData
	}
}

func (d *Decoder) decodeIPv4(data []byte) (*entity.NetworkPacketInfo, error) {
	ihl := data[0] & 0x0F
	if ihl < 5 || ihl > 15 {
		return nil, entity.ErrWrongPacketData
	}

	length := binary.BigEndian.Uint16(data[2:4])
	if int(ihl*4) > int(length) {
		return nil, entity.ErrWrongPacketData
	}

	pi := &entity.NetworkPacketInfo{}

	var endpoint []byte
	switch d.cfg.EndpointHashType {
	case EndpointHashSourceAddress:
		endpoint = data[12:16]
	case EndpointHashDestinationAddress:
		endpoint = data[16:20]
	case EndpointHashFullAddress:
		endpoint = append(data[12:16], data[16:20]...)
	default:
		panic("unknown endpoint hash type")
	}

	pi.Endpoint = entity.PacketEndpoint(fastHash(endpoint, endpointTypeIPv4))
	pi.Protocol = entity.IPProtocol(data[9])

	return pi, nil
}

func (d *Decoder) decodeIPv6(data []byte) (*entity.NetworkPacketInfo, error) {
	if len(data) < 40 {
		return nil, entity.ErrWrongPacketLength
	}

	var endpoint []byte
	switch d.cfg.EndpointHashType {
	case EndpointHashSourceAddress:
		endpoint = data[8:24]
	case EndpointHashDestinationAddress:
		endpoint = data[24:40]
	case EndpointHashFullAddress:
		endpoint = append(data[8:24], data[24:40]...)
	default:
		panic("unknown endpoint hash type")
	}

	//pi := entity.PacketInfoPool.Get()
	pi := &entity.NetworkPacketInfo{}
	pi.Endpoint = entity.PacketEndpoint(fastHash(endpoint, endpointTypeIPv6))
	pi.Protocol = entity.IPProtocol(data[6])

	return pi, nil
}

// fnvHash is used by our fastHash functions, and implements the FNV hash
// created by Glenn Fowler, Landon Curt Noll, and Phong Vo.
// See http://isthe.com/chongo/tech/comp/fnv/.
func fnvHash(s []byte) (h uint64) {
	h = fnvBasis
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= fnvPrime
	}
	return
}

// FastHash provides a quick hashing function for an endpoint, useful if you'd
// like to split up endpoints by modulos or other load-balancing techniques.
// It uses a variant of Fowler-Noll-Vo hashing.
//
// The output of FastHash is not guaranteed to remain the same through future
// code revisions, so should not be used to key values in persistent storage.
func fastHash(s []byte, typ uint64) (h uint64) {
	h = fnvHash(s)
	h ^= typ
	h *= fnvPrime
	return
}
