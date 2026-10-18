package chatlog

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// sessionNameLayout is how session files are named, e.g. 2026-10-18_095701.txt.
const sessionNameLayout = "2006-01-02_150405"

// Session describes one saved session file.
type Session struct {
	Path    string
	Number  int       // 1 for the oldest session in the archive
	Started time.Time // UTC, from the file name; zero when the name doesn't hold a time
	Updated time.Time // when the file was last written
}

// Library reads the sessions an Archive saved. It never writes, so it is safe to use while
// capture is running.
type Library struct {
	dir string
}

// NewLibrary returns a library for the archive in dir.
func NewLibrary(dir string) *Library {
	return &Library{dir: dir}
}

// Sessions returns every saved session, newest first. An archive that doesn't exist yet
// has no sessions.
func (l *Library) Sessions() ([]Session, error) {
	paths, err := sessionFiles(l.dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, 0, len(paths))
	for index, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("read session %q: %w", path, err)
		}
		sessions = append(sessions, Session{
			Path:    path,
			Number:  index + 1,
			Started: sessionStart(path),
			Updated: info.ModTime().UTC(),
		})
	}
	slices.Reverse(sessions)
	return sessions, nil
}

// Records parses the session file at path.
func (l *Library) Records(path string) ([]Record, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open session %q: %w", path, err)
	}
	defer file.Close()
	return Parse(file)
}

// sessionFiles returns the archive's session files, oldest first. File names sort
// chronologically, so lexical order is time order.
func sessionFiles(dir string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Ext(path) == sessionFileExt {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list sessions in %q: %w", dir, err)
	}
	slices.Sort(paths)
	return paths, nil
}

// sessionStart reads the start time from a session file name. Names may carry a "_2"
// style suffix when two sessions started in the same second.
func sessionStart(path string) time.Time {
	name := strings.TrimSuffix(filepath.Base(path), sessionFileExt)
	if len(name) < len(sessionNameLayout) {
		return time.Time{}
	}
	start, err := time.Parse(sessionNameLayout, name[:len(sessionNameLayout)])
	if err != nil {
		return time.Time{}
	}
	return start
}
