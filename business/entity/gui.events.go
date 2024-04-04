// Package entity provides entities for business logic.
package entity

// UI events
const (
	CmdInitialization    GUICommand = "initialization"
	CmdStatistic         GUICommand = "statistic"
	CmdLogger            GUICommand = "logger"
	CmdConnectionConnect GUICommand = "connection.connect"
	CmdConnectionUpdate  GUICommand = "connection.update"
	CmdConnectionDelete  GUICommand = "connection.delete"
	CmdConnectionImport  GUICommand = "connection.import"
	CmdLogsGet           GUICommand = "logs.get"
	CmdDevTools          GUICommand = "dev.tools.show"
)

// GUICommand UI command
type GUICommand string

// String returns UI command string
func (c GUICommand) String() string {
	return string(c)
}

type GUIEvent string
