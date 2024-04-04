package merger

import (
	"math"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/structs"
)

func (m *V1) stream(sessionID uint32, ch chan *message) {
	if m.cfg.Tracing {
		m.log.Debug().Uint32("session_id", sessionID).Int("capacity", cap(ch)).Msg("stream created")
	}

	wlMap := make(map[entity.PacketEndpoint]*waitingList, initialMessageCount)

	sender := func(req *message) {
		if err := m.receiver(req.msg, req.conn); err != nil {
			m.log.Error().Err(err).
				Uint32("session_id", sessionID).
				Msg("error on streaming")
		}
	}

	getWaitingList := func(endpoint entity.PacketEndpoint) *waitingList {
		if wl, ok := wlMap[endpoint]; ok {
			return wl
		}
		wlMap[endpoint] = newWaitingList(sender,
			m.pushToWaitingListLog,
			m.popFromWaitingListLog,
			m.senderLog)
		return wlMap[endpoint]
	}

	for req := range ch {
		if req == nil {
			m.log.Debug().Uint32("session_id", sessionID).Msg("stream finished")
			return
		}

		wl := getWaitingList(req.msg.GetEndpoint())

		if m.cfg.Tracing {
			m.log.Debug().
				Uint32("id", req.msg.ID).
				Uint64("endpoint", req.msg.GetEndpoint().Uint64()).
				Uint32("last_id", wl.getLastID()).
				Int("waiting_list_size", wl.getLength()).
				Int64("ttl", structs.If(wl.getFirstMessageTimestamp() > 0, time.Now().Unix()-wl.getFirstMessageTimestamp(), 0)).
				Msg("received by stream merger")
		}

		if wl.getLastID() >= req.msg.ID && wl.getLastID() < math.MaxUint32 || wl.exists(req.msg.ID) {
			if m.cfg.Tracing {
				m.log.Debug().
					Uint32("id", req.msg.ID).
					Uint64("endpoint", req.msg.GetEndpoint().Uint64()).
					Uint32("session_id", sessionID).
					Msg("DUP!")
			}
			continue
		}

		if !wl.pop(req) {
			continue
		}

		if (!wl.isThisFirstMessage() && (wl.getLastID()+1 == req.msg.ID || req.msg.ID == 0)) || (wl.isThisFirstMessage() && req.msg.ID == 1) {
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
				Int("size", size).
				Int64("ttl", time.Now().Unix()-ts).
				Int("length", wl.getLength()).
				Uint32("head_id", wl.getHead().msg.ID).
				Msg("maximum waiting list size or TTL exceeded")
			wl.reset()
		}
	}
}
