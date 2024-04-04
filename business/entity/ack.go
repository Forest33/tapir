package entity

import "math"

const (
	acknowledgementEndpointSize = 9
	initialAckDataSize          = 10
)

type MessageAcknowledgement struct {
	data    MessageAcknowledgementData
	maxSize int
	size    int
}

type MessageAcknowledgementData map[PacketEndpoint][]uint32

func NewMessageAcknowledgement(data MessageAcknowledgementData) *MessageAcknowledgement {
	ma := &MessageAcknowledgement{}
	if data == nil {
		ma.data = make(map[PacketEndpoint][]uint32, initialAckDataSize)
	} else {
		ma.data = data
	}
	return ma
}

func (ma *MessageAcknowledgement) SetMaxSize(size int) *MessageAcknowledgement {
	ma.maxSize = size
	return ma
}

func (ma *MessageAcknowledgement) Size() int {
	return ma.size
}

func (ma *MessageAcknowledgement) GetMessagesCount() int {
	var count int
	for _, ids := range ma.data {
		count += len(ids)
	}
	return count
}

func (ma *MessageAcknowledgement) Get() MessageAcknowledgementData {
	return ma.data
}

func (ma *MessageAcknowledgement) Push(endpoint PacketEndpoint, messageID uint32) bool {
	ids, ok := ma.data[endpoint]
	if !ok {
		if ma.size+acknowledgementEndpointSize+4 > ma.maxSize {
			return false
		}
		ids = make([]uint32, 0, initialAckDataSize)
		ma.size += acknowledgementEndpointSize
	} else if ma.size+4 > ma.maxSize || len(ids)+1 > math.MaxUint8 {
		return false
	}
	ids = append(ids, messageID)
	ma.data[endpoint] = ids
	ma.size += 4
	return true
}

func (ma *MessageAcknowledgement) Reset() {
	clear(ma.data)
	ma.size = 0
}
