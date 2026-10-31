package update

import "time"

const (
	// CheckInterval is how often GitHub is asked, at most: about once a day.
	CheckInterval = 20 * time.Hour
	// RetryInterval is how soon a failed check (offline, GitHub down) is tried again.
	RetryInterval = time.Hour
	// RemindLaterDelay is how long "Later" hides an offered update.
	RemindLaterDelay = 24 * time.Hour
)

// IsCheckDue reports whether an automatic check should run now, given when the last
// one succeeded. A last check in the future (the clock moved back) counts as due.
func IsCheckDue(lastCheck, now time.Time) bool {
	return lastCheck.IsZero() || now.Sub(lastCheck) >= CheckInterval || now.Before(lastCheck)
}

// Deferral is what the user answered to earlier offers: a version to skip, or a time
// before which not to ask again.
type Deferral struct {
	SkippedVersion string
	RemindAfter    time.Time
}

// ShouldOffer reports whether to show the user an update to latest now.
func ShouldOffer(latest Version, deferral Deferral, now time.Time) bool {
	if deferral.SkippedVersion == latest.String() {
		return false
	}
	return !now.Before(deferral.RemindAfter)
}
