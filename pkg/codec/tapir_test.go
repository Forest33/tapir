package codec

import (
	"fmt"
	"math"
	"net"
	"reflect"
	"strings"
	"testing"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/encryptor"
	"github.com/forest33/tapir/pkg/logger"
)

type testCase struct {
	request *entity.Message
}

const ackMaxSize = 1000

var (
	enc = encryptor.NewAESECB(strings.Repeat("0", 32))
	//enc   = encryptor.NewAESGCM(strings.Repeat("0", 32))
	codec = NewTapirCodec(logger.NewDefault(), &Config{
		HeaderSize:  entity.HeaderSize,
		PayloadSize: 1439,
		GetLength:   enc.GetLength,
	})

	ack = entity.NewMessageAcknowledgement(nil).SetMaxSize(ackMaxSize)

	compressPayload = []byte("test payload, test payload, test payload, test payload, test payload, test payload!")

	testData = map[string]testCase{
		"auth-request": {
			request: &entity.Message{
				Type:      entity.MessageTypeAuthentication,
				SessionID: math.MaxUint32,
				Error:     entity.GetMessageError(entity.ErrNoError),
				Payload: &entity.MessageAuthenticationRequest{
					ClientID:         "ad73d333-d19e-55dd-9e33-2e9ae43e9178",
					Name:             "user",
					Password:         "123456",
					CompressionType:  entity.CompressionZSTD,
					CompressionLevel: entity.CompressionLevel(3),
				},
			},
		},
		"auth-response": {
			request: &entity.Message{
				Type:  entity.MessageTypeAuthentication,
				Error: entity.GetMessageError(entity.ErrNoError),
				Payload: &entity.MessageAuthenticationResponse{
					SessionID: 5,
					LocalIP:   net.ParseIP("192.168.33.1").To4(),
					RemoteIP:  net.ParseIP("192.168.33.2").To4(),
				},
			},
		},
		"handshake": {
			request: &entity.Message{
				Type:  entity.MessageTypeHandshake,
				Error: entity.GetMessageError(entity.ErrNoError),
				Payload: &entity.MessageHandshake{
					Key: []byte("client-public-key"),
				},
			},
		},
		"data": {
			request: &entity.Message{
				Type:      entity.MessageTypeData,
				ID:        3,
				SessionID: 5,
				Payload:   []byte("test payload!"),
			},
		},
		"data-ack": {
			request: &entity.Message{
				Type:      entity.MessageTypeData,
				IsACK:     true,
				ID:        1,
				SessionID: 3,
				Payload:   ack,
			},
		},
		"data-compress-lz4": {
			request: &entity.Message{
				Type:            entity.MessageTypeData,
				CompressionType: entity.CompressionLZ4,
				ID:              3,
				SessionID:       5,
				Payload:         compressPayload,
			},
		},
		"data-compress-lzo": {
			request: &entity.Message{
				Type:            entity.MessageTypeData,
				CompressionType: entity.CompressionLZO,
				ID:              3,
				SessionID:       5,
				Payload:         compressPayload,
			},
		},
		"data-compress-zstd": {
			request: &entity.Message{
				Type:            entity.MessageTypeData,
				CompressionType: entity.CompressionZSTD,
				ID:              3,
				SessionID:       5,
				Payload:         compressPayload,
			},
		},
	}
)

func init() {
	ack.Push(1, 100)
	ack.Push(1, 200)
	ack.Push(1, 300)
	ack.Push(math.MaxUint64-1, 123)
	ack.Push(math.MaxUint64-1, 678)
	ack.Push(math.MaxUint64-1, 2342)
	ack.Push(math.MaxUint64-1, 905)
	ack.Push(math.MaxUint64, 1)
	ack.Push(math.MaxUint64, 2)
	ack.Push(math.MaxUint64, 3)
	ack.Push(math.MaxUint64, 4)
	ack.Push(math.MaxUint64, 5)
}

func TestAcknowledgement(t *testing.T) {
	out, err := codec.MarshalAcknowledgement(ack)
	if err != nil {
		t.Fatalf("failed to marshal acknowledgement: %v", err)
	}

	ack2, err := codec.UnmarshalAcknowledgement(out)
	if err != nil {
		t.Fatalf("failed to unmarshal acknowledgement: %v", err)
	}

	if !reflect.DeepEqual(ack.Get(), ack2.Get()) {
		t.Errorf("acknowledgement mismatch, want: %v, got: %v", ack.Get(), ack2.Get())
	}
}

func TestMessages(t *testing.T) {
	for k, v := range testData {
		header, payload, err := codec.Marshal(v.request)
		if err != nil {
			t.Errorf("[%s] failed to marshal: %v", k, err)
			continue
		}

		m := &entity.Message{}
		if err := codec.UnmarshalHeader(header, m); err != nil {
			t.Errorf("[%s] failed to unmarshal: %v", k, err)
			continue
		}

		m.Payload = payload
		err = codec.UnmarshalPayload(m)
		if err != nil {
			t.Errorf("[%s] failed to unmarshal payload: %v", k, err)
			continue
		}

		if p, ok := m.Payload.(*entity.MessageAcknowledgement); ok {
			if reflect.DeepEqual(v.request.Payload.(*entity.MessageAcknowledgement).Get(), p.Get()) {
				fmt.Printf("unmarshal-payload-%s - ok\n", k)
			} else {
				t.Errorf("unmarshal-payload-%s - fail\n", k)
				fmt.Printf("req: %+v\n", v.request.Payload)
				fmt.Printf("res: %+v\n", m.Payload)
				continue
			}
		} else {
			if reflect.DeepEqual(v.request.Payload, m.Payload) {
				fmt.Printf("unmarshal-payload-%s - ok\n", k)
			} else {
				t.Errorf("unmarshal-payload-%s - fail\n", k)
				fmt.Printf("req: %+v\n", v.request.Payload)
				fmt.Printf("res: %+v\n", m.Payload)
				continue
			}
		}

		m.PayloadLength = 0
		m.Payload = v.request.Payload
		v.request.IsRequest = m.IsRequest
		if reflect.DeepEqual(m, v.request) {
			fmt.Printf("unmarshal-header-%s - ok\n", k)
		} else {
			t.Errorf("unmarshal-header-%s - fail\n", k)
			fmt.Printf("req: %+v\n", v.request)
			fmt.Printf("res: %+v\n", *m)
			continue
		}
	}
}
