// Package chatlog keeps GTA World chat sessions as plain text files and parses them into records.
//
// The captured text is stored exactly as the game wrote it, one append-only file per game
// session. Nothing parsed is written to disk, so improved parse rules apply to old sessions too.
package chatlog

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const sessionFileExt = ".txt"

// Archive saves captured chat text into one file per game session under a directory.
// Only one Archive can be open per directory at a time (see ErrArchiveInUse).
type Archive struct {
	dir     string
	lock    *archiveLock
	session *sessionFile
}

// OpenArchive opens (creating if needed) the archive in dir and resumes its newest session,
// so text captured after a restart continues the same file.
func OpenArchive(dir string) (*Archive, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create archive %q: %w", dir, err)
	}
	lock, err := acquireLock(filepath.Join(dir, "archive.lock"))
	if err != nil {
		return nil, err
	}
	archive := &Archive{dir: dir, lock: lock}
	if err := archive.resumeNewestSession(); err != nil {
		archive.Close()
		return nil, err
	}
	return archive, nil
}

// Dir returns the directory the archive writes to.
func (a *Archive) Dir() string {
	return a.dir
}

// Close releases the open session file and the archive lock.
func (a *Archive) Close() {
	if a.session != nil {
		a.session.close()
		a.session = nil
	}
	a.lock.release()
}

// Save stores whatever part of the game's current chat log is not saved yet and returns how
// many lines it added. The game rewrites its whole log on every line and starts over on
// reconnect, so text either continues the open session or begins a new one.
func (a *Archive) Save(text string, capturedAt time.Time) (int, error) {
	text = normalizeText(text)
	if text == "" {
		return 0, nil
	}
	header := sessionHeader(text)
	if a.session == nil || (header != "" && header != a.session.header) {
		if err := a.openSession(header, capturedAt); err != nil {
			return 0, err
		}
	}
	addition, ok := continuation(a.session.saved, text)
	if !ok {
		// Neither a known header nor a continuation: the game started over without a
		// header we recognise, so keep it as a session of its own.
		if err := a.openSession("", capturedAt); err != nil {
			return 0, err
		}
		addition = text
	}
	return a.session.append(addition)
}

func (a *Archive) resumeNewestSession() error {
	path, err := newestSessionFile(a.dir)
	if err != nil || path == "" {
		return err
	}
	session, err := loadSessionFile(path)
	if err != nil {
		return err
	}
	a.session = session
	return nil
}

// openSession switches to the file for header, reusing it if it already exists.
// Sessions without a header get a new file named after the capture time.
func (a *Archive) openSession(header string, capturedAt time.Time) error {
	if a.session != nil {
		a.session.close()
		a.session = nil
	}
	start := capturedAt.UTC()
	if parsed, ok := parseSessionHeader(header); ok {
		start = parsed
	}
	path := sessionPath(a.dir, start)
	if header == "" {
		path = unusedPath(path)
	}
	session, err := loadSessionFile(path)
	if err != nil {
		return err
	}
	a.session = session
	return nil
}

// sessionPath files sessions by year and month, e.g. logs/2026/10/2026-10-18_095701.txt.
func sessionPath(dir string, start time.Time) string {
	return filepath.Join(dir, start.Format("2006"), start.Format("01"),
		start.Format("2006-01-02_150405")+sessionFileExt)
}

func unusedPath(path string) string {
	base := strings.TrimSuffix(path, sessionFileExt)
	candidate := path
	for n := 2; fileExists(candidate); n++ {
		candidate = fmt.Sprintf("%s_%d%s", base, n, sessionFileExt)
	}
	return candidate
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// newestSessionFile returns the most recent session file, or "" when there is none.
// File names sort chronologically, so the lexically greatest path is the newest.
func newestSessionFile(dir string) (string, error) {
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
		return "", fmt.Errorf("list sessions in %q: %w", dir, err)
	}
	if len(paths) == 0 {
		return "", nil
	}
	return slices.Max(paths), nil
}

// continuation returns the part of text that comes after the saved lines, or false when text
// does not continue them. If the game dropped its oldest lines, the longest saved tail that
// text starts with is used as the overlap.
func continuation(saved, text string) (string, bool) {
	if strings.HasPrefix(text, saved) {
		return text[len(saved):], true
	}
	for start := 0; start < len(saved); {
		lineEnd := strings.IndexByte(saved[start:], '\n')
		if lineEnd < 0 {
			break
		}
		start += lineEnd + 1
		tail := saved[start:]
		if tail != "" && strings.HasPrefix(text, tail) {
			return text[len(tail):], true
		}
	}
	return "", false
}

// normalizeText makes text valid UTF-8 with LF line endings and a final newline,
// so saved files always end on a complete line.
func normalizeText(text string) string {
	text = strings.ToValidUTF8(text, "�")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if text == "" || strings.HasSuffix(text, "\n") {
		return text
	}
	return text + "\n"
}

// sessionFile is the open, append-only file of one game session.
type sessionFile struct {
	header string
	file   *os.File
	saved  string // everything written so far; always ends with "\n" or is empty
}

// loadSessionFile opens path for appending, creating it if needed. A line torn by a crash
// mid-write is cut off; the game still holds it and the next Save writes it again.
func loadSessionFile(path string) (*sessionFile, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create session folder for %q: %w", path, err)
	}
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read session %q: %w", path, err)
	}
	saved := string(content[:strings.LastIndexByte(string(content), '\n')+1])
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open session %q: %w", path, err)
	}
	if err := trimTo(file, int64(len(saved))); err != nil {
		file.Close()
		return nil, fmt.Errorf("repair session %q: %w", path, err)
	}
	return &sessionFile{header: sessionHeader(saved), file: file, saved: saved}, nil
}

func trimTo(file *os.File, size int64) error {
	if err := file.Truncate(size); err != nil {
		return err
	}
	_, err := file.Seek(size, 0)
	return err
}

// append writes addition and flushes it to disk before returning the number of lines added.
func (s *sessionFile) append(addition string) (int, error) {
	if addition == "" {
		return 0, nil
	}
	if _, err := s.file.WriteString(addition); err != nil {
		return 0, fmt.Errorf("write session %q: %w", s.file.Name(), err)
	}
	if err := s.file.Sync(); err != nil {
		return 0, fmt.Errorf("flush session %q: %w", s.file.Name(), err)
	}
	if s.saved == "" {
		s.header = sessionHeader(addition)
	}
	s.saved += addition
	return strings.Count(addition, "\n"), nil
}

func (s *sessionFile) close() {
	if err := s.file.Close(); err != nil {
		// Every append is already synced, so nothing is lost; just note it.
		slog.Warn("close chat session", "path", s.file.Name(), "error", err)
	}
}
