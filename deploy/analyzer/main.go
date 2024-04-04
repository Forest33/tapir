package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/config"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
)

var (
	cfg      = &entity.ServerConfig{}
	zlog     *logger.Logger
	logsPath *string
	logFile  *string
	outFile  *string
	testType *string
)

const (
	defaultConfigFile  = "../../config/tapir-server.yaml"
	defaultLogsPath    = "../../logs"
	initialEventsCount = 100000
	chartWidth         = "2500px"
	chartHeight        = "1000px"
)

func init() {
	configPath := flag.String("config", defaultConfigFile, "path to tapir configuration file")
	logsPath = flag.String("logs-path", defaultLogsPath, "path to logs directory")
	logFile = flag.String("log-file", "", "analyzed log file")
	outFile = flag.String("out-file", "", "result file")
	testType = flag.String("test", "retry", "type of test (retry, merger, ack)")
	flag.Parse()

	if _, err := config.New(*configPath, "", cfg); err != nil {
		log.Fatalf("failed to parse config file: %v", err)
	}

	zlog = logger.New(logger.Config{
		Level:             cfg.Logger.Level,
		TimeFieldFormat:   cfg.Logger.TimeFieldFormat,
		PrettyPrint:       cfg.Logger.PrettyPrint,
		DisableSampling:   cfg.Logger.DisableSampling,
		RedirectStdLogger: cfg.Logger.RedirectStdLogger,
		ErrorStack:        cfg.Logger.ErrorStack,
		ShowCaller:        cfg.Logger.ShowCaller,
	})

	if cfg.Runtime.GoMaxProcs == 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		runtime.GOMAXPROCS(cfg.Runtime.GoMaxProcs)
	}
}

func main() {
	if *logFile == "" {
		last, err := getLastLogFile(*logsPath)
		if err != nil {
			zlog.Fatalf("failed to get last log file: %v", err)
		}
		logFile = &last
	}

	if *outFile == "" {
		dir, file := path.Split(*logFile)
		outFile = structs.Ref(fmt.Sprintf("%s%s-%s.html", dir, strings.TrimSuffix(file, filepath.Ext(file)), *testType))
	}

	var err error
	switch *testType {
	case "retry":
		err = chartRetry(*logFile, *outFile)
	case "merger":
		err = chartMerger(*logFile, *outFile)
	case "ack":
		err = chartAcknowledgement(*logFile, *outFile)
	}
	if err != nil {
		zlog.Fatalf("failed to generate chart: %v", err)
	}
}

func strToDateTime(input string) (time.Time, error) {
	t, err := time.Parse("2006-01-02T15:04:05.000000", input)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func getLastLogFile(dir string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	var oldestTime time.Time
	var last string

	for _, file := range files {
		fi, err := file.Info()
		if err != nil {
			return "", err
		}
		if filepath.Ext(file.Name()) != ".log" {
			continue
		}
		if !fi.IsDir() && fi.ModTime().After(oldestTime) {
			last = file.Name()
			oldestTime = fi.ModTime()
		}
	}

	if last == "" {
		return "", os.ErrNotExist
	}

	return path.Join(dir, last), nil
}
