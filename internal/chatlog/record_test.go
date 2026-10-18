package chatlog

import (
	"strings"
	"testing"
	"time"
)

func TestParseClassifiesLines(t *testing.T) {
	tests := []struct {
		line        string
		wantKind    Kind
		wantSpeaker string
		wantText    string
	}{
		{"[DATE: 18/OCT/2026 | TIME: 09:57:01]", KindHeader, "", "[DATE: 18/OCT/2026 | TIME: 09:57:01]"},
		{"[09:57:01] Welcome to GTA World.", KindUnknown, "", "Welcome to GTA World."},
		{"[09:57:10] [INFO] You do not have a valid ID card.", KindInfo, "", "You do not have a valid ID card."},
		{"[10:51:22] (( PM from (169) John Doe: see you there ))", KindPM, "John Doe", "see you there"},
		{"[10:51:30] (( PM to (169) John Doe: ok ))", KindPM, "John Doe", "ok"},
		{"(( John Doe: brb ))", KindOOC, "John Doe", "brb"},
		{"John Doe says: hi there", KindSay, "John Doe", "hi there"},
		{"John Doe says [low]: quiet", KindSay, "John Doe", "quiet"},
		{"John Doe shouts: hey!", KindShout, "John Doe", "hey!"},
		{"John Doe whispers: psst", KindWhisper, "John Doe", "psst"},
		{"* John Doe waves.", KindMe, "John Doe", "waves."},
		{"* The door is locked. (( John Doe ))", KindDo, "John Doe", "The door is locked."},
		{"[S: 1 | CH: 911] John Doe says: copy", KindUnknown, "", "[S: 1 | CH: 911] John Doe says: copy"},
	}
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			records, err := Parse(strings.NewReader(test.line))
			if err != nil {
				t.Fatal(err)
			}
			got := records[0]
			if got.Kind != test.wantKind || got.Speaker != test.wantSpeaker || got.Text != test.wantText {
				t.Errorf("got kind %d speaker %q text %q, want kind %d speaker %q text %q",
					got.Kind, got.Speaker, got.Text, test.wantKind, test.wantSpeaker, test.wantText)
			}
			if got.Raw != test.line {
				t.Errorf("Raw = %q, want %q", got.Raw, test.line)
			}
		})
	}
}

func TestParseTimesRollOverMidnight(t *testing.T) {
	session := strings.Join([]string{
		"[DATE: 18/OCT/2026 | TIME: 23:59:50]",
		"[23:59:58] before midnight",
		"John Doe says: untimed",
		"[00:00:03] after midnight",
	}, "\n")
	records, err := Parse(strings.NewReader(session))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		time    time.Time
		hasTime bool
	}{
		{time.Date(2026, 10, 18, 23, 59, 50, 0, time.UTC), true},
		{time.Date(2026, 10, 18, 23, 59, 58, 0, time.UTC), true},
		{time.Date(2026, 10, 18, 23, 59, 58, 0, time.UTC), false},
		{time.Date(2026, 10, 19, 0, 0, 3, 0, time.UTC), true},
	}
	for i, record := range records {
		if !record.Time.Equal(want[i].time) || record.HasTime != want[i].hasTime || record.Line != i+1 {
			t.Errorf("line %d: got %v hasTime=%v line=%d, want %v hasTime=%v",
				i+1, record.Time, record.HasTime, record.Line, want[i].time, want[i].hasTime)
		}
	}
}

func TestRawWithoutTimestamp(t *testing.T) {
	tests := []struct{ line, want string }{
		{"[09:57:12] * Jane Roe waves.", "* Jane Roe waves."},
		{"[09:57:15] (( Jane Roe: brb ))", "(( Jane Roe: brb ))"},
		{"John Doe says: untimed", "John Doe says: untimed"},
		{"[DATE: 18/OCT/2026 | TIME: 09:57:01]", "[DATE: 18/OCT/2026 | TIME: 09:57:01]"},
		{"[INFO] [09:57:12] not a leading timestamp", "[INFO] [09:57:12] not a leading timestamp"},
	}
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			records, err := Parse(strings.NewReader(test.line))
			if err != nil {
				t.Fatal(err)
			}
			if got := records[0].RawWithoutTimestamp(); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
