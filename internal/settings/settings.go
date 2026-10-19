// Package settings loads and saves WorldKit's user preferences as a small JSON file.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"sync"

	"world-kit/internal/atomicfile"
)

// Settings are the preferences remembered between launches.
type Settings struct {
	// FiveMDataDir is the FiveM data folder the user picked; empty means the default location.
	FiveMDataDir string `json:"fivemDataDir,omitempty"`
	// HasAnsweredStartupPrompt is true once the user chose whether WorldKit starts with Windows.
	HasAnsweredStartupPrompt bool `json:"hasAnsweredStartupPrompt"`
	// ExportDir is where the editor's Save As dialog opens; empty means Pictures\WorldKit.
	ExportDir string `json:"exportDir,omitempty"`
	// LastExportDir is the folder the last export went to, where the next one starts.
	LastExportDir string `json:"lastExportDir,omitempty"`
	// ExportPresets are the user's saved export choices, in the order they were saved.
	ExportPresets []ExportPreset `json:"exportPresets,omitempty"`
	// DefaultPreset names the preset the export dialog starts with; empty: PNG, lossless.
	DefaultPreset string `json:"defaultExportPreset,omitempty"`
}

// ExportPreset is a saved combination of file format, quality and canvas size.
type ExportPreset struct {
	Name       string `json:"name"`
	Format     string `json:"format"` // "png" or "jpeg"
	Quality    int    `json:"quality,omitempty"`
	IsLossless bool   `json:"lossless,omitempty"` // PNG only
	Width      int    `json:"width"`
	Height     int    `json:"height"`
}

// Preset returns the preset called name.
func (s Settings) Preset(name string) (ExportPreset, bool) {
	for _, preset := range s.ExportPresets {
		if preset.Name == name {
			return preset, true
		}
	}
	return ExportPreset{}, false
}

// SavePreset adds preset, replacing one with the same name.
func (s *Settings) SavePreset(preset ExportPreset) {
	for i := range s.ExportPresets {
		if s.ExportPresets[i].Name == preset.Name {
			s.ExportPresets[i] = preset
			return
		}
	}
	s.ExportPresets = append(s.ExportPresets, preset)
}

// DeletePreset removes the preset called name, and makes it no longer the default.
func (s *Settings) DeletePreset(name string) {
	kept := s.ExportPresets[:0:0]
	for _, preset := range s.ExportPresets {
		if preset.Name != name {
			kept = append(kept, preset)
		}
	}
	s.ExportPresets = kept
	if s.DefaultPreset == name {
		s.DefaultPreset = ""
	}
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
	return s.current.clone()
}

// Update applies change and saves the result. On a save error the settings stay unchanged.
func (s *Store) Update(change func(*Settings)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.current.clone()
	change(&next)
	if err := writeFileAtomically(s.path, next); err != nil {
		return err
	}
	s.current = next
	return nil
}

// writeFileAtomically saves through a temporary file, so a crash mid-write leaves the
// previous settings file intact instead of a half-written one.
func writeFileAtomically(path string, settings Settings) error {
	content, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("encode settings: %w", err)
	}
	if err := atomicfile.Write(path, content); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	return nil
}

// clone copies s so callers can change its presets without touching the stored ones.
func (s Settings) clone() Settings {
	s.ExportPresets = slices.Clone(s.ExportPresets)
	return s
}
