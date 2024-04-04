package usecase

import (
	"context"
	"net"

	"github.com/forest33/tapir/business/entity"
)

func (uc *ClientUseCase) createInterface(localIP, remoteIP net.IP, conn *entity.Connection) error {
	if uc.interfaceConn == nil {
		ctx, cancel := context.WithCancel(uc.ctx)
		ch := make(chan *entity.Message, uc.conn.Tunnel.NumberOfHandlerThreads*10)
		for i := 0; i < uc.conn.Tunnel.NumberOfHandlerThreads; i++ {
			uc.interfaceLoop(ctx, ch)
		}

		ifc, err := uc.iface.Create(&entity.Interface{
			Type: entity.DeviceTypeTUN,
			IP: entity.IfIP{
				ServerLocal:  localIP,
				ServerRemote: remoteIP,
			},
			Receiver: ch,
			Cancel:   cancel,
		})
		if err != nil {
			return err
		}

		uc.interfaceConn = &clientInterfaceInfo{
			handler:     ifc,
			connections: make([]*entity.Connection, 0, uc.conn.Server.MaxPorts()),
		}

		name, _ := ifc.Name()
		uc.log.Info().
			Uint32("session_id", conn.SessionID).
			Str("device", name).
			Int("MTU", uc.conn.Tunnel.MTU).
			Str("server_local_ip", ifc.IP.ServerLocal.String()).
			Str("server_remote_ip", ifc.IP.ServerRemote.String()).
			Str("client_local_ip", ifc.IP.ClientLocal.String()).
			Str("server_remote_ip", ifc.IP.ClientRemote.String()).
			Msg("network interface created")
	}

	uc.interfaceConn.connections = append(uc.interfaceConn.connections, conn)

	return nil
}

func (uc *ClientUseCase) closeInterface() {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	if uc.interfaceConn != nil {
		if err := uc.interfaceConn.handler.Close(); err != nil {
			uc.log.Error().Err(err).Msg("failed to close interface")
		}
		uc.interfaceConn = nil
	}
}

func (uc *ClientUseCase) interfaceLoop(ctx context.Context, ch chan *entity.Message) {
	go func() {
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if err := uc.interfaceReceiver(msg); err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (uc *ClientUseCase) interfaceReceiver(msg *entity.Message) error {
	if !uc.isConnected.Load() {
		return nil
	}

	cc, err := uc.getConnection(msg.PacketInfo)
	if err != nil {
		uc.log.Error().Err(err).Str("interface", msg.PacketInfo.IfName).Msg("no connection")
		return err
	}

	msg.SessionID = uc.sessionID
	msg.Type = entity.MessageTypeData
	msg.CompressionType = uc.compressionType
	msg.CompressionLevel = uc.compressionLevel

	uc.iface.ReceiveLog(msg)

	if err := uc.client.SendAsync(msg, cc.conn); err != nil {
		uc.log.Error().Err(err).Uint32("id", msg.ID).Msg("failed to send data frame")
	}

	uc.addConnectionStat(msg.SessionID, &entity.Statistic{
		OutgoingBytes:  uint64(msg.PayloadLength),
		OutgoingFrames: 1,
	})

	return nil
}
