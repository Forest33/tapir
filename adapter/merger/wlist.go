package merger

import (
	"errors"
	"math"
	"slices"
	"time"

	"github.com/forest33/tapir/business/entity"
)

type waitingList struct {
	lastID         uint32
	firstSent      bool
	data           []*message
	dataSize       int
	firstMessageTs int64
	idsMap         map[uint32]struct{}
	sender         waitingListSender
	popLogger      waitingListLogger
	pushLogger     waitingListLogger
	sendLogger     waitingListLogger
}

type waitingListSender func(*message)
type waitingListLogger func(*entity.Message, *waitingList)

func newWaitingList(sender waitingListSender, pushLogger waitingListLogger, popLogger waitingListLogger, sendLogger waitingListLogger) *waitingList {
	return &waitingList{
		data:       make([]*message, 0, initialMessageCount),
		idsMap:     make(map[uint32]struct{}, initialMessageCount),
		sender:     sender,
		pushLogger: pushLogger,
		popLogger:  popLogger,
		sendLogger: sendLogger,
	}
}

func (wl *waitingList) getLength() int {
	return len(wl.data)
}

func (wl *waitingList) getHead() *message {
	if wl.getLength() == 0 {
		return nil
	}
	return wl.data[0]
}

func (wl *waitingList) getData() []*message {
	return wl.data
}

func (wl *waitingList) getDataSize() int {
	return wl.dataSize
}

func (wl *waitingList) getLastID() uint32 {
	return wl.lastID
}

func (wl *waitingList) getFirstMessageTimestamp() int64 {
	return wl.firstMessageTs
}

func (wl *waitingList) isThisFirstMessage() bool {
	return !wl.firstSent
}

func (wl *waitingList) exists(id uint32) bool {
	_, exists := wl.idsMap[id]
	return exists
}

func (wl *waitingList) send(req *message) {
	wl.sendLogger(req.msg, wl)
	wl.sender(req)
	wl.lastID = req.msg.ID
	wl.firstSent = true
}

func (wl *waitingList) push(req *message) error {
	length := wl.getLength()
	wl.dataSize += int(req.msg.PayloadLength)
	if length > 0 {
		if wl.data[length-1].msg.ID < req.msg.ID {
			wl.pushLogger(req.msg, wl)
			wl.data = append(wl.data, req)
			wl.idsMap[req.msg.ID] = struct{}{}
			return nil
		}
		if wl.data[0].msg.ID > req.msg.ID {
			wl.pushLogger(req.msg, wl)
			wl.data = slices.Insert(wl.data, 0, req)
			wl.idsMap[req.msg.ID] = struct{}{}
			return nil
		}

		var lastMinID uint32 = math.MaxUint32
		for i := 0; i < length; i++ {
			minID := req.msg.ID - wl.data[i].msg.ID
			if minID > lastMinID {
				wl.pushLogger(req.msg, wl)
				wl.data = slices.Insert(wl.data, i, req)
				wl.idsMap[req.msg.ID] = struct{}{}
				return nil
			}
			lastMinID = minID
		}
		return errors.New("wrong waiting list sort")
	}

	wl.pushLogger(req.msg, wl)
	wl.data = append(wl.data, req)
	wl.idsMap[req.msg.ID] = struct{}{}
	wl.firstMessageTs = time.Now().Unix()

	return nil
}

func (wl *waitingList) pop(req *message) bool {
	var (
		length = wl.getLength()
		head   = wl.getHead()
		i      int
	)

	if length > 0 && (head.msg.ID == wl.lastID+1 || head.msg.ID == 0) {
		if req != nil {
			wl.send(req)
		}
		for i = 0; i < length; i++ {
			wl.popLogger(wl.data[i].msg, wl)
			wl.send(wl.data[i])
			delete(wl.idsMap, wl.data[i].msg.ID)
			wl.dataSize -= int(wl.data[i].msg.PayloadLength)
			if i+1 < length && wl.data[i+1].msg.ID != wl.lastID+1 {
				break
			}
		}
		if i == length {
			wl.data = wl.data[:0]
			wl.firstMessageTs = 0
		} else {
			wl.data = append(wl.data[:0], wl.data[i+1:]...)
		}
		return false
	}

	return true
}

func (wl *waitingList) reset() {
	for _, m := range wl.data {
		wl.send(m)
	}
	wl.data = wl.data[:0]
	clear(wl.idsMap)
	wl.dataSize = 0
}
