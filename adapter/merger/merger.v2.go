package merger

import (
	"context"
	"sync"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

type V2 struct {
	ctx         context.Context
	log         *logger.Logger
	cfg         *Config
	receiver    entity.ReceiverHandler
	disconnect  entity.DisconnectHandler
	reset       entity.ResetHandler
	streams     *sync.Map
	sessions    map[uint32]*session
	sessionsMux sync.RWMutex
}

type session struct {
	ctx    context.Context
	cancel context.CancelFunc
}

type streamKey [2]uint64

func NewV2(ctx context.Context, log *logger.Logger, cfg *Config) (*V2, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	m := &V2{
		ctx:      ctx,
		log:      log.Duplicate(log.With().Str("layer", "merger").Logger()),
		cfg:      cfg,
		streams:  &sync.Map{},
		sessions: make(map[uint32]*session, cfg.StreamCount),
	}

	m.streamChecker()

	return m, nil
}

func (m *V2) CreateStream(sessionID uint32) error {
	if m.receiver == nil {
		return entity.ErrReceiverHandlerNotSet
	} else if m.disconnect == nil {
		return entity.ErrDisconnectHandlerNotSet
	}

	m.sessionsMux.Lock()
	defer m.sessionsMux.Unlock()
	if _, ok := m.sessions[sessionID]; !ok {
		ctx, cancel := context.WithCancel(m.ctx)
		m.sessions[sessionID] = &session{
			ctx:    ctx,
			cancel: cancel,
		}
	}

	return nil
}

func (m *V2) DeleteStream(sessionID uint32) {
	m.sessionsMux.Lock()
	defer m.sessionsMux.Unlock()

	if s, ok := m.sessions[sessionID]; ok {
		s.cancel()
		delete(m.sessions, sessionID)
	}
}

func (m *V2) Push(msg *entity.Message, conn *entity.Connection) error {
	if !msg.IsStreamMerge() {
		return m.receiver(msg, conn)
	}

	send := func(ch chan *message) {
		ch <- &message{
			msg:  msg,
			conn: conn,
		}
	}

	log := func() {
		if m.cfg.Tracing {
			m.log.Debug().
				Uint32("id", msg.ID).
				Uint64("endpoint", msg.GetEndpoint().Uint64()).
				Msg("push to stream merger")
		}
	}

	m.sessionsMux.RLock()
	sess, ok := m.sessions[msg.SessionID]
	if ok {
		key := m.getStreamKey(msg.SessionID, msg.GetEndpoint())
		ch, ok := m.streams.LoadOrStore(key, make(chan *message, initialMessageCount)) // TODO get channel from sync.Pool?
		if !ok {
			go m.stream(sess.ctx, msg.SessionID, msg.GetEndpoint(), ch.(chan *message))
		}
		send(ch.(chan *message))
		log()
	} else {
		m.log.Error().Uint32("id", msg.ID).Uint32("session_id", msg.SessionID).Msg("session not found")
		m.reset(msg.SessionID, conn)
		m.sessionsMux.RUnlock()
		return entity.ErrSessionNotExists
	}
	m.sessionsMux.RUnlock()

	return nil
}

func (m *V2) getStreamKey(sessionID uint32, endpoint entity.PacketEndpoint) streamKey {
	return [2]uint64{
		uint64(sessionID),
		endpoint.Uint64(),
	}
}

func (m *V2) SetReceiverHandler(h entity.ReceiverHandler) {
	m.receiver = h
}

func (m *V2) SetDisconnectHandler(h entity.DisconnectHandler) {
	m.disconnect = h
}

func (m *V2) SetResetHandler(h entity.ResetHandler) {
	m.reset = h
}

func (m *V2) streamChecker() {
	if m.cfg.StreamCheckInterval == 0 || m.cfg.StreamTTL == 0 {
		return
	}

	go func() {
		ticker := time.NewTicker(time.Duration(m.cfg.StreamCheckInterval) * time.Second)
		for {
			select {
			case <-m.ctx.Done():
				return
			case <-ticker.C:
				m.streams.Range(func(_ any, ch any) bool {
					ch.(chan *message) <- nil
					return true
				})
			}
		}
	}()
}
