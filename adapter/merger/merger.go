package merger

import (
	"context"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

type message struct {
	msg  *entity.Message
	conn *entity.Connection
}

type Config struct {
	ThreadingBy         string
	WaitingListMaxSize  int
	WaitingListMaxTTL   int64
	StreamCheckInterval int
	StreamTTL           float64
	StreamCount         int
	Tracing             bool
}

const initialMessageCount = 100

func (c Config) validate() error {
	// TODO
	return nil
}

func New(ctx context.Context, log *logger.Logger, cfg *Config) (entity.StreamMerger, error) {
	log.Info().Str("threadingBy", cfg.ThreadingBy).Msg("creating stream merger")

	switch cfg.ThreadingBy {
	case entity.MergerThreadingBySession:
		return NewV1(ctx, log, cfg)
	case entity.MergerThreadingByEndpoint:
		return NewV2(ctx, log, cfg)
	default:
		log.Fatalf("unknown stream merger: %s", cfg.ThreadingBy)
	}

	return nil, nil
}
