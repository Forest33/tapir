//go:build arm64

package packet

import "github.com/forest33/tapir/business/entity"

func decodeLayers(_ []byte, pi *entity.NetworkPacketInfo) (*entity.NetworkPacketInfo, error) {
	return pi, nil
}
