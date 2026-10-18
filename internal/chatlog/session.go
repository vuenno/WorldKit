package chatlog

import (
	"regexp"
	"strings"
	"time"
)

// sessionHeaderPattern matches the line the game writes when a session starts, e.g.
// "[DATE: 18/OCT/2026 | TIME: 09:57:01]". The game's clock is UTC.
var sessionHeaderPattern = regexp.MustCompile(`^\[DATE: (\d{2}/[A-Za-z]{3}/\d{4}) \| TIME: (\d{2}:\d{2}:\d{2})\]$`)

// sessionHeader returns the first line of text if it is a session header, otherwise "".
func sessionHeader(text string) string {
	firstLine, _, _ := strings.Cut(text, "\n")
	if !sessionHeaderPattern.MatchString(firstLine) {
		return ""
	}
	return firstLine
}

// parseSessionHeader returns the UTC start time written in a session header line.
func parseSessionHeader(line string) (time.Time, bool) {
	match := sessionHeaderPattern.FindStringSubmatch(line)
	if match == nil {
		return time.Time{}, false
	}
	start, err := time.Parse("02/Jan/2006 15:04:05", match[1]+" "+match[2])
	if err != nil {
		return time.Time{}, false
	}
	return start, true
}
