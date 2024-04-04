package entity

import (
	"net"
)

const (
	MessageTypeAuthentication = iota + 1
	MessageTypeHandshake
	MessageTypeData
	MessageTypeKeepalive
	MessageTypeReset
)

var (
	MessageTypePayload = map[MessageType]interface{}{
		MessageTypeAuthentication: &MessageAuthenticationRequest{},
		MessageTypeHandshake:      &MessageHandshake{},
	}
	messageTypes = map[MessageType]struct{}{
		MessageTypeAuthentication: {},
		MessageTypeHandshake:      {},
		MessageTypeData:           {},
		MessageTypeKeepalive:      {},
		MessageTypeReset:          {},
	}
)

// Message represents a message that can be sent between the client and server.
type Message struct {
	ID               uint32
	SessionID        uint32
	Type             MessageType
	CompressionType  CompressionType
	CompressionLevel CompressionLevel
	Error            MessageError
	PayloadLength    uint16
	IsError          bool
	IsRequest        bool
	IsACK            bool
	Payload          interface{}
	PacketInfo       *NetworkPacketInfo
}

type MessageType uint8
type MessageError uint8

func (m MessageType) String() string {
	switch m {
	case MessageTypeAuthentication:
		return "auth"
	case MessageTypeHandshake:
		return "handshake"
	case MessageTypeData:
		return "data"
	case MessageTypeKeepalive:
		return "keepalive"
	case MessageTypeReset:
		return "reset"
	default:
		return "unknown"
	}
}

//	IsUserData checks if the message is a data message,
//
// which indicates it is encrypted with the user's key.
//
// It does this by checking if the message Type is MessageTypeData.
//
// Returns true if the Type is MessageTypeData, false otherwise.
func (m *Message) IsUserData() bool {
	return m.Type == MessageTypeData && !m.IsACK
}

// IsStreamMerge checks if the message is a data message,
// which indicates it contains a stream merge payload.
//
// It does this by checking if the message Type is MessageTypeData.
//
// Returns true if the Type is MessageTypeData, false otherwise.
func (m *Message) IsStreamMerge() bool {
	return m.Type == MessageTypeData && m.SessionID != 0
}

func (m *Message) WithWaitingList() bool {
	return m.PacketInfo != nil && m.PacketInfo.Protocol != IPProtocolUDP
}

// IsSendACK checks if the message is a data message
// that is not an ACK message.
//
// This indicates the receiver should send an ACK back.
//
// It checks the Type is MessageTypeData and IsACK is false.
//
// Returns true if the Type is MessageTypeData and IsACK is false.
func (m *Message) IsSendACK() bool {
	return m.Type == MessageTypeData && !m.IsACK
}

func (m *Message) IsPayload() bool {
	return !(m.Type == MessageTypeReset || m.Type == MessageTypeKeepalive)
}

func (m *Message) Reset() {
	m.ID = 0
	m.SessionID = 0
	m.Type = 0
	m.CompressionType = 0
	m.CompressionLevel = 0
	m.Error = 0
	m.PayloadLength = 0
	m.IsError = false
	m.IsRequest = false
	m.IsACK = false
	m.PacketInfo = nil
}

func (m *Message) GetEndpoint() PacketEndpoint {
	if m.PacketInfo == nil {
		return 0
	}
	return m.PacketInfo.Endpoint
}

// Valid checks if the given MessageType is a valid known type.
func (m MessageType) Valid() bool {
	_, ok := messageTypes[m]
	return ok
}

func (m MessageError) Error() error {
	if e, ok := messageErrorToError[m]; ok {
		return e
	}
	return ErrUnknown
}

// MessageAuthenticationRequest represents an authentication request message.
type MessageAuthenticationRequest struct {
	ClientID         string
	Name             string
	Password         string
	CompressionType  CompressionType
	CompressionLevel CompressionLevel
}

// MessageAuthenticationResponse represents an authentication response message.
type MessageAuthenticationResponse struct {
	SessionID uint32
	LocalIP   net.IP
	RemoteIP  net.IP
}

// MessageHandshake represents a handshake message.
type MessageHandshake struct {
	Key []byte
}
