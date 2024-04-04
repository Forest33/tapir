package grpc_server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"

	apiV1 "github.com/forest33/tapir/api/client/v1"
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
)

type Server struct {
	cfg     *entity.ClientConfig
	log     *logger.Logger
	handler ipcHandler
}

type ipcHandler interface {
	Connect(connID int) error
	Disconnect(connID int) error
	GetConnections() ([]*entity.ConnectionInfo, error)
	SetStatisticChannel(ch chan map[int]*entity.Statistic)
	UpdateConfig(jsonData []byte) error
	Shutdown()
}

func New(cfg *entity.ClientConfig, log *logger.Logger, handler ipcHandler) (*Server, error) {
	srv := &Server{
		cfg:     cfg,
		log:     log,
		handler: handler,
	}

	s := grpc.NewServer()
	reflection.Register(s)
	apiV1.RegisterClientServer(s, srv)
	lst, err := net.Listen("tcp", fmt.Sprintf("%s:%d", cfg.IPC.GrpcHost, cfg.IPC.GrpcPort))
	if err != nil {
		return nil, err
	}

	log.Info().
		Str("host", cfg.IPC.GrpcHost).
		Int("port", cfg.IPC.GrpcPort).
		Msg("starting IPC server")

	go func() {
		if err := s.Serve(lst); err != nil {
			log.Fatalf("failed to start gRPC server: %v", err)
		}
	}()

	return srv, nil
}

func (s *Server) Start(ctx context.Context, req *apiV1.RequestById) (*apiV1.State, error) {
	err := s.handler.Connect(int(req.Id))
	if errors.Is(err, entity.ErrMaxConnectionAttempts) {
		return nil, status.Error(codes.DeadlineExceeded, err.Error())
	} else if errors.Is(err, entity.ErrUnauthorized) {
		return nil, status.Error(codes.Unauthenticated, err.Error())
	} else if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	state, _ := s.GetState(ctx, &empty.Empty{})
	return state, nil
}

func (s *Server) Stop(ctx context.Context, req *apiV1.RequestById) (*apiV1.State, error) {
	err := s.handler.Disconnect(int(req.Id))
	state, _ := s.GetState(ctx, &empty.Empty{})
	return state, err
}

func (s *Server) GetState(_ context.Context, _ *empty.Empty) (*apiV1.State, error) {
	connections, err := s.handler.GetConnections()
	if err != nil {
		return nil, err
	}
	return &apiV1.State{Connections: structs.Map(connections, entityToConnection)}, nil
}

func (s *Server) StatisticStream(_ *empty.Empty, stream apiV1.Client_StatisticStreamServer) error {
	statCh := make(chan map[int]*entity.Statistic)
	s.handler.SetStatisticChannel(statCh)
	defer s.handler.SetStatisticChannel(nil)

	for stream.Context().Err() == nil {
		stat, ok := <-statCh
		if !ok {
			break
		}

		data, err := json.Marshal(stat)
		if err != nil {
			s.log.Error().Err(err).Msg("failed to serialize statistic message")
			return err
		}

		err = stream.SendMsg(&apiV1.JsonData{Data: data})
		if err != nil && status.Code(err) != codes.Canceled {
			s.log.Error().Err(err).Msgf("failed to send message")
			return err
		}
	}
	return nil
}

func (s *Server) UpdateConfig(_ context.Context, r *apiV1.Config) (*empty.Empty, error) {
	return &empty.Empty{}, s.handler.UpdateConfig(r.GetJsonData())
}

func (s *Server) Shutdown(_ context.Context, _ *empty.Empty) (*empty.Empty, error) {
	s.handler.Shutdown()
	return &empty.Empty{}, nil
}

func entityToConnection(c *entity.ConnectionInfo) *apiV1.Connection {
	return &apiV1.Connection{
		Id:          int32(c.ID),
		IsConnected: c.IsConnected,
		ConnectTs:   c.ConnectTs,
	}
}
