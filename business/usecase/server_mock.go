package usecase

import (
	"sync"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/codec"
)

type MockNetworkServer struct {
	wg               *sync.WaitGroup
	count            int
	maxMessagesCount int
	codec            codec.Codec
	primaryEncryptor entity.Encryptor
}

func (*MockNetworkServer) Run(host string, port int, proto entity.Protocol) error {
	return nil
}

func (m *MockNetworkServer) SetStatisticHandler(f entity.StatisticHandler) {
}

func (m *MockNetworkServer) SetEncryptorGetter(f entity.EncryptorGetter) {
}

func (*MockNetworkServer) SetReceiverHandler(f entity.ReceiverHandler) {
}

func (*MockNetworkServer) SetDisconnectHandler(f entity.DisconnectHandler) {
}

func (*MockNetworkServer) DropSession(sessionID uint32) {
}

func (m *MockNetworkServer) Send(msg *entity.Message, conn *entity.Connection) error {
	m.count++
	if m.count == m.maxMessagesCount {
		m.wg.Done()
	}

	encodedHeader, encodedPayload, err := m.codec.Marshal(msg)
	if err != nil {
		return err
	}

	var cypherPayload []byte
	if len(encodedPayload) != 0 {
		cypherPayload, err = m.primaryEncryptor.Encrypt(encodedPayload)
		if err != nil {
			return err
		}
	}

	cypherHeader, err := m.primaryEncryptor.Encrypt(encodedHeader)
	if err != nil {
		return err
	}

	data := append(cypherHeader, cypherPayload...)
	_ = data

	//entity.MessagePool.Put(msg)

	return nil
}
