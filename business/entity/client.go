package entity

import (
	"runtime"
	"time"
)

const (
	ClientLogFile = "client.log"
)

type NetworkClient interface {
	Run(host string, port uint16, proto Protocol) (*Connection, error)
	ReceiverTCP(conn *Connection, sessionID uint32)
	ReceiverUDP(conn *Connection, sessionID uint32)
	SendAsync(msg *Message, conn *Connection) error
	SendSync(msg *Message, conn *Connection, timeout time.Duration) (*Message, *Connection, error)
	SetReceiverHandler(f ReceiverHandler)
	SetDisconnectHandler(f DisconnectHandler)
	SetEncryptorGetter(f EncryptorGetter)
	SetStatisticHandler(f StatisticHandler)
}

type ClientConnectionHandler interface {
	Start() error
	Exit()
}

func GetClientBinaryName() string {
	if runtime.GOOS == "windows" {
		return "client.exe"
	}
	return "client"
}
