package usecase

import (
	"context"
	"encoding/binary"
	"math/rand"
	"net"
	"time"

	"github.com/forest33/tapir/business/entity"
)

func (uc *ServerUseCase) createInterface(sessionID uint32) (*ServerInterfaceInfo, error) {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	if sess, ok := uc.sessions[sessionID]; ok && len(sess.IfName) != 0 {
		return uc.interfaces[sess.IfName], nil
	}

	ctx, cancel := context.WithCancel(uc.ctx)
	ch := make(chan *entity.Message, uc.cfg.Tunnel.NumberOfHandlerThreads*10)
	for i := 0; i < uc.cfg.Tunnel.NumberOfHandlerThreads; i++ {
		uc.interfaceLoop(ctx, ch)
	}

	ifc, err := uc.iface.Create(&entity.Interface{
		Type:     entity.DeviceTypeTUN,
		IP:       uc.getTunnelIP(),
		Receiver: ch,
		Cancel:   cancel,
	})
	if err != nil {
		return nil, err
	}

	ic := &ServerInterfaceInfo{
		handler:     ifc,
		Connections: make([]*entity.Connection, 0, uc.cfg.Network.MaxPorts()),
		SessionID:   sessionID,
	}

	ifName, _ := ifc.Name()
	uc.interfaces[ifName] = ic
	uc.sessions[sessionID].IfName = ifName

	uc.log.Info().
		Uint32("session_id", sessionID).
		Str("device", ifName).
		Int("MTU", uc.cfg.Tunnel.MTU).
		Str("server_local_ip", ifc.IP.ServerLocal.String()).
		Str("server_remote_ip", ifc.IP.ServerRemote.String()).
		Str("client_local_ip", ifc.IP.ClientLocal.String()).
		Str("server_remote_ip", ifc.IP.ClientRemote.String()).
		Msg("network interface created")

	return ic, nil
}

func (uc *ServerUseCase) getTunnelIP() entity.IfIP {
	var (
		maxIP uint32
		last  *entity.Interface
	)

	for _, ifc := range uc.interfaces {
		ip := binary.BigEndian.Uint32(ifc.handler.IP.ClientRemote.To4())
		if ip > maxIP {
			maxIP = ip
			last = ifc.handler
		}
	}

	fromIP := binary.BigEndian.Uint32(net.ParseIP(uc.cfg.Tunnel.AddrMin).To4())
	if last != nil {
		fromIP = binary.BigEndian.Uint32(last.IP.ClientRemote.To4()) + 1
	}

	ip := entity.IfIP{
		ServerLocal:  int2ip(fromIP),
		ServerRemote: int2ip(fromIP + 1),
		ClientLocal:  int2ip(fromIP + 2),
		ClientRemote: int2ip(fromIP + 3),
	}

	return ip
}

func (uc *ServerUseCase) getInterfaceByName(ifName string) (ifc *entity.Interface, exists bool) {
	uc.connMux.RLock()

	var ic *ServerInterfaceInfo
	if ic, exists = uc.interfaces[ifName]; exists {
		ifc = ic.handler
	}

	uc.connMux.RUnlock()

	return
}

func (uc *ServerUseCase) addInterfaceConnection(sc *serverConn, conn *entity.Connection) error {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	if ic, ok := uc.interfaces[sc.ifName]; ok {
		conn.CompressionType = sc.compressionType
		conn.CompressionLevel = sc.compressionLevel
		conn.CreatedAt = time.Now().Unix()
		for i, cv := range ic.Connections {
			if cv.SessionID == sc.sessionID && cv.Protocol() == sc.protocol && cv.Port == sc.port {
				ic.Connections[i] = conn
				return nil
			}
		}
		ic.Connections = append(ic.Connections, conn)
		return nil
	}

	return entity.ErrInterfaceNotExists
}

func (uc *ServerUseCase) getInterfaceConnection(packet *entity.NetworkPacketInfo) (ic *ServerInterfaceInfo, conn *entity.Connection, err error) {
	uc.connMux.RLock()

	var ok bool
	if ic, ok = uc.interfaces[packet.IfName]; ok {
		length := len(ic.Connections)
		if length > 0 {
			switch uc.portSelectionStrategy {
			case entity.PortSelectionStrategyRandom:
				conn = uc.interfaces[packet.IfName].Connections[rand.Intn(length)]
			case entity.PortSelectionStrategyHash:
				conn = uc.interfaces[packet.IfName].Connections[int(packet.Endpoint.Uint64()%uint64(length))]
			default:
				err = entity.ErrNoPortSelectionStrategy
			}
		} else {
			err = entity.ErrConnectionNotExists
		}
	} else {
		err = entity.ErrInterfaceNotExists
	}

	uc.connMux.RUnlock()

	return
}

func (uc *ServerUseCase) interfaceLoop(ctx context.Context, ch chan *entity.Message) {
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

func (uc *ServerUseCase) interfaceReceiver(msg *entity.Message) error {
	if msg == nil || msg.PacketInfo == nil {
		uc.log.Fatalf("NIL MESSAGE: %+v", msg)
	}

	_, conn, err := uc.getInterfaceConnection(msg.PacketInfo)
	if err != nil {
		uc.log.Warn().Err(err).Str("if", msg.PacketInfo.IfName).Msg("failed to get interface connections")
		return nil
	} else if conn == nil {
		return nil
	}

	msg.SessionID = conn.SessionID
	msg.Type = entity.MessageTypeData
	msg.CompressionType = conn.CompressionType
	msg.CompressionLevel = conn.CompressionLevel

	uc.iface.ReceiveLog(msg)

	err = uc.srv.Send(msg, conn)
	if err != nil {
		uc.log.Error().Err(err).Uint32("id", msg.ID).Msg("failed to send data frame")
	}

	uc.addSessionStat(msg.SessionID, &entity.Statistic{
		IncomingBytes:  uint64(msg.PayloadLength),
		IncomingFrames: 1,
	})

	return err
}
