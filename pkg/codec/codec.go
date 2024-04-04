package codec

import (
	"github.com/forest33/tapir/business/entity"
)

type Decoder interface {
	UnmarshalHeader(data []byte, m *entity.Message) error
	UnmarshalPayload(m *entity.Message) error
}

type Encoder interface {
	Marshal(m *entity.Message) ([]byte, []byte, error)
}

type Codec interface {
	Decoder
	Encoder
}

type GetLengthFunc func(int) int

type Config struct {
	HeaderSize    int
	PayloadSize   int
	ObfuscateData bool
	GetLength     GetLengthFunc
}
