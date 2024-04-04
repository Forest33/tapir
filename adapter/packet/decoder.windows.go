//go:build windows

package packet

import "github.com/forest33/tapir/business/entity"

func decodeLayers(data []byte, pi *entity.NetworkPacketInfo) (*entity.NetworkPacketInfo, error) {
	return pi, nil
}
