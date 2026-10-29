package screenshot

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"world-kit/internal/atomicfile"
)

const (
	// shotNameLayout is how screenshot files are named (UTC, like session files),
	// e.g. 2026-10-18_101530.png.
	shotNameLayout = "2006-01-02_150405"
	shotFileExt    = ".png"
)

// ErrNotInStore is returned when asked to delete a file the store didn't save.
var ErrNotInStore = errors.New("file is not a saved screenshot")

// Shot describes one saved screenshot file.
type Shot struct {
	Path  string
	Saved time.Time // UTC, from the file name; the file's modification time if the name has none
}

// Store saves chat screenshots as PNG files in one folder and lists, reads and deletes them.
type Store struct {
	dir string
}

// NewStore returns a store for the folder dir, which is created on the first save.
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Save renders lines in style and writes them to a new PNG file named after the current time.
func (s *Store) Save(lines []Line, style Style) (Shot, error) {
	picture, err := Render(lines, style)
	if err != nil {
		return Shot{}, err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return Shot{}, fmt.Errorf("create screenshot folder %q: %w", s.dir, err)
	}
	saved := time.Now().UTC().Truncate(time.Second)
	path, err := s.freePath(saved)
	if err != nil {
		return Shot{}, err
	}
	if err := writePNG(path, picture); err != nil {
		return Shot{}, err
	}
	return Shot{Path: path, Saved: saved}, nil
}

// freePath returns a file name for saved that isn't taken, adding "_2", "_3", … when
// several screenshots are saved in the same second.
func (s *Store) freePath(saved time.Time) (string, error) {
	base := filepath.Join(s.dir, saved.Format(shotNameLayout))
	candidate := base + shotFileExt
	for n := 2; ; n++ {
		_, err := os.Stat(candidate)
		if errors.Is(err, fs.ErrNotExist) {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("check screenshot name %q: %w", candidate, err)
		}
		candidate = fmt.Sprintf("%s_%d%s", base, n, shotFileExt)
	}
}

// writePNG encodes picture and writes it atomically, so a crash never leaves a
// half-written screenshot under its final name.
func writePNG(path string, picture *Picture) error {
	data, err := encodePNG(picture.Image, picture.Look)
	if err != nil {
		return fmt.Errorf("encode screenshot %q: %w", path, err)
	}
	return atomicfile.Write(path, data)
}

// List returns every saved screenshot, newest first. A folder that doesn't exist yet
// holds no screenshots.
func (s *Store) List() ([]Shot, error) {
	entries, err := os.ReadDir(s.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("list screenshots in %q: %w", s.dir, err)
	}
	var shots []Shot
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != shotFileExt {
			continue
		}
		shot, err := s.shotFromEntry(entry)
		if err != nil {
			return nil, err
		}
		shots = append(shots, shot)
	}
	slices.SortStableFunc(shots, func(a, b Shot) int { return strings.Compare(b.Path, a.Path) })
	return shots, nil
}

func (s *Store) shotFromEntry(entry fs.DirEntry) (Shot, error) {
	path := filepath.Join(s.dir, entry.Name())
	if saved, ok := savedFromName(entry.Name()); ok {
		return Shot{Path: path, Saved: saved}, nil
	}
	info, err := entry.Info()
	if err != nil {
		return Shot{}, fmt.Errorf("read screenshot %q: %w", path, err)
	}
	return Shot{Path: path, Saved: info.ModTime().UTC()}, nil
}

func savedFromName(name string) (time.Time, bool) {
	name = strings.TrimSuffix(name, shotFileExt)
	if len(name) < len(shotNameLayout) {
		return time.Time{}, false
	}
	saved, err := time.Parse(shotNameLayout, name[:len(shotNameLayout)])
	return saved, err == nil
}

// SavedImage is a decoded screenshot file and where its rows of text are.
type SavedImage struct {
	Image image.Image
	Grid  RowGrid
}

// Load decodes the screenshot at path.
func (s *Store) Load(path string) (SavedImage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return SavedImage{}, fmt.Errorf("read screenshot %q: %w", path, err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return SavedImage{}, fmt.Errorf("decode screenshot %q: %w", path, err)
	}
	return SavedImage{Image: img, Grid: savedGrid(data)}, nil
}

// Delete removes the screenshot at path for good. It refuses files outside the store's
// folder, so a bad path can never delete anything else.
func (s *Store) Delete(path string) error {
	if filepath.Dir(filepath.Clean(path)) != filepath.Clean(s.dir) || filepath.Ext(path) != shotFileExt {
		return fmt.Errorf("delete %q: %w", path, ErrNotInStore)
	}
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("delete screenshot %q: %w", path, err)
	}
	return nil
}

// Dir returns the folder screenshots are saved in.
func (s *Store) Dir() string {
	return s.dir
}
