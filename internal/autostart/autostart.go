// Package autostart turns "start WorldKit when Windows starts" on and off for the current user.
//
// It uses the per-user Run registry key, which needs no administrator rights and shows up
// in Task Manager's Startup apps list where the user can also disable it.
package autostart

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows/registry"
)

const runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`

// MinimizedFlag is the command-line flag the startup entry passes, so WorldKit
// starts quietly in the taskbar instead of popping up at login.
const MinimizedFlag = "minimized"

// Entry is WorldKit's startup entry in the registry.
type Entry struct {
	name    string
	command string
}

// NewEntry creates the startup entry for the running executable.
func NewEntry(name string) (*Entry, error) {
	executable, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("find WorldKit executable: %w", err)
	}
	return &Entry{name: name, command: fmt.Sprintf(`"%s" -%s`, executable, MinimizedFlag)}, nil
}

// IsEnabled reports whether WorldKit is registered to start with Windows.
func (e *Entry) IsEnabled() (bool, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.QUERY_VALUE)
	if err != nil {
		return false, fmt.Errorf("read startup apps: %w", err)
	}
	defer key.Close()
	_, _, err = key.GetStringValue(e.name)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read startup entry %q: %w", e.name, err)
	}
	return true, nil
}

// SetEnabled registers or removes WorldKit as a startup app.
func (e *Entry) SetEnabled(isEnabled bool) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open startup apps: %w", err)
	}
	defer key.Close()
	if isEnabled {
		err = key.SetStringValue(e.name, e.command)
	} else {
		err = key.DeleteValue(e.name)
		if errors.Is(err, registry.ErrNotExist) {
			err = nil
		}
	}
	if err != nil {
		return fmt.Errorf("update startup entry %q: %w", e.name, err)
	}
	return nil
}
