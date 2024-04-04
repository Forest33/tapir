package merger

import (
	"context"
	"math"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/structs"
)

func (m *V2) stream(ctx context.Context, sessionID uint32, endpoint entity.PacketEndpoint, ch chan *message) {
	defer func() {
		m.streams.Delete(m.getStreamKey(sessionID, endpoint))
		close(ch)
	}()

	var (
		sid           = time.Now().UnixNano()
		lastMessageTs time.Time
	)

	if m.cfg.Tracing {
		m.log.Debug().
			Uint32("session_id", sessionID).
			Uint64("endpoint", endpoint.Uint64()).
			Int("capacity", cap(ch)).
			Int64("sid", sid).
			Msg("stream created")
	}

	sender := func(req *message) {
		if err := m.receiver(req.msg, req.conn); err != nil {
			m.log.Error().Err(err).
				Uint32("session_id", sessionID).
				Uint64("endpoint", req.msg.GetEndpoint().Uint64()).
				Msg("error on streaming")
		}
	}

	// TODO get list from sync.Pool?
	wl := newWaitingList(sender,
		m.pushToWaitingListLog,
		m.popFromWaitingListLog,
		m.senderLog)

	for {
		select {
		case req := <-ch:
			if req == nil {
				if time.Since(lastMessageTs).Seconds() >= m.cfg.StreamTTL {
					m.log.Debug().
						Uint32("session_id", sessionID).
						Uint64("endpoint", endpoint.Uint64()).
						Int64("sid", sid).
						Msg("stream finished due timeout")
					return
				}
				continue
			}

			if m.cfg.Tracing {
				m.log.Debug().
					Uint32("id", req.msg.ID).
					Uint64("endpoint", req.msg.GetEndpoint().Uint64()).
					Uint32("session_id", sessionID).
					Int64("sid", sid).
					Uint32("last_id", wl.getLastID()).
					Int64("ttl", structs.If(wl.getFirstMessageTimestamp() > 0, time.Now().Unix()-wl.getFirstMessageTimestamp(), 0)).
					Int("waiting_list_size", wl.getLength()).
					Msg("received by stream merger")
			}

			if req.msg.SessionID != sessionID || req.msg.GetEndpoint() != endpoint {
				m.log.Error().
					Uint32("req.id", req.msg.ID).
					Uint64("req.endpoint", req.msg.GetEndpoint().Uint64()).
					Uint32("req.session_id", req.msg.SessionID).
					Uint64("endpoint", endpoint.Uint64()).
					Uint32("session_id", sessionID).
					Msg("wrong session id or endpoint received")
				continue
			}

			lastMessageTs = time.Now()

			if wl.getLastID() >= req.msg.ID && wl.getLastID() < math.MaxUint32 || wl.exists(req.msg.ID) {
				if m.cfg.Tracing {
					m.log.Debug().
						Uint32("id", req.msg.ID).
						Uint64("endpoint", req.msg.GetEndpoint().Uint64()).
						Uint32("session_id", sessionID).
						Int64("sid", sid).
						Msg("DUP!")
				}
				continue
			}

			if !wl.pop(req) {
				continue
			}

			if (!wl.isThisFirstMessage() && (wl.getLastID()+1 == req.msg.ID || req.msg.ID == 0)) || wl.isThisFirstMessage() /*|| !req.msg.WithWaitingList()*/ {
				wl.send(req)
				wl.pop(nil)
				continue
			}

			if err := wl.push(req); err != nil {
				m.log.Fatalf("wrong waiting list sort id: %d (%v)", req.msg.ID, structs.Map(wl.getData(), func(m *message) uint32 { return m.msg.ID }))
			}

			size := wl.getDataSize()
			ts := wl.getFirstMessageTimestamp()
			if size > m.cfg.WaitingListMaxSize || (ts != 0 && time.Now().Unix()-ts >= m.cfg.WaitingListMaxTTL) {
				m.log.Error().
					Uint32("id", req.msg.ID).
					Uint64("endpoint", req.msg.GetEndpoint().Uint64()).
					Uint32("session_id", sessionID).
					Int64("sid", sid).
					Int("size", size).
					Int64("ttl", time.Now().Unix()-ts).
					Int("length", wl.getLength()).
					Uint32("head_id", wl.getHead().msg.ID).
					//Interface("waiting_list", structs.Map(wl.getData(), func(m *message) uint32 { return m.msg.ID })).
					Msg("maximum waiting list size or TTL exceeded")
				wl.reset()
			}

		case <-ctx.Done():
			m.log.Debug().
				Uint32("session_id", sessionID).
				Uint64("endpoint", endpoint.Uint64()).
				Int64("sid", sid).
				Msg("stream finished")
			return
		}
	}

}
