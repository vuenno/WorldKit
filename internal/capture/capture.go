// Package capture keeps the chat archive in sync with GTA World's chat log while WorldKit runs.
//
// The game rewrites its whole log on every new line and wipes it on reconnect, so the
// service polls FiveM's data folder and saves new lines before they can be lost.
package capture

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"world-kit/internal/chatlog"
	"world-kit/internal/fivem"
)

const pollInterval = 3 * time.Second

// State is what the capture service is currently doing.
type State int

// Capture states.
const (
	StateStarting     State = iota
	StateNeedsDataDir       // FiveM's data folder was not found; the user must pick it
	StateWatching           // waiting for and saving new chat lines
	StateArchiveInUse       // another WorldKit is already capturing
	StateFailed             // the archive could not be opened; see Status.Err
)

// Status is a snapshot of the capture service for display.
type Status struct {
	State      State
	DataDir    string    // FiveM's kvs folder being watched
	ArchiveDir string    // where sessions are saved
	LastSaved  time.Time // when new lines were last saved; zero if none this run
	LinesSaved int       // lines saved since WorldKit started
	Err        error     // the latest problem, nil when healthy
}

// Options configures a Service.
type Options struct {
	ArchiveDir string
	DataDir    string // user-chosen FiveM data folder; empty means the default location
	OnChange   func() // called from the service goroutine whenever Status changes
}

// Service watches FiveM's data folder and saves new chat lines to the archive.
type Service struct {
	onChange       func()
	dataDirChanges chan string // owned by Run: only it reads and applies data folder changes

	mu     sync.Mutex
	status Status
}

// New creates a Service; call Run to start capturing.
func New(options Options) *Service {
	onChange := options.OnChange
	if onChange == nil {
		onChange = func() {}
	}
	service := &Service{
		onChange:       onChange,
		dataDirChanges: make(chan string, 1),
		status:         Status{ArchiveDir: options.ArchiveDir},
	}
	service.dataDirChanges <- options.DataDir
	return service
}

// Status returns the current status. Safe to call from any goroutine.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// SetDataDir checks that path holds FiveM's data and starts watching it.
// It returns the resolved kvs folder to remember for next launch.
func (s *Service) SetDataDir(path string) (string, error) {
	dir, err := fivem.ResolveKVSDir(path)
	if err != nil {
		return "", err
	}
	s.updateStatus(func(status *Status) {
		status.DataDir = dir
		status.State = StateWatching
		status.Err = nil
	})
	// Replace any change Run has not picked up yet; only the latest one matters.
	select {
	case <-s.dataDirChanges:
	default:
	}
	s.dataDirChanges <- dir
	return dir, nil
}

// Run captures until ctx is cancelled. Only one Run may be active per Service.
func (s *Service) Run(ctx context.Context) {
	archive, err := chatlog.OpenArchive(s.Status().ArchiveDir)
	if err != nil {
		s.reportArchiveError(err)
		return
	}
	defer archive.Close()

	watcher := &watcher{service: s, archive: archive}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case dir := <-s.dataDirChanges:
			watcher.watch(dir)
		case <-ticker.C:
		}
		watcher.poll()
	}
}

func (s *Service) reportArchiveError(err error) {
	state := StateFailed
	if errors.Is(err, chatlog.ErrArchiveInUse) {
		state = StateArchiveInUse
	}
	slog.Error("open chat archive", "error", err)
	s.updateStatus(func(status *Status) {
		status.State = state
		status.Err = err
	})
}

func (s *Service) updateStatus(change func(*Status)) {
	s.mu.Lock()
	change(&s.status)
	s.mu.Unlock()
	s.onChange()
}

// watcher holds the state only Run's goroutine touches.
type watcher struct {
	service   *Service
	archive   *chatlog.Archive
	dataDir   string
	lastStamp fivem.Stamp
}

// watch switches to dir, or to the default location when dir is empty or invalid.
func (w *watcher) watch(dir string) {
	resolved, err := fivem.ResolveKVSDir(dir)
	if dir == "" || err != nil {
		resolved = findDefaultDataDir()
	}
	w.dataDir = resolved
	w.lastStamp = fivem.Stamp{}
	state := StateWatching
	if resolved == "" {
		state = StateNeedsDataDir
	}
	w.service.updateStatus(func(status *Status) {
		status.DataDir = resolved
		status.State = state
	})
}

// poll saves new chat lines if FiveM wrote to its data since the last poll.
func (w *watcher) poll() {
	if w.dataDir == "" {
		// FiveM may get installed or run for the first time while WorldKit is open.
		if dir := findDefaultDataDir(); dir != "" {
			w.watch(dir)
		}
		return
	}
	stamp, err := fivem.ReadStamp(w.dataDir)
	if err != nil || stamp == w.lastStamp {
		w.reportProblem(err)
		return
	}
	text, err := fivem.ReadChatLog(w.dataDir)
	if errors.Is(err, fivem.ErrNoChatLog) {
		w.lastStamp = stamp
		w.reportProblem(nil)
		return
	}
	if err != nil {
		// Usually FiveM writing at the same moment; lastStamp stays, so the next poll retries.
		w.reportProblem(err)
		return
	}
	w.save(text, stamp)
}

func (w *watcher) save(text string, stamp fivem.Stamp) {
	lines, err := w.archive.Save(text, time.Now().UTC())
	if err != nil {
		w.reportProblem(err)
		return
	}
	w.lastStamp = stamp
	if lines == 0 {
		w.reportProblem(nil)
		return
	}
	w.service.updateStatus(func(status *Status) {
		status.LastSaved = time.Now()
		status.LinesSaved += lines
		status.Err = nil
	})
}

// reportProblem records err (or clears it when nil) without stopping capture.
func (w *watcher) reportProblem(err error) {
	if err != nil {
		slog.Warn("capture chat log", "error", err)
	}
	if w.service.Status().Err == nil && err == nil {
		return
	}
	w.service.updateStatus(func(status *Status) { status.Err = err })
}

func findDefaultDataDir() string {
	dir, err := fivem.FindKVSDir()
	if err != nil {
		return ""
	}
	return dir
}
