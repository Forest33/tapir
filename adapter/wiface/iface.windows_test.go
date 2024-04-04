//go:build windows

package wiface

import (
	"context"
	"net"
	"testing"

	"github.com/google/uuid"

	"github.com/forest33/tapir/adapter/packet"
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/command"
	"github.com/forest33/tapir/pkg/logger"
)

func TestCreate(t *testing.T) {
	log := logger.NewDefault()

	cfg := &Config{
		Tracing:     true,
		ServerHost:  "8.8.8.8",
		EndpointTTL: 600,
		Tunnel: &entity.TunnelConfig{
			MTU:                    1500,
			AddrMin:                "192.168.30.0",
			AddrMax:                "192.168.50.0",
			InterfaceUp:            entity.DefaultClientInterfaceUp,
			InterfaceDown:          entity.DefaultClientInterfaceDown,
			NumberOfHandlerThreads: 1,
			Encryption:             entity.EncryptionAES256ECB,
		},
	}

	ifacePacketDecoder := packet.New(&packet.Config{
		EndpointHashType: packet.EndpointHashDestinationAddress,
	})

	cmd, err := command.New(&entity.SystemConfig{
		ClientID: uuid.NewString(),
		Shell:    "powershell.exe",
	})
	if err != nil {
		t.Fatalf("failed to create command executer: %v", err)
	}

	iface, err := New(log, cfg, cmd, ifacePacketDecoder)
	if err != nil {
		t.Fatalf("failed to create interface adapter: %v", err)
	}

	ch := make(chan *entity.Message, 10)
	_, cancel := context.WithCancel(context.Background())

	local, _, _ := net.ParseCIDR("192.168.50.1/32")

	ifc, err := iface.Create(&entity.Interface{
		Type: entity.DeviceTypeTUN,
		IP: entity.IfIP{
			ServerLocal: local,
		},
		Receiver: ch,
		Cancel:   cancel,
	})
	if err != nil {
		t.Fatalf("failed to create interface: %v", err)
	}

	err = iface.Close(ifc)
	if err != nil {
		t.Fatalf("failed to close interface: %v", err)
	}
}
