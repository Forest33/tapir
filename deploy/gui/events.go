package main

import (
	"github.com/forest33/tapir/business/entity"
)

func eventsHandler(r *entity.GUIRequest) *entity.GUIResponse {
	resp := &entity.GUIResponse{}

	zlog.Debug().
		Str("cmd", r.Cmd.String()).
		Interface("payload", r.Payload).
		Msg("event")

	var payload map[string]interface{}
	if r.Payload != nil {
		payload = r.Payload.(map[string]interface{})
	}

	switch r.Cmd {
	case entity.CmdDevTools:
		_ = window.OpenDevTools()
	case entity.CmdConnectionConnect:
		resp = guiUseCase.CommandConnect(payload)
	case entity.CmdConnectionUpdate:
		resp = guiUseCase.CommandConnectionUpdate(payload)
	case entity.CmdConnectionDelete:
		resp = guiUseCase.CommandConnectionDelete(payload)
	case entity.CmdConnectionImport:
		resp = guiUseCase.CommandConnectionImport(payload)
	case entity.CmdLogsGet:
		resp = guiUseCase.CommandLogsGet()
	default:

	}

	return resp
}
