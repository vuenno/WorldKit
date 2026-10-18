package ui

import (
	"testing"
	"time"

	"world-kit/internal/chatlog"
)

func TestDayTexts(t *testing.T) {
	now := time.Date(2026, 10, 18, 14, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		t         time.Time
		wantGroup string
		wantDay   string
	}{
		{"today", time.Date(2026, 10, 18, 9, 57, 0, 0, time.UTC), "Today", "Today, 09:57 UTC"},
		{"yesterday", time.Date(2026, 10, 17, 21, 14, 0, 0, time.UTC), "Yesterday", "Yesterday, 21:14 UTC"},
		{"this week", time.Date(2026, 10, 15, 19, 30, 0, 0, time.UTC), "Earlier this week", "Thu 15 Oct, 19:30 UTC"},
		{"older", time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC), "September 2026", "Wed 02 Sep, 08:00 UTC"},
		{"last year", time.Date(2025, 12, 31, 23, 0, 0, 0, time.UTC), "December 2025", "Wed 31 Dec 2025, 23:00 UTC"},
		{"local time counts as UTC", time.Date(2026, 10, 18, 1, 0, 0, 0, time.FixedZone("GST", 4*60*60)), "Yesterday", "Yesterday, 21:00 UTC"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := dayGroupLabel(test.t, now); got != test.wantGroup {
				t.Errorf("dayGroupLabel = %q, want %q", got, test.wantGroup)
			}
			if got := dayAndTimeText(test.t, now); got != test.wantDay {
				t.Errorf("dayAndTimeText = %q, want %q", got, test.wantDay)
			}
		})
	}
}

func TestDurationText(t *testing.T) {
	tests := map[time.Duration]string{
		20 * time.Second:             "< 1 min",
		48 * time.Minute:             "48 min",
		2*time.Hour + 5*time.Minute:  "2 h 05 min",
		3*time.Hour + 40*time.Minute: "3 h 40 min",
	}
	for d, want := range tests {
		if got := durationText(d); got != want {
			t.Errorf("durationText(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestCountText(t *testing.T) {
	tests := map[int]string{0: "0", 999: "999", 1284: "1,284", 1234567: "1,234,567"}
	for n, want := range tests {
		if got := countText(n); got != want {
			t.Errorf("countText(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestCharactersText(t *testing.T) {
	characters := []chatlog.Character{{Name: "John Doe"}, {Name: "Jane Roe"}, {Name: "Marcus Hale"}, {Name: "Dr. Patel"}}
	if got, want := charactersText(characters, 2), "John Doe, Jane Roe +2"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := charactersText(characters[:1], 3), "John Doe"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := charactersText(nil, 3), "No one spoke"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
