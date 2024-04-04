package entity

type CommandExecutor interface {
	Run(command string) (string, error)
	Start(command string) error
	RunAndWaitResponse(command, successResponse, errorResponse string) error
}
