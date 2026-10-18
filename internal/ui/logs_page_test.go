package ui

import (
	"errors"
	"testing"
	"time"

	"world-kit/internal/chatlog"
)

func TestSessionTexts(t *testing.T) {
	session := chatlog.Session{
		Path:    `C:\logs\2026\10\2026-10-18_095701.txt`,
		Number:  12,
		Started: time.Date(2026, 10, 18, 9, 57, 1, 0, time.UTC),
		Updated: time.Date(2026, 10, 18, 14, 42, 0, 0, time.FixedZone("CEST", 2*60*60)),
	}
	tests := []struct {
		name string
		got  string
		want string
	}{
		{"title", sessionTitle(session), "Sun 18 Oct 2026, 09:57 UTC"},
		{"title without start", sessionTitle(chatlog.Session{Path: session.Path}), "2026-10-18_095701.txt"},
		{"updated in UTC", sessionUpdatedText(session), "Last updated Sun 18 Oct 2026, 12:42 UTC"},
		{"number", sessionNumberText(session), "Session #12"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.got != test.want {
				t.Errorf("got %q, want %q", test.got, test.want)
			}
		})
	}
}

func TestChatLinesKeepRawText(t *testing.T) {
	records := []chatlog.Record{{Raw: "[09:57:03] * Jane Roe waves."}, {Raw: "(( Jane Roe: brb ))"}}
	want := "[09:57:03] * Jane Roe waves.\n(( Jane Roe: brb ))"
	if got := joinChatLines(newChatLines(records)); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

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
