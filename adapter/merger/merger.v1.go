package merger

import (
	"context"
	"sync"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

type V1 struct {
	ctx        context.Context
	log        *logger.Logger
	cfg        *Config
	receiver   entity.ReceiverHandler
	disconnect entity.DisconnectHandler
	reset      entity.ResetHandler
	streams    map[uint32]stream
	streamsMux sync.RWMutex
}

type stream struct {
	ch chan *message
}

func NewV1(ctx context.Context, log *logger.Logger, cfg *Config) (*V1, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &V1{
		ctx:     ctx,
		log:     log.Duplicate(log.With().Str("layer", "merger").Logger()),
		cfg:     cfg,
		streams: make(map[uint32]stream, cfg.StreamCount),
	}, nil
}

func (m *V1) CreateStream(sessionID uint32) error {
	if m.receiver == nil {
		return entity.ErrReceiverHandlerNotSet
	} else if m.disconnect == nil {
		return entity.ErrDisconnectHandlerNotSet
	}

	m.streamsMux.Lock()
	defer m.streamsMux.Unlock()

	if _, ok := m.streams[sessionID]; ok {
		return nil
	}
	m.streams[sessionID] = stream{
		ch: make(chan *message, initialMessageCount),
	}
	go m.stream(sessionID, m.streams[sessionID].ch)

	m.log.Info().Uint32("session_id", sessionID).Msg("stream created")

	return nil
}

func (m *V1) DeleteStream(sessionID uint32) {
	m.streamsMux.Lock()
	defer m.streamsMux.Unlock()

	if st, ok := m.streams[sessionID]; ok {
		st.ch <- nil
		close(st.ch)
		delete(m.streams, sessionID)
	}
}

func (m *V1) Push(msg *entity.Message, conn *entity.Connection) error {
	if !msg.IsStreamMerge() {
		return m.receiver(msg, conn)
	}

	m.streamsMux.RLock()
	if s, ok := m.streams[msg.SessionID]; ok {
		if m.cfg.Tracing {
			m.log.Debug().
				Uint32("id", msg.ID).
				Uint64("endpoint", msg.GetEndpoint().Uint64()).
				Msg("push to stream merger")
		}
		s.ch <- &message{
			msg:  msg,
			conn: conn,
		}
	} else {
		m.log.Error().Uint32("id", msg.ID).Uint32("session_id", msg.SessionID).Msg("stream not found")
		m.reset(msg.SessionID, conn)
	}
	m.streamsMux.RUnlock()

	return nil
}

func (m *V1) SetReceiverHandler(h entity.ReceiverHandler) {
	m.receiver = h
}

func (m *V1) SetDisconnectHandler(h entity.DisconnectHandler) {
	m.disconnect = h
}

func (m *V1) SetResetHandler(h entity.ResetHandler) {
	m.reset = h
}
