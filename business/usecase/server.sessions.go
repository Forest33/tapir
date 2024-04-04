package usecase

import (
	"math"
	"math/rand"
	"time"

	"github.com/forest33/tapir/business/entity"
)

func (uc *ServerUseCase) createSession(clientID, userName string) uint32 {
	uc.sessMux.Lock()
	defer uc.sessMux.Unlock()

	sessionID, ok := uc.client2session[clientID]
	if !ok {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for {
			sessionID = uint32(r.Intn(math.MaxUint32-1) + 1)
			if _, ok := uc.sessions[sessionID]; !ok {
				break
			}
		}
		uc.sessions[sessionID] = &ServerSessionInfo{
			ClientID: clientID,
			UserName: userName,
			Stat:     &entity.Statistic{},
		}
		uc.client2session[clientID] = sessionID
	}

	return sessionID
}

func (uc *ServerUseCase) checkSession(sessionID uint32, clientID, userName string) error {
	uc.sessMux.RLock()
	defer uc.sessMux.RUnlock()

	if s, ok := uc.sessions[sessionID]; !ok {
		return entity.ErrSessionNotExists
	} else if s.ClientID != clientID || s.UserName != userName {
		return entity.ErrUnauthorized
	}

	return nil
}

func (uc *ServerUseCase) dropSessionByID(sessionID uint32) error {
	uc.sessMux.Lock()
	defer uc.sessMux.Unlock()

	uc.log.Info().
		Uint32("session_id", sessionID).
		Msg("dropping session by session id")

	if _, ok := uc.sessions[sessionID]; !ok {
		return entity.ErrSessionNotExists
	}

	uc.dropSession(sessionID)

	return nil
}

func (uc *ServerUseCase) dropSessionByClientID(clientID string) error {
	uc.sessMux.Lock()
	defer uc.sessMux.Unlock()

	sessionID, ok := uc.client2session[clientID]
	if !ok {
		return entity.ErrSessionNotExists
	}

	uc.log.Info().
		Str("client_id", clientID).
		Uint32("session_id", sessionID).
		Msg("dropping session by client id")

	uc.dropSession(sessionID)

	return nil
}

func (uc *ServerUseCase) dropSession(sessionID uint32) {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	uc.log.Info().
		Uint32("session_id", sessionID).
		Str("client_id", uc.sessions[sessionID].ClientID).
		Msg("dropping session")

	ifName := uc.sessions[sessionID].IfName
	if _, ok := uc.interfaces[ifName]; !ok {
		uc.log.Error().
			Uint32("session_id", sessionID).
			Str("if", ifName).
			Msg("interface not exists")
		return
	}

	for _, conn := range uc.interfaces[ifName].Connections {
		if conn.Retry != nil {
			conn.Retry.Stop()
		}
		if conn.Ack != nil {
			conn.Ack.Stop()
		}
	}
	if err := uc.interfaces[ifName].handler.Close(); err != nil {
		uc.log.Error().Err(err).Msg("failed to close network interface")
	}

	delete(uc.interfaces, ifName)
	delete(uc.client2session, uc.sessions[sessionID].ClientID)
	delete(uc.sessions, sessionID)
	uc.merger.DeleteStream(sessionID)
	uc.srv.DropSession(sessionID)
}
