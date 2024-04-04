package automaxprocs

import (
	"go.uber.org/automaxprocs/maxprocs"

	"github.com/forest33/tapir/pkg/logger"
)

func Init(log *logger.Logger) {
	_, err := maxprocs.Set(maxprocs.Logger(log.Printf))
	if err != nil {
		log.Error().Err(err).Msg("failed to set automaxprocs")
	}
}
