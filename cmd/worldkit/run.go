package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"gioui.org/app"
	"gioui.org/unit"
	"golang.org/x/sys/windows"

	"world-kit/internal/autostart"
	"world-kit/internal/capture"
	"world-kit/internal/chatlog"
	"world-kit/internal/diagnostics"
	"world-kit/internal/exports"
	"world-kit/internal/filedialog"
	"world-kit/internal/screenshot"
	"world-kit/internal/settings"
	"world-kit/internal/ui"
	"world-kit/internal/update"
)

const appName = "WorldKit"

// appVersion is this build's version, shown in Settings and compared with GitHub's
// releases. Release builds set it with -ldflags "-X main.appVersion=x.y.z"
// (.github/workflows/release.yml); "dev" never updates.
var appVersion = "dev"

// run wires WorldKit's services to a window and blocks until the window is closed.
// The returned error is also written to the diagnostics log, which main can't do: the log
// is closed by then.
func run(isMinimized bool) (runErr error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("find app data folder: %w", err)
	}
	dataDir := filepath.Join(configDir, appName)
	diagnosticsDir := filepath.Join(dataDir, "diagnostics")
	diagnosticsSession, err := diagnostics.Start(diagnosticsDir)
	if err != nil {
		return err
	}
	defer diagnosticsSession.End(&runErr)
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
	updates := newUpdateService(store, diagnosticsDir, window.Invalidate)
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	workers.Go(func() { captureService.Run(ctx) })
	workers.Go(func() { updates.Run(ctx) })
	defer func() {
		cancel()
		workers.Wait()
	}()

	services := newServices(captureService, store)
	services.Updates = updates
	services.Version = appVersion
	services.Logs = chatlog.NewLibrary(archiveDir)
	services.Files = filedialog.Dialog{}
	services.Screenshots = screenshot.NewStore(filepath.Join(dataDir, "screenshots"))
	services.Exports = exports.Open(filepath.Join(dataDir, "exports.json"))
	services.DefaultExportDir = defaultExportDir()
	return ui.Run(window, services)
}

// newUpdateService checks GitHub for newer releases: WorldKit's only network access.
func newUpdateService(store *settings.Store, diagnosticsDir string, onChange func()) *update.Service {
	return update.New(update.Options{
		CurrentVersion: appVersion,
		Client:         update.NewClient(update.ClientOptions{CurrentVersion: appVersion}),
		Settings:       store,
		DownloadDir:    filepath.Join(os.TempDir(), appName+"-update"),
		InstallLogPath: filepath.Join(diagnosticsDir, "update-install.log"),
		OnChange:       onChange,
	})
}

// defaultExportDir is Pictures\WorldKit, where exports go until Settings says otherwise.
func defaultExportDir() string {
	pictures, err := windows.KnownFolderPath(windows.FOLDERID_Pictures, 0)
	if err != nil {
		slog.Warn("find Pictures folder", "error", err)
		home, _ := os.UserHomeDir() // empty on error: the Save dialog then opens where it last was
		pictures = filepath.Join(home, "Pictures")
	}
	return filepath.Join(pictures, appName)
}

func newWindow(isMinimized bool) *app.Window {
	window := new(app.Window)
	window.Option(
		app.Title(appName),
		// WorldKit draws its own title bar (ui.titleBar); Windows still resizes and snaps it.
		app.Decorated(false),
		app.Size(unit.Dp(1280), unit.Dp(820)),
		app.MinSize(unit.Dp(900), unit.Dp(600)),
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
