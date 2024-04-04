package wiface

import (
	"bytes"
	"fmt"
	"io/fs"
	"net"
	"runtime"
	"sync"
	"text/template"
	"time"

	"github.com/pkg/errors"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/logger"
	"github.com/forest33/tapir/pkg/structs"
)

type Iface struct {
	log                  *logger.Logger
	cfg                  *Config
	cmd                  entity.CommandExecutor
	packetDecoder        entity.PacketDecoder
	interfaceCreatorFunc interfaceCreatorFunc
	interfaceStartupFunc interfaceStartupFunc
	defaultGatewayIP     string
	defaultGatewayDev    string
	deviceIndex          string
	seqPool              *endpointSequencePool
	sync.Mutex
}

type Config struct {
	Tracing              bool
	ServerHost           string
	Tunnel               *entity.TunnelConfig
	InterfaceCreatorFunc interfaceCreatorFunc
	InterfaceStartupFunc interfaceStartupFunc
	EndpointTTL          int64
}

type interfaceCreatorFunc func(int) (entity.InterfaceHandler, error)
type interfaceStartupFunc func(info *entity.Interface, isUp bool) error

type endpointSequence struct {
	id uint32
	ts int64
}

const (
	initialEndpoints   = 100
	incrementEndpoints = 10
)

func New(log *logger.Logger, cfg *Config, cmd entity.CommandExecutor, decoder entity.PacketDecoder) (entity.InterfaceAdapter, error) {
	ifc := &Iface{
		log:                  log.Duplicate(log.With().Str("layer", "if").Logger()),
		cfg:                  cfg,
		cmd:                  cmd,
		packetDecoder:        decoder,
		interfaceCreatorFunc: cfg.InterfaceCreatorFunc,
		seqPool:              newEndpointSequencePool(),
	}

	ifc.interfaceStartupFunc = structs.If(cfg.InterfaceStartupFunc == nil, ifc.startup, cfg.InterfaceStartupFunc)

	if cfg.ServerHost != "" {
		ips, err := net.LookupIP(cfg.ServerHost)
		if err != nil {
			return nil, err
		} else if len(ips) == 0 {
			return nil, fmt.Errorf("failed to resolve server address %s", cfg.ServerHost)
		}
		cfg.ServerHost = ips[0].String()
	}

	return ifc, nil
}

func (i *Iface) listen(ifc *entity.Interface) {
	ifName, _ := ifc.Name()
	i.log.Debug().Str("device", ifName).Msg("listening network interface")

	go func() {
		defer func() {
			if err := i.Close(ifc); err != nil {
				i.log.Error().Err(err).Str("device", ifName).Msg("interface restore error")
			}
		}()

		var (
			endpointsPool   = newEndpointSequencePool()
			endpointsMap    = make(map[entity.PacketEndpoint]*endpointSequence, initialEndpoints)
			maxEndpoints    = initialEndpoints
			endpointsMapLen int
		)

		getMessageID := func(endpointID entity.PacketEndpoint) uint32 {
			if _, ok := endpointsMap[endpointID]; !ok {
				endpointsMap[endpointID] = endpointsPool.get()
				endpointsMapLen++
			}
			endpointsMap[endpointID].id++
			endpointsMap[endpointID].ts = time.Now().Unix()
			return endpointsMap[endpointID].id
		}

		buf := make([]byte, i.cfg.Tunnel.MTU)
		for {
			n, err := ifc.Handler.Read(buf)
			if err != nil {
				if !isInterfaceClosed(err) {
					i.log.Error().Err(err).Str("device", ifName).Msg("interface read error")
				}
				return
			}

			msg := entity.MessagePool.Get(n)
			//msg := &entity.Message{}
			//msg.Payload = make([]byte, n)

			msg.PacketInfo, err = i.packetDecoder.Decode(buf[:n])
			if err != nil {
				continue
			}

			msg.ID = getMessageID(msg.GetEndpoint())
			copy(msg.Payload.([]byte), buf[:n])
			msg.PayloadLength = uint16(n)
			msg.PacketInfo.IfName = ifName

			ifc.Receiver <- msg

			if i.cfg.EndpointTTL > 0 && endpointsMapLen >= maxEndpoints {
				curTs := time.Now().Unix()
				deleted := 0
				for e, es := range endpointsMap {
					if curTs > es.ts+i.cfg.EndpointTTL {
						delete(endpointsMap, e)
						endpointsPool.put(es)
						deleted++
					}
				}
				endpointsMapLen -= deleted
				if deleted == 0 {
					maxEndpoints += incrementEndpoints
				} else if maxEndpoints-deleted <= initialEndpoints {
					maxEndpoints = initialEndpoints
				} else if maxEndpoints-incrementEndpoints >= initialEndpoints {
					maxEndpoints -= incrementEndpoints
				} else {
					maxEndpoints--
				}
			}
		}
	}()
}

func (i *Iface) Write(ifc *entity.Interface, data interface{}) error {
	_, err := ifc.Handler.Write(data.([]byte))
	return err
}

func (i *Iface) startup(info *entity.Interface, isUp bool) error {
	commands, ok := i.cfg.Tunnel.InterfaceUp[runtime.GOOS]
	if !isUp {
		commands, ok = i.cfg.Tunnel.InterfaceDown[runtime.GOOS]
	}
	if !ok {
		return fmt.Errorf("failed to get network startup configuration for OS %s", runtime.GOOS)
	}

	if len(commands) == 0 {
		return nil
	}

	ifName, _ := info.Handler.Name()
	tmplVar := map[string]string{
		"mtu":                     fmt.Sprintf("%d", i.cfg.Tunnel.MTU),
		"client_tunnel_local_ip":  info.IP.ClientLocal.String(),
		"client_tunnel_remote_ip": info.IP.ClientRemote.String(),
		"server_tunnel_local_ip":  info.IP.ServerLocal.String(),
		"server_tunnel_remote_ip": info.IP.ServerRemote.String(),
		"tunnel_dev":              ifName,
		"gateway_ip":              i.defaultGatewayIP,
		"gateway_dev":             i.defaultGatewayDev,
		"server_ip":               i.cfg.ServerHost,
		"tunnel_index":            i.deviceIndex,
	}

	logMsg := structs.If(isUp, "interface up", "interface down")
	buf := bytes.NewBuffer(nil)

	for _, c := range commands {
		tmpl, err := template.New(fmt.Sprintf("startup-%t", isUp)).Parse(c)
		if err != nil {
			return err
		}
		if err := tmpl.Execute(buf, tmplVar); err != nil {
			return err
		}

		_, err = i.cmd.Run(buf.String())
		if err != nil {
			i.log.Error().Str("cmd", buf.String()).Msg("failed to execute")
		} else {
			i.log.Info().Str("cmd", buf.String()).Msg(logMsg)
		}

		buf.Reset()
	}

	return nil
}

func (i *Iface) Close(info *entity.Interface) error {
	i.Lock()
	defer i.Unlock()

	if info.Handler == nil {
		return nil
	}

	err := i.startup(info, false)
	close(info.Receiver)
	return err
}

func isInterfaceClosed(err error) bool {
	if fsErr, ok := err.(*fs.PathError); ok {
		if errors.Is(fsErr, fs.ErrClosed) {
			return true
		}
	}
	return false
}
