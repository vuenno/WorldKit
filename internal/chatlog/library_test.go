package chatlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLibrarySessions(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"2026/09/2026-09-30_220000.txt",
		"2026/10/2026-10-18_095701.txt",
		"2026/10/2026-10-18_095701_2.txt",
		"2026/10/notes.md",
	} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(headerA+"\nhello\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	sessions, err := NewLibrary(dir).Sessions()
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		name    string
		number  int
		started time.Time
	}{
		{"2026-10-18_095701_2.txt", 3, time.Date(2026, 10, 18, 9, 57, 1, 0, time.UTC)},
		{"2026-10-18_095701.txt", 2, time.Date(2026, 10, 18, 9, 57, 1, 0, time.UTC)},
		{"2026-09-30_220000.txt", 1, time.Date(2026, 9, 30, 22, 0, 0, 0, time.UTC)},
	}
	if len(sessions) != len(want) {
		t.Fatalf("got %d sessions, want %d", len(sessions), len(want))
	}
	for i, session := range sessions {
		if filepath.Base(session.Path) != want[i].name || session.Number != want[i].number ||
			!session.Started.Equal(want[i].started) || session.Updated.IsZero() {
			t.Errorf("session %d = %+v, want %+v", i, session, want[i])
		}
	}
}

func TestLibrarySessionsWithoutArchive(t *testing.T) {
	sessions, err := NewLibrary(filepath.Join(t.TempDir(), "missing")).Sessions()
	if err != nil || len(sessions) != 0 {
		t.Errorf("got %v, %v; want no sessions and no error", sessions, err)
	}
}

func TestLibraryRecords(t *testing.T) {
	path := filepath.Join(t.TempDir(), "2026-10-18_095701.txt")
	if err := os.WriteFile(path, []byte(headerA+"\n* John Doe waves.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	records, err := NewLibrary(filepath.Dir(path)).Records(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 || records[1].Kind != KindMe || records[1].Speaker != "John Doe" {
		t.Errorf("got %+v", records)
	}
}
