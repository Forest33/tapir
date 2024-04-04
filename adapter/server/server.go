package server

import (
	"sync"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/codec"
)

var (
	once     = sync.Once{}
	instance entity.NetworkServer
)

type Config struct {
	Codec            codec.Codec
	PrimaryEncryptor entity.Encryptor
	ReadBufferSize   int
	WriteBufferSize  int
	MultipathTCP     bool
	MTU              int
	MaxSessionsCount int
	Tracing          bool
}

func (c Config) validate() error {
	// TODO
	return nil
}

type connControl struct {
	retry entity.NetworkRetry
	ack   entity.NetworkAck
}
