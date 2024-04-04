package grpc_client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	apiV1 "github.com/forest33/tapir/api/client/v1"
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
)

const (
	requestTimeout = time.Minute
)

type Client struct {
	cfg    *entity.ClientIPCSettings
	log    *logger.Logger
	client apiV1.ClientClient
}

func New(cfg *entity.ClientIPCSettings, log *logger.Logger) (*Client, error) {
	c := &Client{
		cfg: cfg,
		log: log,
	}

	conn, err := grpc.Dial(fmt.Sprintf("%s:%d", cfg.GrpcHost, cfg.GrpcPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	c.client = apiV1.NewClientClient(conn)

	return c, nil
}

func (c *Client) Start(ctx context.Context, connID int32) (*entity.ClientState, error) {
	ctx, cancel := c.getContext(ctx)
	defer cancel()

	state, err := c.client.Start(ctx, &apiV1.RequestById{Id: connID})
	if status.Code(err) == codes.Unavailable {
		return nil, entity.ErrGrpcServerUnavailable
	} else if status.Code(err) == codes.DeadlineExceeded {
		return nil, entity.ErrMaxConnectionAttempts
	} else if status.Code(err) == codes.Unauthenticated {
		return nil, entity.ErrUnauthorized
	} else if err != nil {
		return nil, err
	}

	return &entity.ClientState{
		Connections: structs.Map(state.Connections, connectionToEntity),
	}, nil
}

func (c *Client) Stop(ctx context.Context, connID int32) (*entity.ClientState, error) {
	ctx, cancel := c.getContext(ctx)
	defer cancel()

	state, err := c.client.Stop(ctx, &apiV1.RequestById{Id: connID})
	if status.Code(err) == codes.Unavailable {
		return nil, entity.ErrGrpcServerUnavailable
	} else if err != nil {
		return nil, err
	}

	return &entity.ClientState{
		Connections: structs.Map(state.Connections, connectionToEntity),
	}, nil
}

func (c *Client) GetState(ctx context.Context) (*entity.ClientState, error) {
	ctx, cancel := c.getContext(ctx)
	defer cancel()

	state, err := c.client.GetState(ctx, &empty.Empty{})
	if status.Code(err) == codes.Unavailable {
		return nil, entity.ErrGrpcServerUnavailable
	} else if err != nil {
		return nil, err
	}

	return &entity.ClientState{
		Connections: structs.Map(state.Connections, connectionToEntity),
	}, nil
}

func (c *Client) UpdateConfig(ctx context.Context, jsonData []byte) error {
	ctx, cancel := c.getContext(ctx)
	defer cancel()

	_, err := c.client.UpdateConfig(ctx, &apiV1.Config{JsonData: jsonData})

	return err
}

func (c *Client) Shutdown(ctx context.Context) error {
	ctx, cancel := c.getContext(ctx)
	defer cancel()

	_, err := c.client.Shutdown(ctx, &empty.Empty{})

	return err
}

func (c *Client) StatisticStream(ctx context.Context) (chan map[int]*entity.Statistic, error) {
	resp, err := c.client.StatisticStream(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}

	ch := make(chan map[int]*entity.Statistic)

	go func() {
		defer close(ch)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				stream, err := resp.Recv()
				if err == io.EOF {
					return
				} else if err != nil {
					c.log.Error().Err(err).Msg("failed to receive statistic")
					return
				}
				stat := map[int]*entity.Statistic{}
				if err = json.Unmarshal(stream.GetData(), &stat); err != nil {
					c.log.Error().Err(err).Msg("failed to unmarshal statistic")
					return
				}
				ch <- stat
			}
		}
	}()

	return ch, nil
}

func (c *Client) getContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, requestTimeout)
}

func connectionToEntity(c *apiV1.Connection) *entity.ConnectionInfo {
	return &entity.ConnectionInfo{
		ID:          int(c.Id),
		IsConnected: c.IsConnected,
		ConnectTs:   c.ConnectTs,
	}
}
