package ui

import (
	"errors"
	"testing"
	"time"
)

func TestBackgroundLoadRerunsOutdatedRead(t *testing.T) {
	loader := newBackgroundLoad[int](func() {})
	release := make(chan struct{})
	loader.Start(func() (int, error) { <-release; return 1, nil })
	loader.Start(func() (int, error) { return 2, errors.New("second") })
	close(release)

	result := waitForResult(t, loader)
	if result.value != 2 || result.err == nil {
		t.Errorf("got %+v, want the second read's result", result)
	}
	if loader.IsLoading() {
		t.Error("still loading after the result arrived")
	}
}

func waitForResult(t *testing.T, loader *backgroundLoad[int]) loadResult[int] {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if result, ok := loader.Result(); ok {
			return result
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("no result within a second")
	return loadResult[int]{}
}
