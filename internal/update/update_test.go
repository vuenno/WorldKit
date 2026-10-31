package update

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"world-kit/internal/settings"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		text    string
		want    Version
		wantErr bool
	}{
		{text: "1.2.3", want: Version{Major: 1, Minor: 2, Patch: 3}},
		{text: "v0.10.0", want: Version{Minor: 10}},
		{text: " v2.0.1 ", want: Version{Major: 2, Patch: 1}},
		{text: "1.0.0-beta.2", want: Version{Major: 1, Prerelease: "beta.2"}},
		{text: "1.0.0+build.7", want: Version{Major: 1}},
		{text: "1.0.0-rc.1+build", want: Version{Major: 1, Prerelease: "rc.1"}},
		{text: "dev", wantErr: true},
		{text: "", wantErr: true},
		{text: "1.2", wantErr: true},
		{text: "1.2.3.4", wantErr: true},
		{text: "01.2.3", wantErr: true},
		{text: "1.2.x", wantErr: true},
		{text: "1.2.-3", wantErr: true},
		{text: "1.2.3-", wantErr: true},
		{text: "1.2.3-beta..1", wantErr: true},
	}
	for _, test := range tests {
		got, err := ParseVersion(test.text)
		if test.wantErr {
			if !errors.Is(err, ErrNotVersion) {
				t.Errorf("ParseVersion(%q) error = %v, want ErrNotVersion", test.text, err)
			}
			continue
		}
		if err != nil || got != test.want {
			t.Errorf("ParseVersion(%q) = %+v, %v; want %+v", test.text, got, err, test.want)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"1.2.3", "1.2.3", 0},
		{"v1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.3.0", "1.2.9", 1},
		{"2.0.0", "1.99.99", 1},
		{"0.9.0", "0.10.0", -1},
		{"1.0.0", "1.0.0-rc.1", 1},
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
		{"1.0.0-beta.2", "1.0.0-beta.11", -1},
		{"1.0.0-rc.1", "1.0.0-beta.11", 1},
		{"1.0.0+a", "1.0.0+b", 0},
	}
	for _, test := range tests {
		a, b := mustVersion(t, test.a), mustVersion(t, test.b)
		if got := a.Compare(b); got != test.want {
			t.Errorf("%s.Compare(%s) = %d, want %d", test.a, test.b, got, test.want)
		}
		if got := b.Compare(a); got != -test.want {
			t.Errorf("%s.Compare(%s) = %d, want %d", test.b, test.a, got, -test.want)
		}
	}
}

func TestVersionString(t *testing.T) {
	for _, text := range []string{"1.2.3", "0.0.1-rc.1"} {
		if got := mustVersion(t, "v"+text).String(); got != text {
			t.Errorf("String() = %q, want %q", got, text)
		}
	}
}

func TestIsCheckDue(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		lastCheck time.Time
		want      bool
	}{
		{"never checked", time.Time{}, true},
		{"an hour ago", now.Add(-time.Hour), false},
		{"just under the interval", now.Add(-CheckInterval + time.Minute), false},
		{"a day ago", now.Add(-24 * time.Hour), true},
		{"clock moved back", now.Add(time.Hour), true},
	}
	for _, test := range tests {
		if got := IsCheckDue(test.lastCheck, now); got != test.want {
			t.Errorf("%s: IsCheckDue = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestShouldOffer(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	latest := Version{Minor: 3}
	tests := []struct {
		name     string
		deferral Deferral
		want     bool
	}{
		{"no answer yet", Deferral{}, true},
		{"this version skipped", Deferral{SkippedVersion: "0.3.0"}, false},
		{"an older version skipped", Deferral{SkippedVersion: "0.2.0"}, true},
		{"later, not yet due", Deferral{RemindAfter: now.Add(time.Hour)}, false},
		{"later, due", Deferral{RemindAfter: now.Add(-time.Minute)}, true},
		{"later, due right now", Deferral{RemindAfter: now}, true},
	}
	for _, test := range tests {
		if got := ShouldOffer(latest, test.deferral, now); got != test.want {
			t.Errorf("%s: ShouldOffer = %v, want %v", test.name, got, test.want)
		}
	}
}

func TestInstallerArgs(t *testing.T) {
	tests := []struct {
		name    string
		options InstallOptions
		want    []string
	}{
		{"defaults", InstallOptions{}, []string{"/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/CLOSEAPPLICATIONS", "/RELAUNCH=1"}},
		{"wait and log", InstallOptions{WaitForPID: 42, LogPath: `C:\a b\install.log`}, []string{
			"/VERYSILENT", "/SUPPRESSMSGBOXES", "/NORESTART", "/CLOSEAPPLICATIONS", "/RELAUNCH=1",
			"/WAITPID=42", `/LOG=C:\a b\install.log`,
		}},
	}
	for _, test := range tests {
		if got := installerArgs(test.options); !slices.Equal(got, test.want) {
			t.Errorf("%s: installerArgs = %q, want %q", test.name, got, test.want)
		}
	}
}

// fakeGitHub serves a latest release and its assets like api.github.com and github.com.
type fakeGitHub struct {
	server       *httptest.Server
	tag          string
	installer    []byte
	listedSize   int64  // the size the release lists; defaults to len(installer)
	checksum     string // the .sha256 file's content; defaults to the right one
	hasChecksum  bool
	notesURL     string // defaults to the release page
	installerURL string // defaults to the right download address
	status       int    // the API's status; defaults to 200
	requests     atomic.Int32
}

func newFakeGitHub(t *testing.T, tag string) *fakeGitHub {
	t.Helper()
	fake := &fakeGitHub{tag: tag, installer: []byte("MZ pretend installer"), hasChecksum: true}
	fake.server = httptest.NewServer(http.HandlerFunc(fake.serve))
	t.Cleanup(fake.server.Close)
	return fake
}

func (f *fakeGitHub) client() *Client {
	return NewClient(ClientOptions{CurrentVersion: "0.2.0", APIBaseURL: f.server.URL, SiteBaseURL: f.server.URL})
}

func (f *fakeGitHub) installerName() string { return "WorldKit-Setup-" + f.tag[1:] + ".exe" }

func (f *fakeGitHub) downloadURL(name string) string {
	return f.server.URL + "/" + Repository + "/releases/download/" + f.tag + "/" + name
}

func (f *fakeGitHub) serve(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/repos/" + Repository + "/releases/latest":
		f.requests.Add(1)
		f.serveRelease(w, r)
	case "/" + Repository + "/releases/download/" + f.tag + "/" + f.installerName():
		w.Write(f.installer)
	case "/" + Repository + "/releases/download/" + f.tag + "/" + f.installerName() + ".sha256":
		fmt.Fprint(w, f.checksumContent())
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeGitHub) serveRelease(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("User-Agent") == "" {
		http.Error(w, "no user agent", http.StatusForbidden)
		return
	}
	if f.status != 0 {
		http.Error(w, "unavailable", f.status)
		return
	}
	size := f.listedSize
	if size == 0 {
		size = int64(len(f.installer))
	}
	notes := cmp.Or(f.notesURL, f.server.URL+"/"+Repository+"/releases/tag/"+f.tag)
	assets := fmt.Sprintf(`{"name":%q,"size":%d,"browser_download_url":%q}`,
		f.installerName(), size, cmp.Or(f.installerURL, f.downloadURL(f.installerName())))
	if f.hasChecksum {
		assets += fmt.Sprintf(`,{"name":%q,"size":80,"browser_download_url":%q}`,
			f.installerName()+".sha256", f.downloadURL(f.installerName()+".sha256"))
	}
	fmt.Fprintf(w, `{"tag_name":%q,"html_url":%q,"assets":[%s]}`, f.tag, notes, assets)
}

func (f *fakeGitHub) checksumContent() string {
	if f.checksum != "" {
		return f.checksum
	}
	sum := sha256.Sum256(f.installer)
	return hex.EncodeToString(sum[:]) + "  " + f.installerName() + "\n"
}

func TestLatest(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*fakeGitHub)
		wantErr bool
	}{
		{name: "complete release"},
		{name: "no checksum", change: func(f *fakeGitHub) { f.hasChecksum = false }, wantErr: true},
		{name: "API error", change: func(f *fakeGitHub) { f.status = http.StatusForbidden }, wantErr: true},
		{name: "bad tag", change: func(f *fakeGitHub) { f.tag = "vnext" }, wantErr: true},
		{name: "foreign notes page", change: func(f *fakeGitHub) { f.notesURL = "https://example.com/notes" }, wantErr: true},
		{name: "foreign download", change: func(f *fakeGitHub) { f.installerURL = "https://example.com/setup.exe" }, wantErr: true},
	}
	for _, test := range tests {
		fake := newFakeGitHub(t, "v0.3.0")
		if test.change != nil {
			test.change(fake)
		}
		release, err := fake.client().Latest(context.Background())
		if test.wantErr {
			if err == nil {
				t.Errorf("%s: Latest succeeded, want an error", test.name)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: Latest: %v", test.name, err)
		}
		if release.Version != (Version{Minor: 3}) || release.Installer.Name != "WorldKit-Setup-0.3.0.exe" ||
			release.Installer.Size != int64(len(fake.installer)) || release.Checksum.URL == "" {
			t.Errorf("%s: Latest = %+v", test.name, release)
		}
	}
}

func TestDownload(t *testing.T) {
	tests := []struct {
		name    string
		change  func(*fakeGitHub)
		wantErr error
	}{
		{name: "matching file"},
		{name: "listed size differs", change: func(f *fakeGitHub) { f.listedSize = 5 }, wantErr: ErrCorruptDownload},
		{name: "wrong hash", change: func(f *fakeGitHub) { f.checksum = fmt.Sprintf("%064x", 0) }, wantErr: ErrCorruptDownload},
		{name: "checksum not hex", change: func(f *fakeGitHub) { f.checksum = "nope" }, wantErr: ErrCorruptDownload},
	}
	for _, test := range tests {
		fake := newFakeGitHub(t, "v0.3.0")
		if test.change != nil {
			test.change(fake)
		}
		client := fake.client()
		release, err := client.Latest(context.Background())
		if err != nil {
			t.Fatalf("%s: Latest: %v", test.name, err)
		}
		dir := filepath.Join(t.TempDir(), "update")
		path, err := client.Download(context.Background(), release, dir)
		if test.wantErr != nil {
			if !errors.Is(err, test.wantErr) {
				t.Errorf("%s: Download error = %v, want %v", test.name, err, test.wantErr)
			}
			if entries, _ := os.ReadDir(dir); len(entries) != 0 {
				t.Errorf("%s: a failed download left %d files", test.name, len(entries))
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: Download: %v", test.name, err)
		}
		content, err := os.ReadFile(path)
		if err != nil || string(content) != string(fake.installer) || filepath.Base(path) != release.Installer.Name {
			t.Errorf("%s: downloaded %q to %q (%v)", test.name, content, path, err)
		}
	}
}

// memorySettings is a settings store that keeps everything in memory.
type memorySettings struct {
	mu      sync.Mutex
	current settings.Settings
}

func (m *memorySettings) Get() settings.Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.current
}

func (m *memorySettings) Update(change func(*settings.Settings)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	change(&m.current)
	return nil
}

func TestServiceFindsNewerRelease(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		current   string
		tag       string
		wantState State
	}{
		{"newer release", "0.2.0", "v0.3.0", StateAvailable},
		{"same release", "0.3.0", "v0.3.0", StateUpToDate},
		{"older release", "0.4.0", "v0.3.0", StateUpToDate},
	}
	for _, test := range tests {
		fake := newFakeGitHub(t, test.tag)
		store := &memorySettings{}
		service := New(Options{CurrentVersion: test.current, Client: fake.client(), Settings: store, Now: func() time.Time { return now }})
		status := runUntilChecked(t, service)
		if status.State != test.wantState {
			t.Errorf("%s: state = %v, want %v", test.name, status.State, test.wantState)
		}
		if !store.Get().LastUpdateCheck.Equal(now) {
			t.Errorf("%s: LastUpdateCheck = %v, want %v", test.name, store.Get().LastUpdateCheck, now)
		}
		_, isOffered := service.Offer(now)
		if isOffered != (test.wantState == StateAvailable) {
			t.Errorf("%s: offered = %v", test.name, isOffered)
		}
	}
}

func TestServiceSkipAndLater(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	fake := newFakeGitHub(t, "v0.3.0")
	store := &memorySettings{}
	service := New(Options{CurrentVersion: "0.2.0", Client: fake.client(), Settings: store, Now: func() time.Time { return now }})
	runUntilChecked(t, service)
	release, _ := service.Offer(now)

	if err := service.RemindLater(now); err != nil {
		t.Fatal(err)
	}
	if _, ok := service.Offer(now.Add(time.Hour)); ok {
		t.Error("offered an hour after Later")
	}
	if _, ok := service.Offer(now.Add(RemindLaterDelay)); !ok {
		t.Error("not offered once Later's delay passed")
	}
	if err := service.Skip(release.Version); err != nil {
		t.Fatal(err)
	}
	if _, ok := service.Offer(now.Add(48 * time.Hour)); ok {
		t.Error("offered a skipped version")
	}
}

func TestServiceManualCheckForgetsSkip(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	fake := newFakeGitHub(t, "v0.3.0")
	// Checked recently and skipped: an automatic check isn't due.
	store := &memorySettings{current: settings.Settings{LastUpdateCheck: now.Add(-time.Hour), SkippedUpdate: "0.3.0"}}
	service := New(Options{CurrentVersion: "0.2.0", Client: fake.client(), Settings: store, Now: func() time.Time { return now }})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go service.Run(ctx)
	service.CheckNow()
	status := waitForStatus(t, service)
	if status.State != StateAvailable || fake.requests.Load() != 1 {
		t.Fatalf("state = %v after %d requests, want available after 1", status.State, fake.requests.Load())
	}
	if _, ok := service.Offer(now); !ok || store.Get().SkippedUpdate != "" {
		t.Errorf("a manual check didn't offer the skipped version (skipped = %q)", store.Get().SkippedUpdate)
	}
}

func TestServiceRespectsSettingsAndDevBuilds(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		current   string
		settings  settings.Settings
		wantState State
	}{
		{"dev build", "dev", settings.Settings{}, StateDevBuild},
		{"checks turned off", "0.2.0", settings.Settings{IsUpdateCheckOff: true}, StateNotChecked},
		{"checked an hour ago", "0.2.0", settings.Settings{LastUpdateCheck: now.Add(-time.Hour)}, StateNotChecked},
	}
	for _, test := range tests {
		fake := newFakeGitHub(t, "v9.0.0")
		store := &memorySettings{current: test.settings}
		service := New(Options{CurrentVersion: test.current, Client: fake.client(), Settings: store, Now: func() time.Time { return now }})
		ctx, cancel := context.WithCancel(context.Background())
		stopped := make(chan struct{})
		go func() {
			service.Run(ctx)
			close(stopped)
		}()
		time.Sleep(50 * time.Millisecond) // long enough for a due check to have started
		cancel()
		<-stopped
		if got := service.Status().State; got != test.wantState || fake.requests.Load() != 0 {
			t.Errorf("%s: state = %v after %d requests, want %v and none", test.name, got, fake.requests.Load(), test.wantState)
		}
	}
}

// runUntilChecked runs service's startup check and returns its status.
func runUntilChecked(t *testing.T, service *Service) Status {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go service.Run(ctx)
	return waitForStatus(t, service)
}

func waitForStatus(t *testing.T, service *Service) Status {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		status := service.Status()
		if status.State != StateNotChecked && status.State != StateChecking {
			return status
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("no check finished in time")
	return Status{}
}

func mustVersion(t *testing.T, text string) Version {
	t.Helper()
	version, err := ParseVersion(text)
	if err != nil {
		t.Fatal(err)
	}
	return version
}
