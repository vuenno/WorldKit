// Package exports remembers the images the Screenshot Editor exported, wherever the user
// saved them, in one small JSON index. The files themselves are never copied.
package exports

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"world-kit/internal/atomicfile"
)

var (
	// ErrNotExported is returned when asked to delete a file the index doesn't list.
	ErrNotExported = errors.New("file is not an exported screenshot")
	// ErrChanged is returned when asked to delete a file that changed since it was
	// exported: it may be a different file that took the name.
	ErrChanged = errors.New("file changed since it was exported")
)

// Entry is one exported image.
type Entry struct {
	Path       string    `json:"path"`
	Saved      time.Time `json:"saved"`  // UTC
	Format     string    `json:"format"` // "png" or "jpeg"
	Quality    int       `json:"quality,omitempty"`
	IsLossless bool      `json:"lossless,omitempty"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	Bytes      int64     `json:"bytes"`
	Modified   time.Time `json:"modified"` // the file's modification time right after export
}

// Item is an entry as found on disk now.
type Item struct {
	Entry
	IsMissing bool // the file was moved or deleted
	IsChanged bool // the file is there but isn't the one WorldKit wrote
}

// Index is the list of exports, saved to a JSON file on every change. Safe for concurrent use.
type Index struct {
	path string
	mu   sync.Mutex
}

// Open returns the index stored at path; the file is created on the first Add.
func Open(path string) *Index {
	return &Index{path: path}
}

// Add records an export, replacing any entry for the same file.
func (x *Index) Add(entry Entry) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	entries, err := x.read()
	if err != nil {
		return err
	}
	entries = slices.DeleteFunc(entries, func(e Entry) bool { return samePath(e.Path, entry.Path) })
	return x.write(append(entries, entry))
}

// List returns every export, newest first, checking each file on disk.
func (x *Index) List() ([]Item, error) {
	x.mu.Lock()
	entries, err := x.read()
	x.mu.Unlock()
	if err != nil {
		return nil, err
	}
	items := make([]Item, len(entries))
	for i, entry := range entries {
		items[i] = Item{Entry: entry}
		info, err := os.Stat(entry.Path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			items[i].IsMissing = true
		case err != nil:
			slog.Warn("check exported screenshot", "path", entry.Path, "error", err)
		default:
			items[i].IsChanged = !matches(entry, info)
		}
	}
	slices.SortStableFunc(items, func(a, b Item) int { return b.Saved.Compare(a.Saved) })
	return items, nil
}

// Remove forgets an export without touching its file.
func (x *Index) Remove(path string) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	entries, err := x.read()
	if err != nil {
		return err
	}
	return x.write(slices.DeleteFunc(entries, func(e Entry) bool { return samePath(e.Path, path) }))
}

// Delete removes an exported file for good and forgets it. It only deletes files the
// index lists, with an image extension, that are still the file WorldKit wrote, so a bad
// path can never delete anything else.
func (x *Index) Delete(path string) error {
	x.mu.Lock()
	defer x.mu.Unlock()
	entries, err := x.read()
	if err != nil {
		return err
	}
	index := slices.IndexFunc(entries, func(e Entry) bool { return samePath(e.Path, path) })
	if index < 0 || !hasImageExtension(path) {
		return fmt.Errorf("delete %q: %w", path, ErrNotExported)
	}
	entry := entries[index]
	info, err := os.Lstat(entry.Path)
	if err != nil {
		return fmt.Errorf("delete exported screenshot %q: %w", entry.Path, err)
	}
	if !info.Mode().IsRegular() || !matches(entry, info) {
		return fmt.Errorf("delete %q: %w", entry.Path, ErrChanged)
	}
	if err := os.Remove(entry.Path); err != nil {
		return fmt.Errorf("delete exported screenshot %q: %w", entry.Path, err)
	}
	return x.write(slices.Delete(entries, index, index+1))
}

// matches reports whether info still describes the file the entry recorded.
func matches(entry Entry, info fs.FileInfo) bool {
	return info.Size() == entry.Bytes && info.ModTime().Equal(entry.Modified)
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) // Windows paths ignore case
}

func hasImageExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg":
		return true
	}
	return false
}

// read loads the entries; a missing file is an empty index and a corrupt one is logged and
// treated as empty, so it never stops the app (it is rewritten on the next change).
func (x *Index) read() ([]Entry, error) {
	content, err := os.ReadFile(x.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read export list %q: %w", x.path, err)
	}
	var entries []Entry
	if err := json.Unmarshal(content, &entries); err != nil {
		slog.Warn("export list is corrupt; starting a new one", "path", x.path, "error", err)
		return nil, nil
	}
	return entries, nil
}

func (x *Index) write(entries []Entry) error {
	content, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("encode export list: %w", err)
	}
	return atomicfile.Write(x.path, content)
}
