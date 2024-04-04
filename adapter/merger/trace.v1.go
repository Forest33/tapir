package merger

import (
	"github.com/forest33/tapir/business/entity"
)

func (m *V1) pushToWaitingListLog(msg *entity.Message, waitingList *waitingList) {
	if !m.cfg.Tracing {
		return
	}

	var headID uint32
	if waitingList.getLength() > 0 {
		headID = waitingList.getHead().msg.ID
	}

	m.log.Debug().
		Uint32("id", msg.ID).
		Uint64("endpoint", msg.GetEndpoint().Uint64()).
		Int("size", waitingList.getLength()).
		Uint32("session_id", msg.SessionID).
		//Uints32("waiting_list", structs.Map(waitingList, func(m *message) uint32 { return m.msg.ID })).
		Uint32("waiting_list_head", headID).
		Msg("push to waiting list")
}

func (m *V1) popFromWaitingListLog(msg *entity.Message, waitingList *waitingList) {
	if !m.cfg.Tracing {
		return
	}

	m.log.Debug().
		Uint32("id", msg.ID).
		Uint64("endpoint", msg.GetEndpoint().Uint64()).
		Int("size", waitingList.getLength()).
		Uint32("session_id", msg.SessionID).
		Msg("pop from waiting list")
}

func (m *V1) senderLog(msg *entity.Message, waitingList *waitingList) {
	if !m.cfg.Tracing {
		return
	}

	var headID uint32
	if waitingList.getLength() > 0 {
		headID = waitingList.getHead().msg.ID
	}

	m.log.Debug().
		Uint32("id", msg.ID).
		Uint64("endpoint", msg.GetEndpoint().Uint64()).
		Uint32("last_id", waitingList.getLastID()).
		Uint32("session_id", msg.SessionID).
		Uint8("compression", uint8(msg.CompressionType)).
		//Uints32("waiting_list", structs.Map(waitingList, func(m *message) uint32 { return m.msg.ID })).
		//Str("hash", util.GetMD5Hash(msg.Payload.([]byte))).
		Uint32("waiting_list_head", headID).
		Int("waiting_list_size", waitingList.getLength()).
		Msg("sent merged message")
}
