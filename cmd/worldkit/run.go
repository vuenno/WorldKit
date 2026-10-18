package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"gioui.org/app"
	"gioui.org/unit"

	"world-kit/internal/autostart"
	"world-kit/internal/capture"
	"world-kit/internal/chatlog"
	"world-kit/internal/settings"
	"world-kit/internal/ui"
)

const appName = "WorldKit"

// run wires WorldKit's services to a window and blocks until the window is closed.
func run(isMinimized bool) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("find app data folder: %w", err)
	}
	dataDir := filepath.Join(configDir, appName)
	store, err := settings.Open(filepath.Join(dataDir, "settings.json"))
	if err != nil {
		return err
	}

	window := newWindow(isMinimized)
	archiveDir := filepath.Join(dataDir, "logs")
	captureService := capture.New(capture.Options{
		ArchiveDir: archiveDir,
		DataDir:    store.Get().FiveMDataDir,
		OnChange:   window.Invalidate,
	})
	ctx, cancel := context.WithCancel(context.Background())
	captureStopped := make(chan struct{})
	go func() {
		captureService.Run(ctx)
		close(captureStopped)
	}()
	defer func() {
		cancel()
		<-captureStopped
	}()

	services := newServices(captureService, store)
	services.Logs = chatlog.NewLibrary(archiveDir)
	return ui.Run(window, services)
}

func newWindow(isMinimized bool) *app.Window {
	window := new(app.Window)
	window.Option(
		app.Title(appName),
		app.Size(unit.Dp(960), unit.Dp(600)),
		app.MinSize(unit.Dp(640), unit.Dp(400)),
	)
	if isMinimized {
		window.Option(app.Minimized.Option())
	}
	return window
}

func newServices(captureService *capture.Service, store *settings.Store) ui.Services {
	services := ui.Services{Capture: captureService, Settings: store}
	entry, err := autostart.NewEntry(appName)
	if err != nil {
		slog.Warn("start with Windows unavailable", "error", err)
		return services
	}
	services.Startup = entry
	return services
}
