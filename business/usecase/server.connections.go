package usecase

import (
	"slices"

	"github.com/rs/zerolog"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/structs"
)

type serverConn struct {
	ifName           string
	encryptor        entity.Encryptor
	sessionID        uint32
	port             uint16
	protocol         entity.Protocol
	compressionType  entity.CompressionType
	compressionLevel entity.CompressionLevel
}

func (uc *ServerUseCase) addConnection(conn *entity.Connection, sc *serverConn) {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()
	for ck, cv := range uc.connections {
		if cv.sessionID == sc.sessionID && cv.protocol == sc.protocol && cv.port == sc.port {
			delete(uc.connections, ck)
		}
	}
	uc.connections[conn.Key()] = sc
}

func (uc *ServerUseCase) getConnection(conn *entity.Connection) (sc *serverConn, exists bool) {
	uc.connMux.RLock()
	sc, exists = uc.connections[conn.Key()]
	uc.connMux.RUnlock()
	return
}

func (uc *ServerUseCase) removeConnection(conn *entity.Connection) {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	if c, ok := uc.connections[conn.Key()]; ok {
		delete(uc.connections, conn.Key())

		if len(c.ifName) != 0 {
			if ifc, ok := uc.interfaces[c.ifName]; ok {
				i := slices.IndexFunc(ifc.Connections, func(c *entity.Connection) bool { return c.Key() == conn.Key() })
				if i != -1 {
					uc.interfaces[c.ifName].Connections = slices.Delete(uc.interfaces[c.ifName].Connections, i, i+1)
				}

				if len(uc.interfaces[c.ifName].Connections) == 0 {
					uc.merger.DeleteStream(conn.SessionID)

					if sess, ok := uc.sessions[conn.SessionID]; ok {
						delete(uc.client2session, sess.ClientID)
					}
					delete(uc.sessions, conn.SessionID)
					uc.srv.DropSession(conn.SessionID)

					if err := uc.interfaces[c.ifName].handler.Close(); err != nil {
						uc.log.Error().Err(err).Msg("failed to close network interface")
					}
					delete(uc.interfaces, c.ifName)
				}
			}
		}
	}
}

func (uc *ServerUseCase) setConnectionEncryptor(conn *entity.Connection, enc entity.Encryptor) {
	uc.connMux.Lock()
	uc.connections[conn.Key()].encryptor = enc
	uc.connMux.Unlock()
}

func (uc *ServerUseCase) getConnectionEncryptor(conn *entity.Connection) (entity.Encryptor, error) {
	uc.connMux.RLock()
	var enc entity.Encryptor
	c, ok := uc.connections[conn.Key()]
	if ok {
		enc = c.encryptor
	}
	uc.connMux.RUnlock()
	return enc, structs.If(!ok, entity.ErrConnectionNotExists, nil)
}

func (uc *ServerUseCase) disconnect(conn *entity.Connection, err error) {
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
}
