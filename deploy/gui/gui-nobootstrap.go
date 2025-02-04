package main

import (
	"fmt"

	"github.com/asticode/go-astilectron"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/resources"
)

func createWindow() error {
	iconPath := resources.GetApplicationIcon()
	var err error

	zlog.Debug().Str("path", iconPath).Msg("application icon")

	ast, err = astilectron.New(zlog, getAstilectronOptions())
	if err != nil {
		return fmt.Errorf("creating astilectron failed: %v", err)
	}

	ast.HandleSignals()

	if err = ast.Start(); err != nil {
		return fmt.Errorf("starting astilectron failed: %v", err)
	}

	homepage := resources.GetHomepage()
	zlog.Debug().Str("path", homepage).Msg("homepage")

	if window, err = ast.NewWindow(homepage, getWindowOptions()); err != nil {
		return fmt.Errorf("new window failed: %v", err)
	}

	if err = window.Create(); err != nil {
		return fmt.Errorf("creating window failed: %v", err)
	}

	if entity.IsDebug() {
		if err := window.OpenDevTools(); err != nil {
			zlog.Error().Err(err).Msg("failed to open dev tools")
		}
	}

	return nil
}

func createTray() error {
	tray = ast.NewTray(getTrayOptions())

	if err := tray.Create(); err != nil {
		return err
	}

	var m = tray.NewMenu(getTrayMenuOptions())

	return m.Create()
}

type noBootstrapResponse struct {
	Payload interface{} `json:"payload"`
}

func noBootstrapMessageHandler() {
	window.OnMessage(func(m *astilectron.EventMessage) interface{} {
		req := &entity.GUIRequest{}
		if err := m.Unmarshal(&req); err != nil {
			zlog.Error().Msgf("failed to unmarshal GUI event: %v", err)
			return nil
		}
		resp := eventsHandler(req)
		return &noBootstrapResponse{
			Payload: &bootstrapResponse{
				Data:  resp.Payload,
				Error: resp.Error,
			},
		}
	})
}

func withoutBootstrap() {
	zlog.Info().
		Bool("debug", entity.IsDebug()).
		Msg("UI not using bootstrap")

	if err := createWindow(); err != nil {
		zlog.Fatalf("error creating UI: %v", err)
	}
	defer ast.Close()

	noBootstrapMessageHandler()

	if err := createTray(); err != nil {
		zlog.Fatalf("error creating tray: %v", err)
	}

	initGUIEvents()
	initAsyncMessages()

	ast.Wait()
}
