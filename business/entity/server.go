package entity

type NetworkServer interface {
	Run(host string, port uint16, proto Protocol) error
	Send(msg *Message, conn *Connection) error
	SetReceiverHandler(f ReceiverHandler)
	SetDisconnectHandler(f DisconnectHandler)
	SetEncryptorGetter(f EncryptorGetter)
	SetStatisticHandler(f StatisticHandler)
	DropSession(sessionID uint32)
}
