//go:build darwin

package wiface

import (
	"bufio"
	"fmt"
	"strings"

	"github.com/forest33/wtun/tun"
	"github.com/pkg/errors"

	"github.com/forest33/tapir/business/entity"
)

func (i *Iface) Create(ifc *entity.Interface) (*entity.Interface, error) {
	i.Lock()
	defer i.Unlock()

	var err error
	ifc.Handler, err = func() (entity.InterfaceHandler, error) {
		if i.interfaceCreatorFunc == nil {
			return tun.CreateTUN("utun", i.cfg.Tunnel.MTU)
		}
		return i.interfaceCreatorFunc(i.cfg.Tunnel.MTU)
	}()
	if err != nil {
		return nil, err
	}

	if err := i.up(ifc); err != nil {
		return nil, err
	}

	i.listen(ifc)

	return ifc, nil
}

func (i *Iface) up(info *entity.Interface) error {
	out, err := i.cmd.Run(fmt.Sprintf("route -n get default"))
	if err != nil {
		return errors.Wrap(err, out)
	}

	scanner := bufio.NewScanner(strings.NewReader(out))
	for scanner.Scan() {
		l := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(l, "gateway:") {
			i.defaultGatewayIP = strings.TrimSpace(strings.ReplaceAll(l, "gateway:", ""))
		} else if strings.HasPrefix(l, "interface:") {
			i.defaultGatewayDev = strings.TrimSpace(strings.ReplaceAll(l, "interface:", ""))
		}
		if i.defaultGatewayIP != "" && i.defaultGatewayDev != "" {
			break
		}
	}

	i.log.Info().
		Str("ip", i.defaultGatewayIP).
		Str("device", i.defaultGatewayDev).
		Msg("gateway info")

	return i.interfaceStartupFunc(info, true)
}
