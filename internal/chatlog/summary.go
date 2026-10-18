package chatlog

import "time"

// Summary describes a session at a glance: who was in it, how long it ran and what kind
// of chat it held.
type Summary struct {
	Lines      int
	Characters []Character // everyone who spoke, most active first
	Says       int         // says, shouts, whispers, server and other lines
	Emotes     int         // /me and /do
	OOC        int         // /b
	PMs        int
	First      time.Time // the first timestamp; zero when the session has none
	Last       time.Time // the last timestamp
}

// Duration returns how long the session ran, from its first to its last timestamp.
func (s Summary) Duration() time.Duration {
	if s.First.IsZero() || s.Last.Before(s.First) {
		return 0
	}
	return s.Last.Sub(s.First)
}

// Summarize counts records by kind and finds the session's characters and time span. The
// session header is not counted as a line.
func Summarize(records []Record) Summary {
	summary := Summary{Characters: Characters(records)}
	for _, record := range records {
		if record.HasTime {
			if summary.First.IsZero() {
				summary.First = record.Time
			}
			summary.Last = record.Time
		}
		if record.Kind == KindHeader {
			continue
		}
		summary.Lines++
		switch record.Kind {
		case KindMe, KindDo:
			summary.Emotes++
		case KindOOC:
			summary.OOC++
		case KindPM:
			summary.PMs++
		default:
			summary.Says++
		}
	}
	return summary
}
