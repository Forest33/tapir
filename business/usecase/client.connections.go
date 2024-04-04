package usecase

import (
	"errors"
	"math/rand"
	"time"

	"github.com/rs/zerolog"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/structs"
)

func (uc *ClientUseCase) addConnection(conn *entity.Connection, cc *clientConn) {
	if _, ok := uc.connectionsMap[conn.Key()]; !ok {
		cc.idx = len(uc.connections)
		uc.connectionsMap[conn.Key()] = cc
		uc.connections = append(uc.connections, cc)
	}
}

func (uc *ClientUseCase) setConnectionEncryptor(conn *entity.Connection, enc entity.Encryptor) {
	if _, ok := uc.connectionsMap[conn.Key()]; ok {
		uc.connectionsMap[conn.Key()].encryptor = enc
	}
}

func (uc *ClientUseCase) removeConnection(conn *entity.Connection) {
	uc.connMux.Lock()

	if cc, ok := uc.connectionsMap[conn.Key()]; ok {
		uc.connections = structs.Delete(uc.connections, cc.idx)
		delete(uc.connectionsMap, conn.Key())
	}

	uc.connMux.Unlock()

	if len(uc.connectionsMap) == 0 {
		uc.Stop()
	}
}

func (uc *ClientUseCase) closeAndRemoveAllConnections() {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	for _, cc := range uc.connections {
		if err := cc.conn.Close(); err != nil {
			uc.log.Error().Err(err).Msg("failed to close connection")
		}
	}

	uc.connections = uc.connections[:0]
	uc.connectionsMap = make(map[entity.ConnectionKey]*clientConn, uc.conn.Server.MaxPorts())
}

func (uc *ClientUseCase) getConnection(packet *entity.NetworkPacketInfo) (cc *clientConn, err error) {
	uc.connMux.RLock()

	length := len(uc.connectionsMap)
	if length > 0 {
		switch uc.portSelectionStrategy {
		case entity.PortSelectionStrategyRandom:
			cc = uc.connections[rand.Intn(length)]
		case entity.PortSelectionStrategyHash:
			cc = uc.connections[int(packet.Endpoint.Uint64()%uint64(length))]
		default:
			err = entity.ErrNoPortSelectionStrategy
		}
	} else {
		err = entity.ErrConnectionNotExists
	}

	uc.connMux.RUnlock()

	return
}

func (uc *ClientUseCase) getConnectionEncryptor(conn *entity.Connection) (entity.Encryptor, error) {
	uc.connMux.RLock()
	var enc entity.Encryptor
	c, ok := uc.connectionsMap[conn.Key()]
	if ok {
		enc = c.encryptor
	}
	uc.connMux.RUnlock()
	return enc, structs.If(!ok, entity.ErrConnectionNotExists, nil)
}

func (uc *ClientUseCase) createConnection(port uint16, proto entity.Protocol) error {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	removeConnection := func(conn *entity.Connection) {
		if cc, ok := uc.connectionsMap[conn.Key()]; ok {
			uc.connections = structs.Delete(uc.connections, cc.idx)
			delete(uc.connectionsMap, conn.Key())
		}
	}

	for i := 0; i < uc.conn.Server.MaxConnectionAttempts; i++ {
		uc.log.Debug().
			Str("host", uc.conn.Server.Host).
			Uint16("port", port).
			Str("protocol", proto.String()).
			Int("attempt", i+1).
			Msg("connecting to server...")

		conn, err := uc.client.Run(uc.conn.Server.Host, port, proto)
		if err != nil {
			uc.log.Error().Err(err).
				Str("host", uc.conn.Server.Host).
				Uint16("port", port).
				Str("protocol", proto.String()).
				Msg("server connection error")
			time.Sleep(time.Second)
			continue
		}

		cc := &clientConn{
			conn:      conn,
			encryptor: GetEncryptor(uc.conn.Authentication.Key, uc.conn.Tunnel.Encryption),
		}

		uc.addConnection(conn, cc)

		if err := uc.commandAuthentication(cc); err != nil {
			if errors.Is(err, entity.ErrUnauthorized) {
				return err
			}
			removeConnection(conn)
			uc.log.Error().Err(err).
				Uint32("session_id", uc.sessionID).
				Str("host", uc.conn.Server.Host).
				Uint16("port", port).
				Str("protocol", proto.String()).
				Msg("server authentication error")
			time.Sleep(time.Second)
			continue
		}

		if err := uc.commandHandshake(cc); err != nil {
			removeConnection(conn)
			uc.log.Error().Err(err).
				Uint32("session_id", uc.sessionID).
				Str("host", uc.conn.Server.Host).
				Uint16("port", port).
				Str("protocol", proto.String()).
				Msg("server handshake error")
			time.Sleep(time.Second)
			continue
		}

		if proto == entity.ProtoTCP {
			uc.client.ReceiverTCP(conn, uc.sessionID)
		} else {
			uc.client.ReceiverUDP(conn, uc.sessionID)
		}

		return nil
	}

	return entity.ErrMaxConnectionAttempts
}

func (uc *ClientUseCase) disconnect(conn *entity.Connection, err error) {
	var ev *zerolog.Event
	if err != nil {
		ev = uc.log.Error().Err(err)
	} else {
		ev = uc.log.Info()
	}
	ev.Uint32("session_id", conn.SessionID).
		Str("addr", conn.Addr.String()).
		Msg("disconnected")

	uc.removeConnection(conn)
	if err := conn.Close(); err != nil && !entity.IsErrorInterruptingNetwork(err) {
		uc.log.Error().Err(err).Uint32("session_id", uc.sessionID).Msg("error when closing connection")
	}
	if err != nil && !uc.isExit.Load() {
		err := uc.createConnection(conn.Port, structs.If(conn.TCPConn != nil, entity.ProtoTCP, entity.ProtoUDP))
		if err != nil {
			uc.Stop()
		}
	}
}
