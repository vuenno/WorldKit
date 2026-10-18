// Package diagnostics saves what WorldKit logs and how it crashed to files, because the
// release build has no console: without this, log lines and panic traces are lost.
package diagnostics

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

const (
	appLogName      = "app.log"
	previousLogName = "app.previous.log" // the run before this one, kept so a crash's lead-up survives a restart
	crashNameLayout = "crash-2006-01-02_150405.log"
	crashGlob       = "crash-*.log"
)

// Session is one run's log and crash files. Call Close when the app exits normally.
type Session struct {
	appLog    *os.File
	crashPath string
}

// Start sends slog output to dir\app.log and fatal crash output (unrecovered panics in any
// goroutine, runtime fatal errors) to dir\crash-<start time>.log. A clean exit removes its
// empty crash file, so any crash file left behind marks a run that didn't exit cleanly; an
// empty one means it died without Go getting to write anything (e.g. killed by Windows).
func Start(dir string) (*Session, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create diagnostics folder %q: %w", dir, err)
	}
	leftovers, _ := filepath.Glob(filepath.Join(dir, crashGlob)) // only a bad pattern fails; crashGlob is constant
	appLog, err := openAppLog(dir)
	if err != nil {
		return nil, err
	}
	crashPath := filepath.Join(dir, time.Now().UTC().Format(crashNameLayout))
	if err := setCrashOutput(crashPath); err != nil {
		appLog.Close()
		return nil, err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(appLog, &slog.HandlerOptions{Level: slog.LevelDebug})))
	slog.Info("WorldKit started", "crashLog", crashPath)
	if len(leftovers) > 0 {
		slog.Warn("earlier runs did not exit cleanly", "crashLogs", leftovers)
	}
	return &Session{appLog: appLog, crashPath: crashPath}, nil
}

// openAppLog keeps the previous run's log as app.previous.log and starts a fresh app.log.
func openAppLog(dir string) (*os.File, error) {
	path := filepath.Join(dir, appLogName)
	err := os.Rename(path, filepath.Join(dir, previousLogName))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("keep previous app log: %w", err)
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create app log %q: %w", path, err)
	}
	return file, nil
}

func setCrashOutput(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create crash log %q: %w", path, err)
	}
	// SetCrashOutput keeps its own duplicate of the handle, so ours can be closed.
	defer file.Close()
	if err := debug.SetCrashOutput(file, debug.CrashOptions{}); err != nil {
		return fmt.Errorf("set crash output: %w", err)
	}
	return nil
}

// End is deferred by the function that runs the app, with a pointer to its error result.
// On a panic it logs the panic and its stack, then panics again without closing, so the
// crash log still gets the trace: closing would release crash output and delete the file
// before the runtime writes to it. Otherwise it calls Close.
func (s *Session) End(exitErr *error) {
	recovered := recover()
	if recovered == nil {
		s.Close(*exitErr)
		return
	}
	slog.Error("WorldKit panicked", "panic", recovered, "stack", string(debug.Stack()))
	panic(recovered)
}

// Close logs why the app stopped (exitErr, nil for a normal exit), stops writing crash
// output and removes this run's crash file if nothing crashed.
func (s *Session) Close(exitErr error) {
	if exitErr != nil {
		// e.g. Gio closes the window with an error instead of panicking when the GPU fails.
		slog.Error("WorldKit stopped with an error", "error", exitErr)
	} else {
		slog.Info("WorldKit stopped")
	}
	// Releases the runtime's handle so the empty file can be deleted.
	if err := debug.SetCrashOutput(nil, debug.CrashOptions{}); err != nil {
		slog.Warn("release crash output", "error", err)
	}
	removeIfEmpty(s.crashPath)
	if err := s.appLog.Close(); err != nil {
		slog.Warn("close app log", "error", err)
	}
}

func removeIfEmpty(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() > 0 {
		return
	}
	if err := os.Remove(path); err != nil {
		slog.Warn("remove empty crash log", "path", path, "error", err)
	}
}
