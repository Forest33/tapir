package profiler

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"sync"

	"github.com/forest33/tapir/pkg/logger"
)

type Config struct {
	Host string
	Port int
}

var (
	once = sync.Once{}
)

func Start(cfg *Config, log *logger.Logger) {
	once.Do(func() {
		log.Info().
			Str("host", cfg.Host).
			Int("port", cfg.Port).
			Msg("starting profiler")
		go func() {
			err := http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), nil)
			if err != nil {
				log.Fatalf("failed to start profiler: %v", err)
			}
		}()
	})
}
