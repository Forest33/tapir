package entity

type ClientState struct {
	Connections []*ConnectionInfo
}

type ConnectionInfo struct {
	ID          int
	Handler     ClientConnectionHandler
	IsConnected bool
	ConnectTs   int64
}
