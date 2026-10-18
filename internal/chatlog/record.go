package chatlog

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// Kind is the type of a chat line, recognised from its text.
type Kind int

// Chat line kinds. Lines that match no rule are KindUnknown and keep their raw text.
const (
	KindUnknown Kind = iota
	KindHeader       // "[DATE: 18/OCT/2026 | TIME: 09:57:01]"
	KindSay          // "John Doe says: hi" (also "says [low]:")
	KindShout        // "John Doe shouts: hey!"
	KindWhisper      // "John Doe whispers: psst"
	KindMe           // "* John Doe waves."
	KindDo           // "* The door is locked. (( John Doe ))"
	KindOOC          // "(( John Doe: brb ))"
	KindPM           // "(( PM from (169) John Doe: hi ))"; Speaker is the other player
	KindInfo         // "[INFO] You do not have a valid ID card..."
)

// Record is one parsed chat line.
type Record struct {
	Line    int       // 1-based line number in the session
	Time    time.Time // UTC; zero date when the session has no header
	HasTime bool      // false when the line had no timestamp and inherited the previous one
	Kind    Kind
	Speaker string // empty when the kind has no speaker
	Text    string // the message without timestamp, speaker or decoration
	Raw     string // the line exactly as stored
}

// lineRule recognises one kind of line. Its pattern may capture "speaker" and "text".
type lineRule struct {
	kind    Kind
	pattern *regexp.Regexp
}

// lineRules are tried in order; the first match wins, so specific rules come first.
var lineRules = []lineRule{
	{KindHeader, sessionHeaderPattern},
	{KindPM, regexp.MustCompile(`^\(\( PM (?:from|to) \(\d+\) (?P<speaker>[^:]+): (?P<text>.*?) \)\)$`)},
	{KindOOC, regexp.MustCompile(`^\(\( (?P<speaker>[^:()]+): (?P<text>.*?) \)\)$`)},
	{KindDo, regexp.MustCompile(`^\* (?P<text>.+) \(\( (?P<speaker>[^()]+) \)\)$`)},
	{KindMe, regexp.MustCompile(`^\* (?P<speaker>\S+ \S+) (?P<text>.+)$`)},
	{KindInfo, regexp.MustCompile(`^\[INFO\] (?P<text>.*)$`)},
	{KindSay, regexp.MustCompile(`^(?P<speaker>[^\[*(][^:]*?) says(?: \[low\])?: (?P<text>.*)$`)},
	{KindShout, regexp.MustCompile(`^(?P<speaker>[^\[*(][^:]*?) shouts: (?P<text>.*)$`)},
	{KindWhisper, regexp.MustCompile(`^(?P<speaker>[^\[*(][^:]*?) whispers(?: to [^:]+)?: (?P<text>.*)$`)},
}

var timestampPattern = regexp.MustCompile(`^\[(\d{2}:\d{2}:\d{2})\] `)

// Parse reads one session's stored text and returns its records in order.
func Parse(r io.Reader) ([]Record, error) {
	var records []Record
	var clock sessionClock
	scanner := bufio.NewScanner(r)
	scanner.Buffer(nil, 1<<20)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		raw := scanner.Text()
		start, isHeader := parseSessionHeader(raw)
		if isHeader {
			clock = sessionClock{current: start}
		}
		record := parseLine(raw, &clock)
		record.Line = lineNumber
		record.HasTime = record.HasTime || isHeader
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		return records, fmt.Errorf("parse chat session: %w", err)
	}
	return records, nil
}

func parseLine(raw string, clock *sessionClock) Record {
	record := Record{Raw: raw, Text: raw, Time: clock.current}
	body := raw
	if match := timestampPattern.FindStringSubmatch(raw); match != nil {
		if timeOfDay, err := time.Parse(time.TimeOnly, match[1]); err == nil {
			record.Time = clock.advance(timeOfDay)
			record.HasTime = true
		}
		body = raw[len(match[0]):]
		record.Text = body
	}
	classify(&record, body)
	return record
}

func classify(record *Record, body string) {
	for _, rule := range lineRules {
		match := rule.pattern.FindStringSubmatch(body)
		if match == nil {
			continue
		}
		record.Kind = rule.kind
		if i := rule.pattern.SubexpIndex("speaker"); i > 0 {
			record.Speaker = strings.TrimSpace(match[i])
		}
		if i := rule.pattern.SubexpIndex("text"); i > 0 {
			record.Text = match[i]
		}
		return
	}
}

// sessionClock turns time-only timestamps into full times. Lines only carry HH:MM:SS, so a
// jump back of more than half a day is read as passing midnight.
type sessionClock struct {
	current time.Time
}

func (c *sessionClock) advance(timeOfDay time.Time) time.Time {
	year, month, day := c.current.Date()
	next := time.Date(year, month, day,
		timeOfDay.Hour(), timeOfDay.Minute(), timeOfDay.Second(), 0, time.UTC)
	if next.Before(c.current.Add(-12 * time.Hour)) {
		next = next.AddDate(0, 0, 1)
	}
	c.current = next
	return next
}
