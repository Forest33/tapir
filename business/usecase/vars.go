package usecase

import (
	"crypto/ecdh"
	"encoding/binary"
	"net"
	"strings"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/codec"
	"github.com/forest33/tapir/pkg/logger"
)

type configHandler interface {
	Save()
	Update(data interface{})
	GetPath() string
	AddObserver(func(interface{})) error
}

var fakeKey = strings.Repeat("0", 32)

func int2ip(nn uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nn)
	return ip
}

func GetECDHCurve() ecdh.Curve {
	return ecdh.X25519()
}

func GetCodec(log *logger.Logger, payloadSize int, encryptorMethod entity.EncryptorMethod, obfuscateData *bool) codec.Codec {
	return codec.NewTapirCodec(log, &codec.Config{
		HeaderSize:    entity.HeaderSize,
		PayloadSize:   payloadSize,
		ObfuscateData: *obfuscateData,
		GetLength:     GetEncryptor(fakeKey, encryptorMethod).GetLength,
	})
}

func GetMaxAcknowledgementSize(c *entity.TunnelConfig) int {
	return c.MTU - GetEncryptor(fakeKey, c.Encryption).GetLength(entity.HeaderSize)
}
