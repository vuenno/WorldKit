package ui

import (
	"errors"
	"testing"
	"time"

	"world-kit/internal/capture"
)

func TestCaptureStatusText(t *testing.T) {
	tests := []struct {
		name   string
		status capture.Status
		want   string
	}{
		{"needs folder", capture.Status{State: capture.StateNeedsDataDir}, "FiveM data folder not found. Chat isn't being saved."},
		{"second instance", capture.Status{State: capture.StateArchiveInUse}, "Another WorldKit is already running and saving chat."},
		{"watching", capture.Status{State: capture.StateWatching}, "Watching for GTA World chat."},
		{"retrying", capture.Status{State: capture.StateWatching, Err: errors.New("busy")}, "Couldn't read FiveM data just now. Retrying…"},
		{
			"saved lines",
			capture.Status{State: capture.StateWatching, LinesSaved: 12, LastSaved: time.Date(2026, 10, 18, 9, 57, 1, 0, time.Local)},
			"Saved 12 new lines, latest at 09:57:01.",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := captureStatusText(test.status); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
