package exports

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// exportFile writes a file and returns an entry that matches it.
func exportFile(t *testing.T, path string, saved time.Time) Entry {
	t.Helper()
	if err := os.WriteFile(path, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return Entry{Path: path, Saved: saved, Format: "png", Bytes: info.Size(), Modified: info.ModTime()}
}

func TestAddListRemove(t *testing.T) {
	dir := t.TempDir()
	index := Open(filepath.Join(dir, "exports.json"))
	older := exportFile(t, filepath.Join(dir, "a.png"), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC))
	newer := exportFile(t, filepath.Join(dir, "b.png"), time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC))
	for _, entry := range []Entry{older, newer, newer} {
		if err := index.Add(entry); err != nil {
			t.Fatal(err)
		}
	}
	items, err := index.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Path != newer.Path {
		t.Fatalf("List = %+v, want b then a, no duplicate", items)
	}
	if err := os.Remove(older.Path); err != nil {
		t.Fatal(err)
	}
	items, _ = index.List()
	if !items[1].IsMissing {
		t.Error("a deleted file should be listed as missing")
	}
	if err := index.Remove(strings.ToUpper(older.Path)); err != nil {
		t.Fatal(err)
	}
	if items, _ = index.List(); len(items) != 1 {
		t.Errorf("after Remove: %+v", items)
	}
}

func TestDeleteOnlyListedUnchangedFiles(t *testing.T) {
	dir := t.TempDir()
	index := Open(filepath.Join(dir, "exports.json"))
	listed := exportFile(t, filepath.Join(dir, "listed.png"), time.Now())
	if err := index.Add(listed); err != nil {
		t.Fatal(err)
	}
	stranger := filepath.Join(dir, "other.png")
	exportFile(t, stranger, time.Now())
	if err := index.Delete(stranger); !errors.Is(err, ErrNotExported) {
		t.Errorf("Delete(unlisted) = %v, want ErrNotExported", err)
	}
	if err := os.WriteFile(listed.Path, []byte("a different, longer file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := index.Delete(listed.Path); !errors.Is(err, ErrChanged) {
		t.Errorf("Delete(changed) = %v, want ErrChanged", err)
	}
	listed = exportFile(t, listed.Path, time.Now())
	if err := index.Add(listed); err != nil {
		t.Fatal(err)
	}
	if err := index.Delete(listed.Path); err != nil {
		t.Fatalf("Delete = %v", err)
	}
	if _, err := os.Stat(listed.Path); !errors.Is(err, os.ErrNotExist) {
		t.Error("file should be gone")
	}
	if _, err := os.Stat(stranger); err != nil {
		t.Error("an unlisted file must never be deleted")
	}
}
