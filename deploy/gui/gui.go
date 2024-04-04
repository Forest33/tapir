package main

import (
	"github.com/asticode/go-astikit"
	"github.com/asticode/go-astilectron"

	"github.com/forest33/tapir/business/entity"
	"github.com/forest33/tapir/pkg/resources"
)

const (
	defaultMinWindowWidth     = 400
	defaultMinWindowHeight    = 400
	defaultVersionAstilectron = "0.56.0"
	defaultVersionElectron    = "13.6.9"
)

func getAstilectronOptions() astilectron.Options {
	if VersionAstilectron == "" {
		VersionAstilectron = defaultVersionAstilectron
	}
	if VersionElectron == "" {
		VersionElectron = defaultVersionElectron
	}

	iconPath := resources.GetApplicationIcon()

	zlog.Debug().
		Str("path", iconPath).
		Msgf("application icon")

	return astilectron.Options{
		AppName:            applicationName,
		AppIconDarwinPath:  iconPath,
		AppIconDefaultPath: iconPath,
		BaseDirectoryPath:  homeDir,
		VersionAstilectron: VersionAstilectron,
		VersionElectron:    VersionElectron,
	}
}

func getWindowOptions() *astilectron.WindowOptions {
	return &astilectron.WindowOptions{
		Center:         astikit.BoolPtr(true),
		Frame:          astikit.BoolPtr(true),
		Show:           astikit.BoolPtr(true),
		Width:          astikit.IntPtr(cfg.GUI.WindowWidth),
		Height:         astikit.IntPtr(cfg.GUI.WindowHeight),
		MinWidth:       astikit.IntPtr(defaultMinWindowWidth),
		MinHeight:      astikit.IntPtr(defaultMinWindowHeight),
		UseContentSize: astikit.BoolPtr(true),
		X:              astikit.IntPtr(cfg.GUI.WindowX),
		Y:              astikit.IntPtr(cfg.GUI.WindowY),
		Title:          astikit.StrPtr(applicationName),
		Custom: &astilectron.WindowCustomOptions{
			HideOnClose: astikit.BoolPtr(!entity.IsDebug()),
		},
		WebPreferences: &astilectron.WebPreferences{
			NodeIntegrationInWorker: astikit.BoolPtr(true),
			EnableRemoteModule:      astikit.BoolPtr(true),
		},
	}
}

func getTrayOptions() *astilectron.TrayOptions {
	iconPath := resources.GetTrayIcon()

	zlog.Debug().Str("path", iconPath).Msgf("tray icon")

	return &astilectron.TrayOptions{
		Image:   astikit.StrPtr(iconPath),
		Tooltip: astikit.StrPtr(applicationName),
	}
}

func getTrayMenuOptions() []*astilectron.MenuItemOptions {
	return []*astilectron.MenuItemOptions{
		{
			Label: astikit.StrPtr("Show"),
			OnClick: func(e astilectron.Event) (deleteListener bool) {
				_ = window.Show()
				return
			},
		},
		{
			Label: astikit.StrPtr("Exit"),
			Role:  astilectron.MenuItemRoleQuit,
		},
	}
}

func initGUIEvents() {
	window.On(astilectron.EventNameWindowEventMove, func(e astilectron.Event) bool {
		cfg.GUI.WindowX = *e.Bounds.X
		cfg.GUI.WindowY = *e.Bounds.Y
		return false
	})
	window.On(astilectron.EventNameWindowEventResize, func(e astilectron.Event) bool {
		cfg.GUI.WindowWidth = *e.Bounds.Width
		cfg.GUI.WindowHeight = *e.Bounds.Height
		return false
	})
}

func initAsyncMessages() {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-guiUseCase.GetAsyncChannel():
				err := window.SendMessage(msg, func(_ *astilectron.EventMessage) {})
				if err != nil {
					zlog.Error().Msgf("failed to send GUI message: %v", err)
				}
			}
		}
	}()
}
