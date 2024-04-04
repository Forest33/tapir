// Package logger wrapper for zerolog
package logger

import (
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

const (
	eventsChannelCapacity = 100
)

// Config logger settings
type Config struct {
	Level             string
	TimeFieldFormat   string
	PrettyPrint       bool
	RedirectStdLogger bool
	DisableSampling   bool
	ErrorStack        bool
	ShowCaller        bool
	EventsChannel     bool
	FileName          string
}

// Logger object capable of interacting with Logger
type Logger struct {
	zero              zerolog.Logger
	zeroErr           zerolog.Logger
	level             string
	prettyPrint       bool
	redirectSTDLogger bool
	rootInitialized   bool
	showCaller        bool
	extWriter         io.Writer
	eventsChannel     chan []byte
}

// NewDefault creates Logger with default settings
func NewDefault() *Logger {
	zerolog.SetGlobalLevel(getZerologLevel(defaultConfig.Level))
	zerolog.DisableSampling(true)
	zerolog.TimeFieldFormat = defaultConfig.TimeFieldFormat
	if defaultConfig.ErrorStack {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	}

	var logger Logger
	logger.level = defaultConfig.Level
	logger.prettyPrint = defaultConfig.PrettyPrint
	logger.showCaller = defaultConfig.ShowCaller
	logger.compileLogger()

	return &logger
}

var defaultConfig = Config{
	Level:           "debug",
	TimeFieldFormat: time.RFC3339,
	PrettyPrint:     true,
	ErrorStack:      false,
	ShowCaller:      false,
}

// New creates a new Logger
func New(config Config) *Logger {
	zerolog.SetGlobalLevel(getZerologLevel(config.Level))
	zerolog.DisableSampling(config.DisableSampling)
	zerolog.TimeFieldFormat = config.TimeFieldFormat
	if config.ErrorStack {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	}

	var logger Logger
	logger.level = config.Level
	logger.prettyPrint = config.PrettyPrint
	logger.redirectSTDLogger = config.RedirectStdLogger
	logger.showCaller = config.ShowCaller

	if config.FileName != "" {
		var err error
		fName := prepareLogFileName(config.FileName)
		logger.extWriter, err = os.Create(fName)
		if err != nil {
			log.Fatalf("failed to create log file: %v", err)
		}
	}
	if config.EventsChannel {
		logger.eventsChannel = make(chan []byte, eventsChannelCapacity)
	}

	logger.compileLogger()

	return &logger
}

// Debug starts a new message with debug level
func (l *Logger) Debug() *zerolog.Event {
	return l.zero.Debug()
}

// Info starts a new message with info level
func (l *Logger) Info() *zerolog.Event {
	return l.zero.Info()
}

// Error starts a new message with error level
func (l *Logger) Error() *zerolog.Event {
	return l.zeroErr.Error()
}

// Warn starts a new message with warn level
func (l *Logger) Warn() *zerolog.Event {
	return l.zeroErr.Warn()
}

// Panic starts a new message with panic level
func (l *Logger) Panic() *zerolog.Event {
	return l.zeroErr.Panic()
}

// With creates a child logger with the field added to its context
func (l *Logger) With() zerolog.Context {
	return l.zero.With()
}

// Fatal sends the event with fatal level
func (l *Logger) Fatal(v ...interface{}) {
	l.zeroErr.Fatal().Msgf("%v", v)
}

// Fatalf sends the event with formatted msg with fatal level
func (l *Logger) Fatalf(format string, v ...interface{}) {
	l.zeroErr.Fatal().Msgf(format, v...)
}

// Print sends the event with debug level
func (l *Logger) Print(v ...interface{}) {
	l.zero.Debug().Msgf("%v", v)
}

// Printf sends the event with formatted msg with debug level
func (l *Logger) Printf(format string, v ...interface{}) {
	l.zero.Debug().Msgf(format, v...)
}

func (l *Logger) initRootLogger() {
	l.rootInitialized = true

	outWriters := []io.Writer{os.Stdout}
	errWriters := []io.Writer{os.Stderr}

	if l.extWriter != nil {
		outWriters = append(outWriters, l.extWriter)
		errWriters = append(errWriters, l.extWriter)
	}
	if l.eventsChannel != nil {
		cw := chanelWriter{ch: l.eventsChannel}
		outWriters = append(outWriters, cw)
		errWriters = append(errWriters, cw)
	}

	l.zero = zerolog.New(zerolog.MultiLevelWriter(outWriters...)).With().Logger()
	l.zeroErr = zerolog.New(zerolog.MultiLevelWriter(errWriters...)).With().Logger()
}

func (l *Logger) compileLogger() {
	if !l.rootInitialized {
		l.initRootLogger()
	}

	if l.redirectSTDLogger {
		l.setLogOutputToZerolog()
	}

	l.initDefaultFields()

	if l.prettyPrint {
		l.addPrettyPrint()
	}
}

func (l *Logger) initDefaultFields() {
	l.zero = l.zero.With().Timestamp().Logger()
	l.zeroErr = l.zeroErr.With().Timestamp().Logger()
	if l.showCaller {
		l.zero = l.zero.With().Caller().Logger()
		l.zeroErr = l.zero.With().Caller().Logger()
	}
}

type chanelWriter struct {
	ch chan []byte
}

func (w chanelWriter) Write(ev []byte) (int, error) {
	buf := make([]byte, len(ev)-1)
	copy(buf, ev)
	w.ch <- buf
	return len(ev), nil
}

func (l *Logger) EventsChannel() chan []byte {
	return l.eventsChannel
}

func (l *Logger) addPrettyPrint() {
	prettyStdout := zerolog.ConsoleWriter{Out: os.Stdout}
	prettyStderr := zerolog.ConsoleWriter{Out: os.Stderr}

	l.zero = l.zero.Output(prettyStdout)
	l.zeroErr = l.zeroErr.Output(prettyStderr)
}

func (l *Logger) setLogOutputToZerolog() {
	log.SetFlags(0)
	log.SetOutput(l.zero)
}

func (l *Logger) Duplicate(zero zerolog.Logger) *Logger {
	dup := &Logger{
		level:             l.level,
		prettyPrint:       l.prettyPrint,
		redirectSTDLogger: l.redirectSTDLogger,
		rootInitialized:   l.rootInitialized,
		showCaller:        l.showCaller,
		eventsChannel:     l.eventsChannel,
	}

	outWriters := []io.Writer{os.Stdout}
	errWriters := []io.Writer{os.Stderr}

	if l.extWriter != nil {
		outWriters = append(outWriters, l.extWriter)
		errWriters = append(errWriters, l.extWriter)
	}
	if l.eventsChannel != nil {
		cw := chanelWriter{ch: l.eventsChannel}
		outWriters = append(outWriters, cw)
		errWriters = append(errWriters, cw)
	}

	dup.zero = zero.Output(zerolog.MultiLevelWriter(outWriters...)).With().Logger()
	dup.zeroErr = zero.Output(zerolog.MultiLevelWriter(errWriters...)).With().Logger()

	if l.prettyPrint {
		dup.addPrettyPrint()
	}

	return dup
}

func getZerologLevel(lvl string) zerolog.Level {
	switch strings.ToLower(lvl) {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	case "panic":
		return zerolog.PanicLevel
	case "disabled":
		return zerolog.Disabled
	}
	return zerolog.NoLevel
}

func prepareLogFileName(pattern string) string {
	cur := time.Now()
	pattern = strings.ReplaceAll(pattern, "%d", cur.Format("2"))
	pattern = strings.ReplaceAll(pattern, "%D", cur.Format("02"))
	pattern = strings.ReplaceAll(pattern, "%m", cur.Format("1"))
	pattern = strings.ReplaceAll(pattern, "%M", cur.Format("01"))
	pattern = strings.ReplaceAll(pattern, "%y", cur.Format("06"))
	pattern = strings.ReplaceAll(pattern, "%Y", cur.Format("2006"))
	pattern = strings.ReplaceAll(pattern, "%H", cur.Format("15"))
	pattern = strings.ReplaceAll(pattern, "%N", cur.Format("04"))
	pattern = strings.ReplaceAll(pattern, "%S", cur.Format("05"))
	return pattern
}
