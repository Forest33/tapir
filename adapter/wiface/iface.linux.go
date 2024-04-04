//go:build linux

package wiface

import (
	"fmt"
	"regexp"

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
			return tun.CreateTUN("", i.cfg.Tunnel.MTU, 0)
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
	out, err := i.cmd.Run(fmt.Sprintf("ip route show default"))
	if err != nil {
		return errors.Wrap(err, out)
	}

	r := regexp.MustCompile(`default via ([\d.]+) dev ([a-zA-Z0-9]+)`)
	match := r.FindStringSubmatch(out)
	if len(match) < 3 {
		return fmt.Errorf("failed to get default gateway (%s)", out)
	}

	i.defaultGatewayIP = match[1]
	i.defaultGatewayDev = match[2]

	i.log.Info().
		Str("ip", i.defaultGatewayIP).
		Str("device", i.defaultGatewayDev).
		Msg("gateway info")

	return i.interfaceStartupFunc(info, true)
}
