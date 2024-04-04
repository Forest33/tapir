package rest

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/business/usecase"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
)

type Server struct {
	cfg           *Config
	log           *logger.Logger
	serverUseCase ServerUseCase
	router        *gin.Engine
}

type Config struct {
	Host string
	Port int
}

type ServerUseCase interface {
	GetSessions() map[uint32]*usecase.ServerSessionInfo
	GetInterfaces() map[string]*usecase.ServerInterfaceInfo
}

type connection struct {
	Protocol        string
	Local           string
	Remote          string
	SessionID       uint32
	CreatedAt       int64
	CompressionType entity.CompressionType
}

type interfaceInfo struct {
	ConnectionsCount int
	Connections      []*connection
	SessionID        uint32
}

type stateResponse struct {
	Sessions   map[uint32]*usecase.ServerSessionInfo
	Interfaces map[string]*interfaceInfo
}

func New(cfg *Config, log *logger.Logger, serverUseCase ServerUseCase) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		cfg:           cfg,
		log:           log,
		serverUseCase: serverUseCase,
		router:        gin.Default(),
	}

	return s, s.init()
}

func (s *Server) init() error {
	s.router.GET("/api/v1/state", s.handlerState)
	return nil
}

func (s *Server) Start() {
	go func() {
		s.log.Info().
			Str("host", s.cfg.Host).
			Int("port", s.cfg.Port).
			Msg("starting HTTP server")

		err := s.router.Run(fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port))
		if err != nil {
			s.log.Fatalf("failed to start HTTP server: %v", err)
		}
	}()
}

func (s *Server) handlerState(ctx *gin.Context) {
	ifInfo := s.serverUseCase.GetInterfaces()

	resp := &stateResponse{
		Sessions:   s.serverUseCase.GetSessions(),
		Interfaces: make(map[string]*interfaceInfo, len(ifInfo)),
	}

	for name, info := range ifInfo {
		resp.Interfaces[name] = &interfaceInfo{
			ConnectionsCount: len(info.Connections),
			Connections:      structs.Map(info.Connections, entityToConnection),
			SessionID:        info.SessionID,
		}
	}

	ctx.JSON(http.StatusOK, resp)
}

func entityToConnection(c *entity.Connection) *connection {
	var (
		protocol string
		local    string
		remote   string
	)

	if c.TCPConn != nil {
		protocol = "TCP"
		local = c.TCPConn.LocalAddr().String()
		remote = c.TCPConn.RemoteAddr().String()
	} else if c.UDPConn != nil {
		protocol = "UDP"
		local = c.UDPConn.LocalAddr().String()
		remote = c.Addr.String()
	}

	return &connection{
		Protocol:        protocol,
		Local:           local,
		Remote:          remote,
		SessionID:       c.SessionID,
		CreatedAt:       c.CreatedAt,
		CompressionType: c.CompressionType,
	}
}
