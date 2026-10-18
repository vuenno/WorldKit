package diagnostics

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const (
	crashHelperEnv     = "WORLDKIT_DIAGNOSTICS_CRASH_DIR"
	crashHelperKindEnv = "WORLDKIT_DIAGNOSTICS_CRASH_KIND"
)

// TestCrashHelper is not a real test: TestPanicIsSavedToCrashLog runs it in a child process,
// where it starts a session and panics in a goroutine, like a crash in the app would.
func TestCrashHelper(t *testing.T) {
	dir := os.Getenv(crashHelperEnv)
	if dir == "" {
		t.Skip("only runs as a child process")
	}
	session, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv(crashHelperKindEnv) == "run" {
		runAndPanic(session)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		panic("boom from a background goroutine")
	}()
	<-done
}

// runAndPanic stands in for the app's run function, which defers End.
func runAndPanic(session *Session) (runErr error) {
	defer session.End(&runErr)
	panic("boom from the run function")
}

func TestPanicIsSavedToCrashLog(t *testing.T) {
	tests := []struct {
		kind, wantMessage string
	}{
		{"goroutine", "boom from a background goroutine"},
		// The app's run function defers End; a panic through it must still reach the file.
		{"run", "boom from the run function"},
	}
	for _, test := range tests {
		t.Run(test.kind, func(t *testing.T) {
			dir := t.TempDir()
			child := exec.Command(os.Args[0], "-test.run=^TestCrashHelper$")
			child.Env = append(os.Environ(), crashHelperEnv+"="+dir, crashHelperKindEnv+"="+test.kind)
			if err := child.Run(); err == nil {
				t.Fatal("child exited cleanly, want a crash")
			}
			crashFiles, _ := filepath.Glob(filepath.Join(dir, crashGlob))
			if len(crashFiles) != 1 {
				t.Fatalf("crash files = %v, want 1", crashFiles)
			}
			crash, err := os.ReadFile(crashFiles[0])
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(crash), test.wantMessage) {
				t.Errorf("crash log = %q, want %q in it", crash, test.wantMessage)
			}
		})
	}
}

func TestExitLeavesNoCrashFileAndLogsTheError(t *testing.T) {
	dir := t.TempDir()
	session, err := Start(dir)
	if err != nil {
		t.Fatal(err)
	}
	session.Close(errors.New("window closed: GPU failed"))
	if crashFiles, _ := filepath.Glob(filepath.Join(dir, crashGlob)); len(crashFiles) != 0 {
		t.Errorf("crash files = %v, want none", crashFiles)
	}
	appLog, err := os.ReadFile(filepath.Join(dir, appLogName))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"WorldKit started", "window closed: GPU failed"} {
		if !strings.Contains(string(appLog), want) {
			t.Errorf("app log = %q, want %q in it", appLog, want)
		}
	}
}
