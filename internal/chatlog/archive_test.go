package chatlog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const (
	headerA = "[DATE: 18/OCT/2026 | TIME: 09:57:01]"
	headerB = "[DATE: 19/OCT/2026 | TIME: 18:00:00]"
)

var captureTime = time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)

func TestArchiveSave(t *testing.T) {
	tests := []struct {
		name      string
		snapshots []string
		wantFiles map[string]string // path relative to the archive -> content
		wantLines int
	}{
		{
			name:      "growing log appends only new lines",
			snapshots: []string{headerA + "\nhello", headerA + "\nhello\nworld", headerA + "\nhello\nworld"},
			wantFiles: map[string]string{"2026/10/2026-10-18_095701.txt": headerA + "\nhello\nworld\n"},
			wantLines: 3,
		},
		{
			name:      "new header starts a new file",
			snapshots: []string{headerA + "\none", headerB + "\ntwo"},
			wantFiles: map[string]string{
				"2026/10/2026-10-18_095701.txt": headerA + "\none\n",
				"2026/10/2026-10-19_180000.txt": headerB + "\ntwo\n",
			},
			wantLines: 4,
		},
		{
			name:      "dropped oldest lines still continue the session",
			snapshots: []string{headerA + "\na\nb\nc", "b\nc\nd"},
			wantFiles: map[string]string{"2026/10/2026-10-18_095701.txt": headerA + "\na\nb\nc\nd\n"},
			wantLines: 5,
		},
		{
			name:      "unrelated headerless text becomes its own session",
			snapshots: []string{headerA + "\na", "x\ny"},
			wantFiles: map[string]string{
				"2026/10/2026-10-18_095701.txt": headerA + "\na\n",
				"2026/10/2026-10-18_120000.txt": "x\ny\n",
			},
			wantLines: 4,
		},
		{
			name:      "CRLF is normalized",
			snapshots: []string{headerA + "\r\nhello\r\n"},
			wantFiles: map[string]string{"2026/10/2026-10-18_095701.txt": headerA + "\nhello\n"},
			wantLines: 2,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			archive := mustOpenArchive(t, dir)
			defer archive.Close()
			totalLines := 0
			for _, snapshot := range test.snapshots {
				lines, err := archive.Save(snapshot, captureTime)
				if err != nil {
					t.Fatal(err)
				}
				totalLines += lines
			}
			if totalLines != test.wantLines {
				t.Errorf("saved %d lines, want %d", totalLines, test.wantLines)
			}
			assertFiles(t, dir, test.wantFiles)
		})
	}
}

func TestArchiveResumesAfterRestartAndRepairsTornLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "2026", "10", "2026-10-18_095701.txt")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(headerA+"\nhello\nwor"), 0o644); err != nil {
		t.Fatal(err)
	}

	archive := mustOpenArchive(t, dir)
	defer archive.Close()
	lines, err := archive.Save(headerA+"\nhello\nworld\nagain", captureTime)
	if err != nil {
		t.Fatal(err)
	}
	if lines != 2 {
		t.Errorf("saved %d lines, want 2", lines)
	}
	assertFiles(t, dir, map[string]string{"2026/10/2026-10-18_095701.txt": headerA + "\nhello\nworld\nagain\n"})
}

func TestOpenArchiveRejectsSecondInstance(t *testing.T) {
	dir := t.TempDir()
	first := mustOpenArchive(t, dir)
	defer first.Close()

	if _, err := OpenArchive(dir); !errors.Is(err, ErrArchiveInUse) {
		t.Fatalf("second OpenArchive error = %v, want ErrArchiveInUse", err)
	}
}

func mustOpenArchive(t *testing.T, dir string) *Archive {
	t.Helper()
	archive, err := OpenArchive(dir)
	if err != nil {
		t.Fatal(err)
	}
	return archive
}

func assertFiles(t *testing.T, dir string, want map[string]string) {
	t.Helper()
	got := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != sessionFileExt {
			return err
		}
		content, err := os.ReadFile(path)
		relative, _ := filepath.Rel(dir, path) // path is always inside dir
		got[filepath.ToSlash(relative)] = string(content)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Errorf("got files %v, want %v", got, want)
	}
	for path, content := range want {
		if got[path] != content {
			t.Errorf("%s = %q, want %q", path, got[path], content)
		}
	}
}
