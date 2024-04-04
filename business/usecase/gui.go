package usecase

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"text/template"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
)

const (
	guiChannelCapacity = 10
	clientWaitingTime  = time.Second * 10
	maxLogFileLines    = 1000
)

// GUIUseCase object capable of interacting with GUIUseCase
type GUIUseCase struct {
	ctx        context.Context
	log        *logger.Logger
	cfg        *entity.ClientConfig
	cfgHandler configHandler
	ipcClient  ipcClient
	cmd        entity.CommandExecutor
	homeDir    string
	messageCh  chan *entity.GUIRequest
}

type ipcClient interface {
	Start(ctx context.Context, connID int32) (*entity.ClientState, error)
	Stop(ctx context.Context, connID int32) (*entity.ClientState, error)
	GetState(ctx context.Context) (*entity.ClientState, error)
	StatisticStream(ctx context.Context) (chan map[int]*entity.Statistic, error)
	UpdateConfig(ctx context.Context, jsonData []byte) error
	Shutdown(ctx context.Context) error
}

type initializationRequest struct {
	Config *entity.ClientConfig
	State  *entity.ClientState
}

// NewGUIUseCase creates a new GUIUseCase
func NewGUIUseCase(ctx context.Context, log *logger.Logger, cfg *entity.ClientConfig, cfgHandler configHandler,
	ipcClient ipcClient, cmd entity.CommandExecutor, homeDir string) (*GUIUseCase, error) {
	uc := &GUIUseCase{
		ctx:        ctx,
		log:        log,
		cfg:        cfg,
		cfgHandler: cfgHandler,
		ipcClient:  ipcClient,
		cmd:        cmd,
		homeDir:    homeDir,
		messageCh:  make(chan *entity.GUIRequest, guiChannelCapacity),
	}

	uc.init()

	return uc, nil
}

func (uc *GUIUseCase) init() {
	go func() {
		var (
			state *entity.ClientState
			err   error
		)

		defer func() {
			if err == nil {
				statCh, err := uc.ipcClient.StatisticStream(uc.ctx)
				if err != nil {
					uc.log.Error().Err(err).Msg("failed to get statistic stream")
					return
				}
				uc.statisticHandler(statCh)
			}
		}()

		state, err = uc.ipcClient.GetState(uc.ctx)
		if errors.Is(err, entity.ErrGrpcServerUnavailable) {
			if err = uc.runClient(); err == nil {
				select {
				case <-time.After(clientWaitingTime):
					uc.messageCh <- &entity.GUIRequest{Cmd: entity.CmdInitialization, Error: entity.ErrGrpcServerUnavailable}
					uc.sendInitialization(nil, nil, entity.ErrGrpcServerUnavailable)
					return
				default:
					for {
						state, err = uc.ipcClient.GetState(uc.ctx)
						if err == nil {
							uc.sendInitialization(uc.cfg, state, nil)
							return
						}
						time.Sleep(time.Millisecond * 100)
					}
				}
			}
		} else if err != nil {
			uc.sendInitialization(nil, nil, err)
		} else {
			uc.sendInitialization(uc.cfg, state, nil)
		}
	}()
}

func (uc *GUIUseCase) sendInitialization(cfg *entity.ClientConfig, state *entity.ClientState, err error) {
	if err != nil {
		uc.messageCh <- &entity.GUIRequest{Cmd: entity.CmdInitialization, Error: err}
		return
	}
	uc.messageCh <- &entity.GUIRequest{Cmd: entity.CmdInitialization, Payload: &initializationRequest{
		Config: cfg,
		State:  state,
	}}
}

func (uc *GUIUseCase) CommandConnect(payload map[string]interface{}) *entity.GUIResponse {
	req := &entity.ConnectRequest{}
	if err := req.Model(payload); err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	var (
		err   error
		state *entity.ClientState
	)

	if req.IsConnect {
		state, err = uc.ipcClient.Start(uc.ctx, req.ConnID)
	} else {
		state, err = uc.ipcClient.Stop(uc.ctx, req.ConnID)
	}
	if err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	return &entity.GUIResponse{
		Payload: state,
	}
}

func (uc *GUIUseCase) CommandConnectionUpdate(payload map[string]interface{}) *entity.GUIResponse {
	req := &entity.UpdateConnectionRequest{}
	if err := req.Model(payload); err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	if int(req.ConnID) >= len(uc.cfg.Connections) || req.ConnID < 0 {
		return &entity.GUIResponse{Error: entity.ErrValidation.Error()}
	}

	uc.cfg.Connections[req.ConnID].Name = req.Name
	uc.cfg.Connections[req.ConnID].Server.Host = req.ServerHost
	uc.cfg.Connections[req.ConnID].Server.PortMin = req.PortMin
	uc.cfg.Connections[req.ConnID].Server.PortMax = req.PortMax
	uc.cfg.Connections[req.ConnID].Server.UseTCP = req.UseTCP
	uc.cfg.Connections[req.ConnID].Server.UseUDP = req.UseUDP
	uc.cfg.Connections[req.ConnID].User.Name = req.Username
	uc.cfg.Connections[req.ConnID].User.Password = req.Password

	if err := uc.updateConfig(); err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	return &entity.GUIResponse{}
}

func (uc *GUIUseCase) CommandConnectionDelete(payload map[string]interface{}) *entity.GUIResponse {
	req := &entity.DeleteConnectionRequest{}
	if err := req.Model(payload); err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	if int(req.ConnID) >= len(uc.cfg.Connections) || req.ConnID < 0 {
		return &entity.GUIResponse{Error: entity.ErrValidation.Error()}
	}

	uc.cfg.Connections = slices.Delete(uc.cfg.Connections, int(req.ConnID), int(req.ConnID)+1)

	if err := uc.updateConfig(); err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	return &entity.GUIResponse{}
}

func (uc *GUIUseCase) CommandConnectionImport(payload map[string]interface{}) *entity.GUIResponse {
	req := &entity.ImportConnectionRequest{}
	if err := req.Model(payload); err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	data, err := os.ReadFile(req.File)
	if err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	conn := &entity.ClientConnection{}
	if err := conn.Unmarshal(data); err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	uc.cfg.Connections = append(uc.cfg.Connections, conn)

	if err := uc.updateConfig(); err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}

	return &entity.GUIResponse{}
}

func (uc *GUIUseCase) CommandLogsGet() *entity.GUIResponse {
	f, err := os.Open(filepath.Join(uc.homeDir, entity.ClientLogFile))
	if err != nil {
		return &entity.GUIResponse{Error: err.Error()}
	}
	defer func() {
		if err := f.Close(); err != nil {
			uc.log.Error().Err(err).Msg("failed to close log file")
		}
	}()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, string(sc.Bytes()))
	}

	if l := len(lines); l > maxLogFileLines {
		lines = lines[l-maxLogFileLines : l]
	}

	return &entity.GUIResponse{Payload: lines}
}

func (uc *GUIUseCase) Shutdown() {
	data, err := uc.cfg.Marshal()
	if err != nil {
		uc.log.Error().Err(err).Msg("failed to marshal client config")
		return
	}

	if err := uc.ipcClient.UpdateConfig(uc.ctx, data); err != nil {
		uc.log.Error().Err(err).Msg("failed to update client config")
		return
	}

	if !*uc.cfg.GUI.ShutdownClientOnExit {
		return
	}

	if err := uc.ipcClient.Shutdown(uc.ctx); err != nil {
		uc.log.Error().Err(err).Msg("failed to send shutdown")
	}
}

func (uc *GUIUseCase) GetAsyncChannel() chan *entity.GUIRequest {
	return uc.messageCh
}

func (uc *GUIUseCase) updateConfig() error {
	data, err := uc.cfg.Marshal()
	if err != nil {
		return err
	}

	if err := uc.ipcClient.UpdateConfig(uc.ctx, data); err != nil {
		return err
	}

	state, err := uc.ipcClient.GetState(uc.ctx)
	if err != nil {
		uc.log.Error().Err(err).Msg("failed to get client state")
	}

	uc.sendInitialization(uc.cfg, state, nil)

	return nil
}

func (uc *GUIUseCase) runClient() error {
	tmplVar := map[string]string{
		"client": filepath.Join(uc.homeDir, entity.GetClientBinaryName()),
		"config": filepath.Dir(uc.cfgHandler.GetPath()),
	}

	buf := bytes.NewBuffer(nil)
	tmpl, err := template.New("client-run").Parse(uc.cfg.ClientRunner.Get())
	if err != nil {
		return err
	}
	if err := tmpl.Execute(buf, tmplVar); err != nil {
		return err
	}

	//err = uc.cmd.RunAndWaitResponse(buf.String(), entity.ClientSuccessResponse, entity.ClientErrorResponse)
	err = uc.cmd.Start(buf.String())
	if err != nil {
		uc.log.Error().Str("cmd", buf.String()).Msg("failed to execute")
	} else {
		uc.log.Info().Str("cmd", buf.String()).Msg("client started")
	}

	return err
}

func (uc *GUIUseCase) statisticHandler(ch chan map[int]*entity.Statistic) {
	go func() {
		for {
			select {
			case stat, ok := <-ch:
				if !ok {
					return
				}
				uc.messageCh <- &entity.GUIRequest{
					Cmd:     entity.CmdStatistic,
					Payload: stat,
				}
			case <-uc.ctx.Done():
				return
			}
		}
	}()
}

func (uc *GUIUseCase) loggerHandler(ch chan []byte) {
	go func() {
		for {
			select {
			case ev, ok := <-ch:
				if !ok {
					return
				}
				uc.messageCh <- &entity.GUIRequest{
					Cmd:     entity.CmdLogger,
					Payload: string(ev),
				}
			case <-uc.ctx.Done():
				return
			}
		}
	}()
}
