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
	// ExtractPresets are the user's saved looks for Extract image (background, chat font
	// and scale, max width), in the order they were saved.
	ExtractPresets []ExtractPreset `json:"extractPresets,omitempty"`
	// DefaultExtractPreset names the preset Extract image starts with; empty: the defaults.
	DefaultExtractPreset string `json:"defaultExtractPreset,omitempty"`
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

// ExtractPreset is a saved look for chat screenshots made with Extract image.
type ExtractPreset struct {
	Name      string `json:"name"`
	Backdrop  string `json:"backdrop"`            // "transparent", "dark" or "light"
	ChatFont  string `json:"chatFont,omitempty"`  // "Arial" or "Mukta"; empty: Arial
	ChatScale int    `json:"chatScale,omitempty"` // percent; zero: from FontSize, or the default
	// FontSize is what presets saved before chat scales recorded instead: the font size in
	// pixels. It is only read, to pick the nearest chat scale.
	FontSize int `json:"fontSize,omitempty"`
	// WidthLimit is the image's max width in pixels; zero: the in-game chat's width.
	WidthLimit int `json:"widthLimit,omitempty"`
}

func (p ExportPreset) presetName() string  { return p.Name }
func (p ExtractPreset) presetName() string { return p.Name }

// named is a preset, found by its name.
type named interface{ presetName() string }

func findPreset[T named](presets []T, name string) (T, bool) {
	for _, preset := range presets {
		if preset.presetName() == name {
			return preset, true
		}
	}
	var none T
	return none, false
}

// savePreset adds preset, replacing one with the same name in place.
func savePreset[T named](presets []T, preset T) []T {
	for i := range presets {
		if presets[i].presetName() == preset.presetName() {
			presets[i] = preset
			return presets
		}
	}
	return append(presets, preset)
}

func deletePreset[T named](presets []T, name string) []T {
	return slices.DeleteFunc(slices.Clone(presets), func(p T) bool { return p.presetName() == name })
}

// Preset returns the export preset called name.
func (s Settings) Preset(name string) (ExportPreset, bool) {
	return findPreset(s.ExportPresets, name)
}

// SavePreset adds an export preset, replacing one with the same name.
func (s *Settings) SavePreset(preset ExportPreset) {
	s.ExportPresets = savePreset(s.ExportPresets, preset)
}

// DeletePreset removes the export preset called name, and makes it no longer the default.
func (s *Settings) DeletePreset(name string) {
	s.ExportPresets = deletePreset(s.ExportPresets, name)
	if s.DefaultPreset == name {
		s.DefaultPreset = ""
	}
}

// ExtractPreset returns the extract preset called name.
func (s Settings) ExtractPreset(name string) (ExtractPreset, bool) {
	return findPreset(s.ExtractPresets, name)
}

// SaveExtractPreset adds an extract preset, replacing one with the same name.
func (s *Settings) SaveExtractPreset(preset ExtractPreset) {
	s.ExtractPresets = savePreset(s.ExtractPresets, preset)
}

// DeleteExtractPreset removes the extract preset called name, and makes it no longer the
// default.
func (s *Settings) DeleteExtractPreset(name string) {
	s.ExtractPresets = deletePreset(s.ExtractPresets, name)
	if s.DefaultExtractPreset == name {
		s.DefaultExtractPreset = ""
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
	s.ExtractPresets = slices.Clone(s.ExtractPresets)
	return s
}
