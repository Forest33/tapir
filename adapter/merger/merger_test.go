package merger

import (
	"context"
	"testing"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

type testMessage struct {
	msg *entity.Message
}

type testCase struct {
	input  []*testMessage
	output []*testMessage
}

var (
	testCases = map[string]testCase{
		"wrong_order_v1": {
			input: []*testMessage{
				{msg: &entity.Message{ID: 10, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 9, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 5, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 8, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 7, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 6, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 4, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 3, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 2, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 12, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 13, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 14, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 15, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 21, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 20, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 19, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 18, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 17, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 16, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 22, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
			},
			output: []*testMessage{
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 2, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 3, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 4, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 5, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 6, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 7, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 8, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 9, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 10, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 12, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 13, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 14, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 15, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 16, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 17, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 18, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 19, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 20, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 21, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 22, SessionID: 1, Type: entity.MessageTypeData}},
			},
		},
		"wrong_order_v2": {
			input: []*testMessage{
				{msg: &entity.Message{ID: 10, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 12, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 13, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 14, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 15, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 21, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 20, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 19, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 18, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 17, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 16, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 22, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
			},
			output: []*testMessage{
				{msg: &entity.Message{ID: 10, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 12, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 13, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 14, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 15, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 16, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 17, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 18, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 19, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 20, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 21, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 22, SessionID: 1, Type: entity.MessageTypeData}},
			},
		},
		"with_dup_and_wrong_order": {
			input: []*testMessage{
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 2, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 4, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 5, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 6, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 5, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 5, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 3, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 8, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 7, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 9, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 10, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 15, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 17, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 18, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 19, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 20, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 2, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 12, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 13, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 21, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 22, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 14, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 16, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 23, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 0, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 2, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 3, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
			},
			output: []*testMessage{
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 2, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 3, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 4, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 5, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 6, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 7, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 8, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 9, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 10, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 12, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 13, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 14, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 15, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 16, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 17, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 18, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 19, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 20, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 21, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 22, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 23, SessionID: 1, Type: entity.MessageTypeData}},
			},
		},
		"maximum_waiting_list_ttl_and_size": {
			input: []*testMessage{
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 2, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 3, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 4, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 5, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 7, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},
				{msg: &entity.Message{ID: 8, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},
				{msg: &entity.Message{ID: 9, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 11}}, // <- delay
				{msg: &entity.Message{ID: 10, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 0}},

				{msg: &entity.Message{ID: 6, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // ignore

				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // DUP
				{msg: &entity.Message{ID: 13, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},
				{msg: &entity.Message{ID: 14, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},
				{msg: &entity.Message{ID: 15, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},
				{msg: &entity.Message{ID: 16, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},
				{msg: &entity.Message{ID: 17, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},
				{msg: &entity.Message{ID: 18, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}}, // <- delay
				{msg: &entity.Message{ID: 19, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},
				{msg: &entity.Message{ID: 20, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}, PayloadLength: 10}},

				{msg: &entity.Message{ID: 12, SessionID: 1, Type: entity.MessageTypeData, Payload: []byte{}}}, // ignore
			},
			output: []*testMessage{
				{msg: &entity.Message{ID: 1, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 2, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 3, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 4, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 5, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 7, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 8, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 9, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 10, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 11, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 13, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 14, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 15, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 16, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 17, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 18, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 19, SessionID: 1, Type: entity.MessageTypeData}},
				{msg: &entity.Message{ID: 20, SessionID: 1, Type: entity.MessageTypeData}},
			},
		},
	}
)

func TestWrongOrderV1(t *testing.T) {
	testCase := testCases["wrong_order_v1"]

	m, err := NewV1(context.Background(), logger.NewDefault(), &Config{
		WaitingListMaxSize: 100,
		WaitingListMaxTTL:  10,
		StreamCount:        1,
	})
	if err != nil {
		t.Errorf("failed to create stream merger: %v", err)
		return
	}

	var (
		done       = make(chan struct{})
		receiveIdx int
	)

	m.SetReceiverHandler(func(msg *entity.Message, conn *entity.Connection) error {
		if testCase.output[receiveIdx].msg.ID != msg.ID {
			t.Errorf("wrong message id should be: %d received: %d", testCase.output[receiveIdx].msg.ID, msg.ID)
			done <- struct{}{}
			return nil
		}
		receiveIdx++
		if receiveIdx >= len(testCase.output) {
			done <- struct{}{}
		}
		return nil
	})

	m.SetDisconnectHandler(func(_ *entity.Connection, _ error) {})

	err = m.CreateStream(1)
	if err != nil {
		t.Errorf("failed to create stream: %v", err)
		return
	}

	for _, req := range testCase.input {
		err = m.Push(req.msg, nil)
		if err != nil {
			t.Errorf("failed add message: %v", err)
			continue
		}
	}

	<-done
}

func TestWrongOrderV2(t *testing.T) {
	testCase := testCases["wrong_order_v2"]

	m, err := NewV2(context.Background(), logger.NewDefault(), &Config{
		WaitingListMaxSize: 100,
		WaitingListMaxTTL:  10,
		StreamCount:        1,
	})
	if err != nil {
		t.Errorf("failed to create stream merger: %v", err)
		return
	}

	var (
		done       = make(chan struct{})
		receiveIdx int
	)

	m.SetReceiverHandler(func(msg *entity.Message, conn *entity.Connection) error {
		if testCase.output[receiveIdx].msg.ID != msg.ID {
			t.Errorf("wrong message id should be: %d received: %d", testCase.output[receiveIdx].msg.ID, msg.ID)
			done <- struct{}{}
			return nil
		}
		receiveIdx++
		if receiveIdx >= len(testCase.output) {
			done <- struct{}{}
		}
		return nil
	})

	m.SetDisconnectHandler(func(_ *entity.Connection, _ error) {})

	err = m.CreateStream(1)
	if err != nil {
		t.Errorf("failed to create stream: %v", err)
		return
	}

	for _, req := range testCase.input {
		err = m.Push(req.msg, nil)
		if err != nil {
			t.Errorf("failed add message: %v", err)
			continue
		}
	}

	<-done
}

func TestDupAndWrongOrderV1(t *testing.T) {
	testCase := testCases["with_dup_and_wrong_order"]

	m, err := NewV1(context.Background(), logger.NewDefault(), &Config{
		WaitingListMaxSize: 20,
		StreamCount:        1,
	})
	if err != nil {
		t.Errorf("failed to create stream merger: %v", err)
		return
	}

	var (
		done       = make(chan struct{})
		receiveIdx int
	)

	m.SetReceiverHandler(func(msg *entity.Message, conn *entity.Connection) error {
		if testCase.output[receiveIdx].msg.ID != msg.ID {
			t.Errorf("wrong message id should be: %d received: %d", testCase.output[receiveIdx].msg.ID, msg.ID)
			done <- struct{}{}
			return nil
		}
		receiveIdx++
		if receiveIdx >= len(testCase.output) {
			done <- struct{}{}
		}
		return nil
	})

	m.SetDisconnectHandler(func(_ *entity.Connection, _ error) {})

	err = m.CreateStream(1)
	if err != nil {
		t.Errorf("failed to create stream: %v", err)
		return
	}

	for _, req := range testCase.input {
		err = m.Push(req.msg, nil)
		if err != nil {
			t.Errorf("failed add message: %v", err)
			continue
		}
	}

	<-done
}

func TestWaitingListTTLV2(t *testing.T) {
	testCase := testCases["maximum_waiting_list_ttl_and_size"]

	m, err := NewV2(context.Background(), logger.NewDefault(), &Config{
		WaitingListMaxSize: 1000,
		WaitingListMaxTTL:  1,
		StreamCount:        1,
	})
	if err != nil {
		t.Errorf("failed to create stream merger: %v", err)
		return
	}

	var (
		done       = make(chan struct{})
		receiveIdx int
	)

	m.SetReceiverHandler(func(msg *entity.Message, conn *entity.Connection) error {
		if testCase.output[receiveIdx].msg.ID != msg.ID {
			t.Errorf("wrong message id should be: %d received: %d", testCase.output[receiveIdx].msg.ID, msg.ID)
			done <- struct{}{}
			return nil
		}
		receiveIdx++
		if receiveIdx >= len(testCase.output) {
			done <- struct{}{}
		}
		return nil
	})

	m.SetDisconnectHandler(func(_ *entity.Connection, _ error) {})

	err = m.CreateStream(1)
	if err != nil {
		t.Errorf("failed to create stream: %v", err)
		return
	}

	for _, req := range testCase.input {
		err = m.Push(req.msg, nil)
		if err != nil {
			t.Errorf("failed add message: %v", err)
			continue
		}
		if req.msg.ID == 9 || req.msg.ID == 18 {
			time.Sleep(2 * time.Second)
		}
	}

	<-done
}

func TestWaitingListSizeV2(t *testing.T) {
	testCase := testCases["maximum_waiting_list_ttl_and_size"]

	m, err := NewV2(context.Background(), logger.NewDefault(), &Config{
		WaitingListMaxSize: 30,
		WaitingListMaxTTL:  1,
		StreamCount:        1,
	})
	if err != nil {
		t.Errorf("failed to create stream merger: %v", err)
		return
	}

	var (
		done       = make(chan struct{})
		receiveIdx int
	)

	m.SetReceiverHandler(func(msg *entity.Message, conn *entity.Connection) error {
		if testCase.output[receiveIdx].msg.ID != msg.ID {
			t.Errorf("wrong message id should be: %d received: %d", testCase.output[receiveIdx].msg.ID, msg.ID)
			done <- struct{}{}
			return nil
		}
		receiveIdx++
		if receiveIdx >= len(testCase.output) {
			done <- struct{}{}
		}
		return nil
	})

	m.SetDisconnectHandler(func(_ *entity.Connection, _ error) {})

	err = m.CreateStream(1)
	if err != nil {
		t.Errorf("failed to create stream: %v", err)
		return
	}

	for _, req := range testCase.input {
		err = m.Push(req.msg, nil)
		if err != nil {
			t.Errorf("failed add message: %v", err)
			continue
		}
	}

	<-done
}
