package usecase

import (
	"context"
	"sync"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

// ConnectionManagerUseCase object capable of interacting with ConnectionManagerUseCase
type ConnectionManagerUseCase struct {
	ctx              context.Context
	log              *logger.Logger
	cfg              *entity.ClientConfig
	createConnection createConnectionFunc
	shutdown         func()
	cfgHandler       configHandler
	connections      map[int]*entity.ConnectionInfo
	statistic        map[int]*entity.Statistic
	connMux          sync.RWMutex
	statMux          sync.RWMutex
	inStatCh         chan *connectionStatisticRequest
	outStatCh        chan map[int]*entity.Statistic
}

type createConnectionFunc func(clientConn *entity.ClientConnection, statHandler entity.StatisticHandler) (entity.ClientConnectionHandler, error)

type connectionStatisticRequest struct {
	ID   int
	stat *entity.Statistic
}

// NewConnectionManagerUseCase creates a new ConnectionManagerUseCase
func NewConnectionManagerUseCase(ctx context.Context, log *logger.Logger, cfg *entity.ClientConfig, createConnectionFunc createConnectionFunc,
	shutdown func(), cfgHandler configHandler) *ConnectionManagerUseCase {
	uc := &ConnectionManagerUseCase{
		ctx:              ctx,
		log:              log.Duplicate(log.With().Str("layer", "ucipc").Logger()),
		cfg:              cfg,
		createConnection: createConnectionFunc,
		shutdown:         shutdown,
		cfgHandler:       cfgHandler,
		connections:      make(map[int]*entity.ConnectionInfo, len(cfg.Connections)),
		statistic:        make(map[int]*entity.Statistic, cfg.MaxPorts()),
		inStatCh:         make(chan *connectionStatisticRequest, len(cfg.Connections)),
	}

	uc.sessionStat()

	return uc
}

func (uc *ConnectionManagerUseCase) Connect(connID int) error {
	if connID < 0 || connID >= len(uc.cfg.Connections) {
		return entity.ErrWrongConnectionId
	}

	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	for id, conn := range uc.connections {
		conn.Handler.Exit()
		delete(uc.connections, id)
		break
	}

	h, err := uc.createConnection(uc.cfg.Connections[connID], uc.createStatisticHandler(connID))
	if err != nil {
		uc.log.Error().Err(err).
			Int("connection_id", connID).
			Msg("failed to create connection")
		return err
	}

	if err := h.Start(); err != nil {
		uc.log.Error().Err(err).
			Int("connection_id", connID).
			Msg("failed to connection to server")
		return err
	}

	uc.connections[connID] = &entity.ConnectionInfo{
		ID:          connID,
		Handler:     h,
		IsConnected: true,
		ConnectTs:   time.Now().Unix(),
	}

	return nil
}

func (uc *ConnectionManagerUseCase) Disconnect(connID int) error {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	if _, ok := uc.connections[connID]; !ok {
		return entity.ErrWrongConnectionId
	}

	uc.connections[connID].Handler.Exit()
	delete(uc.connections, connID)

	return nil
}

func (uc *ConnectionManagerUseCase) GetConnections() ([]*entity.ConnectionInfo, error) {
	uc.connMux.RLock()
	defer uc.connMux.RUnlock()

	conns := make([]*entity.ConnectionInfo, len(uc.cfg.Connections))
	for id := range uc.cfg.Connections {
		if cc, ok := uc.connections[id]; ok {
			conns[id] = cc
			continue
		}
		conns[id] = &entity.ConnectionInfo{
			ID: id,
		}
	}

	return conns, nil
}

func (uc *ConnectionManagerUseCase) UpdateConfig(jsonData []byte) error {
	var err error
	uc.cfg, err = uc.cfg.Unmarshal(jsonData)
	if err != nil {
		uc.log.Error().Err(err).Msg("failed to unmarshal client config")
		return err
	}

	uc.cfgHandler.Update(uc.cfg)
	uc.cfgHandler.Save()

	return nil
}

func (uc *ConnectionManagerUseCase) Shutdown() {
	uc.connMux.Lock()
	defer uc.connMux.Unlock()

	for _, conn := range uc.connections {
		conn.Handler.Exit()
	}

	uc.shutdown()
}

func (uc *ConnectionManagerUseCase) SetStatisticChannel(ch chan map[int]*entity.Statistic) {
	uc.statMux.Lock()
	uc.outStatCh = ch
	uc.statMux.Unlock()
}
