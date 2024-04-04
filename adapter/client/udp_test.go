package client

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/business/usecase"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
)

const (
	mtu = 1500
)

func TestSend(t *testing.T) {
	log := logger.NewDefault()

	cfg := &Config{
		Codec:             usecase.GetCodec(log, mtu, entity.EncryptionAES256ECB, structs.Ref(false)),
		PrimaryEncryptor:  usecase.GetEncryptor(strings.Repeat("1", 32), entity.EncryptionAES256ECB),
		MTU:               mtu,
		ReadBufferSize:    131071,
		WriteBufferSize:   131071,
		MultipathTCP:      true,
		KeepaliveInterval: 10,
		SocketTracing:     false,
	}

	cli, err := New(context.Background(), log, cfg, nil, nil, nil)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	conn, err := cli.Run("192.168.33.2", 33333, entity.ProtoUDP)
	if err != nil {
		t.Fatalf("failed to create connection: %v", err)
	}

	msg := &entity.Message{
		ID:        0,
		SessionID: 0,
		Type:      entity.MessageTypeAuthentication,
		Payload: &entity.MessageAuthenticationRequest{
			ClientID: uuid.New().String(),
			Name:     strings.Repeat("n", 254),
			Password: strings.Repeat("p", 254),
		},
	}

	routines := 20
	messages := 10000

	wg := &sync.WaitGroup{}
	wg.Add(routines)

	for i := 0; i < routines; i++ {
		go func() {
			defer wg.Done()
			for i := 0; i < messages; i++ {
				err := cli.SendAsync(msg, conn)
				if err != nil {
					t.Errorf("failed to send message: %v", err)
				}
			}
		}()
	}

	wg.Wait()
}
