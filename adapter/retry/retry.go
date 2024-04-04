package retry

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

const (
	initRTO = time.Second // net.ipv4.neigh.wlp0s20f3.retrans_time_ms = 1000
	//initSrttFactor   = 1.0 / 64.0
	//initRttvarFactor = 1.0 / 4.0
	//rtoFactor        = 5.0
	initSrttFactor   = 1.0 / 8.0
	initRttvarFactor = 1.0 / 4.0
	rtoFactor        = 4.0
	rttFactor        = int64(time.Millisecond * 100)
	fSecond          = float64(time.Second)
)

type Retry struct {
	ctx           context.Context
	cancel        context.CancelFunc
	log           *logger.Logger
	cfg           *Config
	sendData      entity.RetrySender
	sendKeepalive entity.KeepaliveSender
	disconnect    entity.DisconnectHandler
	conn          *entity.Connection
	messages      *sync.Map
	srtt          float64
	rttvar        float64
	rto           time.Duration
	rtt           float64
	keepaliveCh   chan *struct{}
}

type message struct {
	start    int64
	attempts float64
	timer    *time.Timer
	stop     chan struct{}
	msg      *entity.Message
}

type Config struct {
	MaxTimeout        time.Duration
	BackoffFactor     float64
	KeepaliveTimeout  time.Duration
	KeepaliveInterval time.Duration
	KeepaliveProbes   int
	Tracing           bool
}

type messageKey [2]uint64

func New(ctx context.Context, log *logger.Logger, cfg *Config, dataSender entity.RetrySender, keepaliveSender entity.KeepaliveSender, disconnect entity.DisconnectHandler, conn *entity.Connection) *Retry {
	r := &Retry{
		log:           log.Duplicate(log.With().Str("layer", "retry").Logger()),
		cfg:           cfg,
		sendData:      dataSender,
		sendKeepalive: keepaliveSender,
		disconnect:    disconnect,
		conn:          conn,
		keepaliveCh:   make(chan *struct{}, cfg.KeepaliveProbes),
		messages:      &sync.Map{},
		rto:           initRTO,
	}

	r.ctx, r.cancel = context.WithCancel(ctx)

	r.keepalive()

	return r
}

func (r *Retry) Push(m *entity.Message) {
	msg := &message{
		start: time.Now().UnixNano(),
		timer: time.NewTimer(r.rto),
		stop:  make(chan struct{}, 1),
		msg:   m,
	}
	r.messages.Store(r.getMessageKey(m.GetEndpoint(), m.ID), msg)

	go r.timer(msg, r.rto)
}

func (r *Retry) Ack(ids *entity.MessageAcknowledgement) {
	if ids != nil {
		// TODO getting time.Now().UnixNano() here?
		for endpoint, ids := range ids.Get() {
			for _, id := range ids {
				if m, ok := r.messages.LoadAndDelete(r.getMessageKey(endpoint, id)); ok {
					msg := m.(*message)
					rtt := float64((time.Now().UnixNano() + rttFactor) - msg.start)
					stop := msg.timer.Stop()
					if stop && msg.attempts == 0 {
						r.updateRTO(rtt)
					}

					msg.stop <- struct{}{}
					close(msg.stop)

					if r.cfg.Tracing {
						r.log.Debug().
							Uint32("id", id).
							Uint64("endpoint", endpoint.Uint64()).
							Bool("stop", stop).
							Float64("attempts", msg.attempts).
							Float64("rtt", rtt).
							Float64("srtt", r.srtt).
							Float64("rttvar", r.rttvar).
							Float64("rto", r.rto.Seconds()).
							Msg("timer stopped")
					}
				}
			}
		}
	}
	r.keepaliveCh <- &struct{}{}
}

func (r *Retry) Keepalive() {
	r.keepaliveCh <- nil
}

func (r *Retry) GetRTO() time.Duration {
	return r.rto
}

func (r *Retry) Stop() {
	r.cancel()
}

func (r *Retry) timer(m *message, duration time.Duration) {
	defer func() {
		entity.MessagePool.Put(m.msg)
		r.messages.Delete(r.getMessageKey(m.msg.GetEndpoint(), m.msg.ID))
	}()

	if r.cfg.Tracing {
		r.log.Debug().
			Uint32("id", m.msg.ID).
			Uint64("endpoint", m.msg.GetEndpoint().Uint64()).
			Uint32("session_id", m.msg.SessionID).
			Float64("duration", duration.Seconds()).
			Msg("waiting timer started")
	}

	for {
		select {
		case ts := <-m.timer.C:
			if ts.UnixNano()-m.start >= r.cfg.MaxTimeout.Nanoseconds() {
				if r.cfg.Tracing {
					r.log.Debug().Uint32("id", m.msg.ID).Uint32("session_id", m.msg.SessionID).Msg("maximum retries time exceeded")
				}
				return
			}

			if err := r.sendData(m.msg.Payload.([]byte), m.msg, r.conn); err != nil {
				r.log.Error().Err(err).
					Uint32("id", m.msg.ID).
					Uint64("endpoint", m.msg.GetEndpoint().Uint64()).
					Uint32("session_id", m.msg.SessionID).
					Msg("failed to send retry")
			}

			m.attempts++
			duration += time.Duration(fSecond * math.Exp(m.attempts*r.cfg.BackoffFactor))
			m.timer.Reset(duration)

			if r.cfg.Tracing {
				r.log.Debug().
					Uint32("id", m.msg.ID).
					Uint64("endpoint", m.msg.GetEndpoint().Uint64()).
					Uint32("session_id", m.msg.SessionID).
					Str("duration", fmt.Sprintf("%.3f", duration.Seconds())).
					Float64("attempts", m.attempts).
					Msg("waiting time increased")
			}

			continue
		case <-m.stop:
			return
		case <-r.ctx.Done():
			return
		}
	}
}

func (r *Retry) updateRTO(rtt float64) {
	if r.srtt == 0 {
		r.srtt = rtt
		r.rttvar = rtt / 2
	} else {
		rttrc := math.Abs((rtt - r.rtt) / r.rtt)
		if rttrc > 1 {
			rttrc = 1
		}
		srttFactor := initSrttFactor * (1 + rttrc)
		rttvarFactor := initRttvarFactor * (1 - rttrc)

		r.srtt = (1-srttFactor)*r.srtt + srttFactor*rtt
		r.rttvar = (1-rttvarFactor)*r.rttvar + rttvarFactor*math.Abs(r.srtt-rtt)
	}

	r.rto = time.Duration(r.srtt + rtoFactor*r.rttvar)
	r.rtt = rtt
}

func (r *Retry) keepalive() {
	go func() {
		var (
			probes  int
			lastAck time.Time
			started = time.Now()
		)

		for {
			select {
			case m := <-r.keepaliveCh:
				lastAck = time.Now()
				probes = 0
				if m == nil {
					r.sendKeepalive(r.conn, true)
				}
			case <-r.ctx.Done():
				return
			case <-time.After(r.cfg.KeepaliveInterval):
				if time.Since(lastAck) < r.cfg.KeepaliveTimeout || time.Since(started) < r.cfg.KeepaliveTimeout {
					continue
				}
				if probes >= r.cfg.KeepaliveProbes {
					r.Stop()
					r.disconnect(r.conn, entity.ErrKeepaliveTimeoutExceeded)
					return
				}
				r.sendKeepalive(r.conn, false)
				probes++
			}
		}
	}()
}

func (r *Retry) getMessageKey(endpoint entity.PacketEndpoint, messageID uint32) messageKey {
	return [2]uint64{
		endpoint.Uint64(),
		uint64(messageID),
	}
}
