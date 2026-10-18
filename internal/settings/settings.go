// Package settings loads and saves WorldKit's user preferences as a small JSON file.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
)

// Settings are the preferences remembered between launches.
type Settings struct {
	// FiveMDataDir is the FiveM data folder the user picked; empty means the default location.
	FiveMDataDir string `json:"fivemDataDir,omitempty"`
	// HasAnsweredStartupPrompt is true once the user chose whether WorldKit starts with Windows.
	HasAnsweredStartupPrompt bool `json:"hasAnsweredStartupPrompt"`
}

// Store holds the current settings and writes every change to disk. Safe for concurrent use.
type Store struct {
	path string

	mu      sync.Mutex
	current Settings
}

// Open loads settings from path. A missing file gives the defaults. A corrupt file also
// gives the defaults, so a bad settings file never stops WorldKit from starting; it is
// replaced on the next save.
func Open(path string) (*Store, error) {
	store := &Store{path: path}
	content, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load settings %q: %w", path, err)
	}
	if err := json.Unmarshal(content, &store.current); err != nil {
		slog.Warn("settings file is corrupt; using defaults", "path", path, "error", err)
		store.current = Settings{}
	}
	return store, nil
}

// Get returns a copy of the current settings.
func (s *Store) Get() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

// Update applies change and saves the result. On a save error the settings stay unchanged.
func (s *Store) Update(change func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.current
	change(&next)
	if err := writeFileAtomically(s.path, next); err != nil {
		return err
	}
	s.current = next
	return nil
}

// writeFileAtomically writes to a temporary file first, so a crash mid-write
// leaves the previous settings file intact instead of a half-written one.
func writeFileAtomically(path string, settings Settings) error {
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("save settings %q: %w", path, err)
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "settings-*.tmp")
	if err != nil {
		return fmt.Errorf("save settings %q: %w", path, err)
	}
	defer os.Remove(temp.Name()) // fails harmlessly once the rename has moved it
	_, writeErr := temp.Write(content)
	syncErr := temp.Sync()
	closeErr := temp.Close()
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return fmt.Errorf("save settings %q: %w", path, err)
	}
	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("save settings %q: %w", path, err)
	}
	return nil
}
