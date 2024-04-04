//go:build windows

package wiface

import (
	"fmt"
	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/wtun/tun"
	"github.com/pkg/errors"
	"regexp"
	"strings"
	"time"
)

func (i *Iface) Create(ifc *entity.Interface) (*entity.Interface, error) {
	i.Lock()
	defer i.Unlock()

	var err error
	ifc.Handler, err = func() (entity.InterfaceHandler, error) {
		if i.interfaceCreatorFunc == nil {
			return tun.CreateTUN(fmt.Sprintf("tapir-%d", time.Now().Unix()), "Tapir", i.cfg.Tunnel.MTU)
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
	out, err := i.cmd.Run(fmt.Sprintf("route print | findstr \"\\<0.0.0.0\\>\""))
	if err != nil {
		return errors.Wrap(err, out)
	}

	rxDefaultGateway := regexp.MustCompile(`0.0.0.0\s+0.0.0.0\s+([0-9.]+)`)
	match := rxDefaultGateway.FindStringSubmatch(strings.TrimSpace(out))
	if len(match) < 2 {
		return fmt.Errorf("failed to get default gateway (%s)", out)
	}
	i.defaultGatewayIP = match[1]

	out, err = i.cmd.Run(fmt.Sprintf("Get-NetAdapter | Sort-Object DeviceName | ft Name, IFIndex"))
	if err != nil {
		return errors.Wrap(err, out)
	}

	ifName, _ := info.Name()
	rxDeviceIndex := regexp.MustCompile(fmt.Sprintf(`(?:\r\n|\r|\n)%s\s+(\d+)(?:\r\n|\r|\n)`, ifName))
	match = rxDeviceIndex.FindStringSubmatch(out)
	if len(match) < 2 {
		return fmt.Errorf("failed to get device index (%s)", out)
	}
	i.deviceIndex = match[1]

	i.log.Info().
		Str("ip", i.defaultGatewayIP).
		Str("if", i.deviceIndex).
		Msg("gateway info")

	return i.interfaceStartupFunc(info, true)
}
