package screenshot

import (
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

// Save renders lines and writes them to a new PNG file named after the current time.
func (s *Store) Save(lines []Line) (Shot, error) {
	img, err := Render(lines)
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
	if err := writePNG(path, img); err != nil {
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

// writePNG writes img to a temporary file first, so a crash never leaves a half-written
// screenshot under its final name.
func writePNG(path string, img image.Image) error {
	file, err := os.CreateTemp(filepath.Dir(path), "saving-*.tmp")
	if err != nil {
		return fmt.Errorf("create screenshot file: %w", err)
	}
	tempPath := file.Name()
	encodeErr := png.Encode(file, img)
	closeErr := file.Close()
	if err := errors.Join(encodeErr, closeErr); err != nil {
		// The temp file is useless now; failing to remove it only leaves a stray .tmp.
		_ = os.Remove(tempPath)
		return fmt.Errorf("write screenshot %q: %w", path, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("save screenshot %q: %w", path, err)
	}
	return nil
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

// Load decodes the screenshot at path.
func (s *Store) Load(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open screenshot %q: %w", path, err)
	}
	defer file.Close()
	img, err := png.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode screenshot %q: %w", path, err)
	}
	return img, nil
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
