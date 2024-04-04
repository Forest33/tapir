package entity

import (
	"errors"
	"io"
	"net"
)

var (
	ErrUnknown                     = errors.New("unknown error")
	ErrNoError                     = errors.New("not a error")
	ErrInternalError               = errors.New("internal error")
	ErrCommandShellUndefined       = errors.New("command shell undefined")
	ErrConnectionNotExists         = errors.New("connection not exists ")
	ErrInterfaceNotExists          = errors.New("interface not exists ")
	ErrSessionNotExists            = errors.New("session not exists ")
	ErrWrongMessageHeaderSize      = errors.New("wrong message header size")
	ErrWrongMessagePayload         = errors.New("wrong message payload")
	ErrEmptyMessagePayload         = errors.New("empty message payload")
	ErrWrongMessagePayloadSize     = errors.New("wrong message payload size")
	ErrWrongMessageAcknowledgement = errors.New("wrong message acknowledgement")
	ErrMaxBytesSize                = errors.New("maximum size for byte field exceeded")
	ErrWrongBytesSize              = errors.New("wrong bytes size")
	ErrUnknownCommand              = errors.New("unknown command")
	ErrUnauthorized                = errors.New("invalid username or password")
	ErrReceiverHandlerNotSet       = errors.New("receiver handler is not set")
	ErrDisconnectHandlerNotSet     = errors.New("disconnect handler is not set")
	ErrMaxConnectionAttempts       = errors.New("maximum number of connection attempts exceeded")
	ErrKeepaliveTimeoutExceeded    = errors.New("keepalive timeout exceeded")
	ErrWrongPacketLength           = errors.New("wrong packet length")
	ErrWrongPacketData             = errors.New("wrong packet data")
	ErrNoPortSelectionStrategy     = errors.New("no port selection strategy")
	ErrUnknownEncryptionMethod     = errors.New("unknown encryption method")
	ErrWrongConnectionId           = errors.New("wrong connection index")
	ErrGrpcServerUnavailable       = errors.New("gRPC server is unavailable")
	ErrValidation                  = errors.New("validation error")
)

var (
	unknownMessageError     MessageError = 0xff
	errorToMessageErrorCode              = map[error]MessageError{
		ErrNoError:             0x00,
		ErrWrongMessagePayload: 0x01,
		ErrUnknownCommand:      0x02,
		ErrUnauthorized:        0x03,
		ErrInternalError:       0x04,
	}
	messageErrorToError map[MessageError]error
)

func init() {
	messageErrorToError = make(map[MessageError]error, len(errorToMessageErrorCode))
	for k, v := range errorToMessageErrorCode {
		messageErrorToError[v] = k
	}
}

func GetMessageError(err error) MessageError {
	if code, ok := errorToMessageErrorCode[err]; ok {
		return code
	}
	return unknownMessageError
}

func IsErrorInterruptingNetwork(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return opErr.Timeout() || errors.Is(opErr, net.ErrClosed)
	}
	return errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed)
}
