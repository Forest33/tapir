package client

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/codec"
	"github.com/forest33/tapir/pkg/logger"
)

type Client struct {
	ctx                 context.Context
	log                 *logger.Logger
	originalLog         *logger.Logger
	cfg                 *Config
	receiver            entity.ReceiverHandler
	disconnect          entity.DisconnectHandler
	getUserEncryptor    entity.EncryptorGetter
	retryFactory        entity.NetworkRetryFactory
	ackFactory          entity.NetworkAckFactory
	packetDecoder       entity.PacketDecoder
	addSessionStatistic entity.StatisticHandler
}

type Config struct {
	Codec             codec.Codec
	PrimaryEncryptor  entity.Encryptor
	MTU               int
	ReadBufferSize    int
	WriteBufferSize   int
	MultipathTCP      bool
	KeepaliveInterval time.Duration
	SocketTracing     bool
}

func (c Config) validate() error {
	// TODO
	return nil
}

func New(ctx context.Context, log *logger.Logger, cfg *Config, retryFactory entity.NetworkRetryFactory, ackFactory entity.NetworkAckFactory, packetDecoder entity.PacketDecoder) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &Client{
		ctx:           ctx,
		log:           log.Duplicate(log.With().Str("layer", "cli").Logger()),
		originalLog:   log,
		cfg:           cfg,
		retryFactory:  retryFactory,
		ackFactory:    ackFactory,
		packetDecoder: packetDecoder,
	}, nil
}

func (c *Client) Run(host string, port uint16, proto entity.Protocol) (*entity.Connection, error) {
	switch proto {
	case entity.ProtoTCP:
		addr, err := net.ResolveTCPAddr(proto.String(), fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return nil, err
		}

		d := &net.Dialer{}
		d.SetMultipathTCP(c.cfg.MultipathTCP)
		cn, err := net.Dial(proto.String(), addr.String())
		if err != nil {
			return nil, err
		}
		conn := cn.(*net.TCPConn)

		if c.cfg.ReadBufferSize > 0 {
			if err := conn.SetReadBuffer(c.cfg.ReadBufferSize); err != nil {
				return nil, err
			}
		}
		if c.cfg.WriteBufferSize > 0 {
			if err := conn.SetWriteBuffer(c.cfg.WriteBufferSize); err != nil {
				return nil, err
			}
		}
		if c.cfg.KeepaliveInterval != 0 {
			if err := conn.SetKeepAlive(true); err != nil {
				return nil, err
			}
			if err := conn.SetKeepAlivePeriod(c.cfg.KeepaliveInterval); err != nil {
				return nil, err
			}
		}

		isMultipathTCP, _ := conn.MultipathTCP()
		c.log.Info().
			Str("addr", conn.RemoteAddr().String()).
			Bool("mptcp", isMultipathTCP).
			Msgf("connection established")

		return &entity.Connection{
			TCPConn: conn,
			Port:    port,
			Addr:    conn.RemoteAddr(),
			Proto:   entity.ProtoTCP,
		}, nil
	case entity.ProtoUDP:
		addr, err := net.ResolveUDPAddr(proto.String(), fmt.Sprintf("%s:%d", host, port))
		if err != nil {
			return nil, err
		}

		conn, err := net.DialUDP(proto.String(), nil, addr)
		if err != nil {
			return nil, err
		}

		if c.cfg.ReadBufferSize > 0 {
			if err := conn.SetReadBuffer(c.cfg.ReadBufferSize); err != nil {
				return nil, err
			}
		}
		if c.cfg.WriteBufferSize > 0 {
			if err := conn.SetWriteBuffer(c.cfg.WriteBufferSize); err != nil {
				return nil, err
			}
		}

		return &entity.Connection{
			UDPConn: conn,
			Addr:    addr,
			Port:    port,
			Proto:   entity.ProtoUDP,
		}, nil
	}

	return nil, fmt.Errorf("unknown protocol %s", proto)
}

func (c *Client) SendAsync(msg *entity.Message, conn *entity.Connection) error {
	var (
		userEncryptor entity.Encryptor
		err           error
	)

	if msg.IsUserData() {
		userEncryptor, err = c.getUserEncryptor(conn)
		if err != nil {
			c.log.Error().Err(err).Uint32("session_id", conn.SessionID).Msg("failed to get connection encryptor")
			return err
		}
	}

	if conn.TCPConn != nil {
		return c.sendAsyncTCP(msg, userEncryptor, conn.TCPConn)
	}

	return c.sendAsyncUDP(msg, userEncryptor, conn, conn.Retry)
}

func (c *Client) SendSync(msg *entity.Message, conn *entity.Connection, timeout time.Duration) (*entity.Message, *entity.Connection, error) {
	if conn.TCPConn != nil {
		return c.sendSyncTCP(msg, conn, timeout)
	}

	return c.sendSyncUDP(msg, conn, timeout)
}

func (c *Client) SetReceiverHandler(f entity.ReceiverHandler) {
	c.receiver = f
}

func (c *Client) SetDisconnectHandler(f entity.DisconnectHandler) {
	c.disconnect = f
}

func (c *Client) SetEncryptorGetter(f entity.EncryptorGetter) {
	c.getUserEncryptor = f
}

func (c *Client) SetStatisticHandler(f entity.StatisticHandler) {
	c.addSessionStatistic = f
}
