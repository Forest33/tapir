package entity

import (
	"os"

	"github.com/forest33/tapir/pkg/structs"
)

// UI response statuses
const (
	debugEnv = "TAPIR_DEBUG"
)

// GUIResponseStatus response status
type GUIResponseStatus string

// GUIRequest UI request
type GUIRequest struct {
	Cmd     GUICommand  `json:"name"`
	Payload interface{} `json:"payload"`
	Error   error       `json:"error,omitempty"`
}

// GUIResponse UI response
type GUIResponse struct {
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// IsDebug returns true if application runs on debug mode
func IsDebug() bool {
	return os.Getenv(debugEnv) != ""
}

type ConnectRequest struct {
	ConnID    int32 `json:"id"`
	IsConnect bool  `json:"connect"`
}

func (r *ConnectRequest) Model(payload map[string]interface{}) error {
	if payload == nil {
		return nil
	}

	if v, ok := payload["id"]; ok && v != nil {
		r.ConnID = int32(v.(float64))
	}
	if v, ok := payload["connect"]; ok && v != nil {
		r.IsConnect = v.(bool)
	}

	return nil
}

type UpdateConnectionRequest struct {
	ConnID     int32  `json:"id"`
	Name       string `json:"name"`
	ServerHost string `json:"serverHost"`
	PortMin    uint16 `json:"portMin"`
	PortMax    uint16 `json:"portMax"`
	UseTCP     *bool  `json:"useTCP"`
	UseUDP     *bool  `json:"useUDP"`
	Username   string `json:"username"`
	Password   string `json:"password"`
}

func (r *UpdateConnectionRequest) Model(payload map[string]interface{}) error {
	if payload == nil {
		return nil
	}

	if v, ok := payload["id"]; ok && v != nil {
		r.ConnID = int32(v.(float64))
	}
	if v, ok := payload["name"]; ok && v != nil {
		r.Name = v.(string)
	}
	if v, ok := payload["serverHost"]; ok && v != nil {
		r.ServerHost = v.(string)
	}
	if v, ok := payload["portMin"]; ok && v != nil {
		r.PortMin = uint16(v.(float64))
	}
	if v, ok := payload["portMax"]; ok && v != nil {
		r.PortMax = uint16(v.(float64))
	}
	if v, ok := payload["useTCP"]; ok && v != nil {
		r.UseTCP = structs.Ref(v.(bool))
	}
	if v, ok := payload["useUDP"]; ok && v != nil {
		r.UseUDP = structs.Ref(v.(bool))
	}
	if v, ok := payload["username"]; ok && v != nil {
		r.Username = v.(string)
	}
	if v, ok := payload["password"]; ok && v != nil {
		r.Password = v.(string)
	}

	return nil
}

type DeleteConnectionRequest struct {
	ConnID int32 `json:"id"`
}

func (r *DeleteConnectionRequest) Model(payload map[string]interface{}) error {
	if payload == nil {
		return nil
	}

	if v, ok := payload["id"]; ok && v != nil {
		r.ConnID = int32(v.(float64))
	}

	return nil
}

type ImportConnectionRequest struct {
	File string `json:"file"`
}

func (r *ImportConnectionRequest) Model(payload map[string]interface{}) error {
	if payload == nil {
		return nil
	}

	if v, ok := payload["file"]; ok && v != nil {
		r.File = v.(string)
	}

	return nil
}
