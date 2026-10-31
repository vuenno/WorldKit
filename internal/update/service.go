package update

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"world-kit/internal/settings"
)

// wakeInterval is how often the service looks at the clock and the settings to see
// whether a check is due. Checks themselves are much rarer (CheckInterval).
const wakeInterval = time.Hour

// SettingsStore reads and saves the user's preferences.
type SettingsStore interface {
	Get() settings.Settings
	Update(change func(*settings.Settings)) error
}

// State says what the service last found out.
type State int

const (
	StateNotChecked State = iota // nothing checked yet in this run
	StateChecking
	StateUpToDate
	StateAvailable // Status.Latest is newer than this build
	StateFailed    // the last check failed (logged); retried later
	StateDevBuild  // no version stamped: updates are off
)

// Status is a snapshot of the service for the UI.
type Status struct {
	State  State
	Latest Release // set while a newer release is known, whatever State says
	// HasNewer is true while Latest is newer than this build (also during a re-check).
	HasNewer bool
}

// Options configure a Service.
type Options struct {
	CurrentVersion string // appVersion; anything that isn't a version means a dev build
	Client         *Client
	Settings       SettingsStore
	DownloadDir    string // a folder of WorldKit's own under %TEMP%; emptied before each download
	InstallLogPath string // where the installer writes its log
	OnChange       func() // called when Status changes (redraws the window)
	Now            func() time.Time
}

// Service checks for updates in the background (Run), and downloads and starts the
// installer when asked (Install). Safe for concurrent use.
type Service struct {
	options       Options
	current       Version
	isDevBuild    bool
	checkRequests chan struct{}

	mu         sync.Mutex
	isChecking bool
	hasChecked bool
	lastErr    error
	failedAt   time.Time
	latest     *Release // newer than current; nil when none is known
}

// New makes the service. A CurrentVersion that isn't a version (a local "dev" build)
// gives a service that never checks.
func New(options Options) *Service {
	if options.Now == nil {
		options.Now = time.Now
	}
	if options.OnChange == nil {
		options.OnChange = func() {}
	}
	current, err := ParseVersion(options.CurrentVersion)
	return &Service{
		options:       options,
		current:       current,
		isDevBuild:    err != nil,
		checkRequests: make(chan struct{}, 1),
	}
}

// Run checks whenever a check is due, or asked for with CheckNow, until ctx ends.
func (s *Service) Run(ctx context.Context) {
	if s.isDevBuild {
		return
	}
	s.removeOldDownloads()
	isManual := false
	for {
		if isManual || s.isAutomaticCheckDue() {
			s.check(ctx, isManual)
		}
		timer := time.NewTimer(wakeInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-s.checkRequests:
			isManual = true
		case <-timer.C:
			isManual = false
		}
		timer.Stop()
	}
}

// CheckNow asks Run to check right away (Settings' Check now button). An update found
// this way is offered even if the user skipped it or said Later.
func (s *Service) CheckNow() {
	if s.isDevBuild {
		return
	}
	s.mu.Lock()
	s.isChecking = true // shown at once, before Run picks the request up
	s.mu.Unlock()
	s.options.OnChange()
	select {
	case s.checkRequests <- struct{}{}:
	default: // a check is already queued
	}
}

// Status returns what the service knows now.
func (s *Service) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	var status Status
	if s.latest != nil {
		status.Latest, status.HasNewer = *s.latest, true
	}
	switch {
	case s.isDevBuild:
		status.State = StateDevBuild
	case s.isChecking:
		status.State = StateChecking
	case s.latest != nil:
		status.State = StateAvailable
	case s.lastErr != nil:
		status.State = StateFailed
	case s.hasChecked:
		status.State = StateUpToDate
	}
	return status
}

// Offer returns the release to show the user now, unless they skipped it or asked to
// be reminded later.
func (s *Service) Offer(now time.Time) (Release, bool) {
	status := s.Status()
	if !status.HasNewer {
		return Release{}, false
	}
	current := s.options.Settings.Get()
	deferral := Deferral{SkippedVersion: current.SkippedUpdate, RemindAfter: current.RemindUpdateAfter}
	return status.Latest, ShouldOffer(status.Latest.Version, deferral, now)
}

// RemindLater hides the offer for RemindLaterDelay.
func (s *Service) RemindLater(now time.Time) error {
	return s.saveDeferral(func(current *settings.Settings) { current.RemindUpdateAfter = now.Add(RemindLaterDelay) })
}

// Skip stops offering version; a later one is offered again.
func (s *Service) Skip(version Version) error {
	return s.saveDeferral(func(current *settings.Settings) { current.SkippedUpdate = version.String() })
}

func (s *Service) saveDeferral(change func(*settings.Settings)) error {
	if err := s.options.Settings.Update(change); err != nil {
		return fmt.Errorf("remember update answer: %w", err)
	}
	s.options.OnChange()
	return nil
}

// Install downloads release's installer, checks it and starts it. On success the caller
// must quit WorldKit at once: the installer waits for this process to exit. It blocks for
// the download, so call it off the UI goroutine.
func (s *Service) Install(release Release) error {
	path, err := s.options.Client.Download(context.Background(), release, s.options.DownloadDir)
	if err != nil {
		return err
	}
	slog.Info("starting update installer", "version", release.Version.String(), "path", path)
	return StartInstaller(path, InstallOptions{WaitForPID: os.Getpid(), LogPath: s.options.InstallLogPath})
}

func (s *Service) isAutomaticCheckDue() bool {
	if s.options.Settings.Get().IsUpdateCheckOff {
		return false
	}
	now := s.options.Now()
	s.mu.Lock()
	failedAt := s.failedAt
	s.mu.Unlock()
	if !failedAt.IsZero() && now.Sub(failedAt) < RetryInterval {
		return false
	}
	return IsCheckDue(s.options.Settings.Get().LastUpdateCheck, now)
}

// check asks GitHub once. Failures are only logged: the user is never bothered by them.
func (s *Service) check(ctx context.Context, isManual bool) {
	s.setChecking()
	release, err := s.options.Client.Latest(ctx)
	if errors.Is(err, context.Canceled) {
		s.finishCheck(nil, err, time.Time{})
		return
	}
	now := s.options.Now()
	if err != nil {
		slog.Warn("check for updates", "error", err)
		s.finishCheck(nil, err, now)
		return
	}
	if err := s.options.Settings.Update(func(current *settings.Settings) { s.recordCheck(current, release, now, isManual) }); err != nil {
		slog.Warn("remember update check", "error", err)
	}
	var newer *Release
	if release.Version.IsNewerThan(s.current) {
		newer = &release
		slog.Info("update available", "current", s.current.String(), "latest", release.Version.String())
	}
	s.finishCheck(newer, nil, time.Time{})
}

// recordCheck notes a successful check; a manual one that finds an update also forgets
// earlier Skip and Later answers, since the user just asked.
func (s *Service) recordCheck(current *settings.Settings, release Release, now time.Time, isManual bool) {
	current.LastUpdateCheck = now
	if isManual && release.Version.IsNewerThan(s.current) {
		current.SkippedUpdate = ""
		current.RemindUpdateAfter = time.Time{}
	}
}

func (s *Service) setChecking() {
	s.mu.Lock()
	s.isChecking = true
	s.mu.Unlock()
	s.options.OnChange()
}

// finishCheck stores a check's outcome. On failure a known newer release is kept, so a
// later offline check doesn't hide it.
func (s *Service) finishCheck(newer *Release, err error, failedAt time.Time) {
	s.mu.Lock()
	s.isChecking = false
	s.lastErr = err
	s.failedAt = failedAt
	if err == nil {
		s.hasChecked = true
		s.latest = newer
	}
	s.mu.Unlock()
	s.options.OnChange()
}

// removeOldDownloads deletes an installer left from an earlier update. It may still be
// running (it starts the new WorldKit just before it exits), so failing is fine.
func (s *Service) removeOldDownloads() {
	if s.options.DownloadDir == "" {
		return
	}
	if err := os.RemoveAll(s.options.DownloadDir); err != nil {
		slog.Debug("remove old update download", "error", err)
	}
}
