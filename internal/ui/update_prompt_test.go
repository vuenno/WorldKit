package ui

import (
	"testing"

	"world-kit/internal/update"
)

func TestUpdateStatusText(t *testing.T) {
	newer := update.Release{Version: update.Version{Minor: 3}}
	tests := []struct {
		name           string
		status         update.Status
		isAutomaticOff bool
		want           string
	}{
		{"dev build", update.Status{State: update.StateDevBuild}, false, "Development build: updates are off."},
		{"checking", update.Status{State: update.StateChecking}, false, "Checking for updates…"},
		{"available", update.Status{State: update.StateAvailable, Latest: newer, HasNewer: true}, false, "Version 0.3.0 is available."},
		{"up to date", update.Status{State: update.StateUpToDate}, false, "You have the latest version."},
		{"failed", update.Status{State: update.StateFailed}, false, "Couldn't reach GitHub. Try again later."},
		{"not checked yet", update.Status{}, false, "WorldKit checks once a day."},
		{"automatic checks off", update.Status{}, true, "Automatic checks are off."},
	}
	for _, test := range tests {
		if got := updateStatusText(test.status, test.isAutomaticOff); got != test.want {
			t.Errorf("%s: updateStatusText = %q, want %q", test.name, got, test.want)
		}
	}
}
