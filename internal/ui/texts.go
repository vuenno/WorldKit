package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"world-kit/internal/chatlog"
)

// Times are shown in UTC, as the game writes them.
const (
	clockLayout   = "15:04"
	dayLayout     = "Mon 02 Jan"
	dayYearLayout = "Mon 02 Jan 2006"
	fullLayout    = "Mon 02 Jan 2006, 15:04 UTC"
)

// dayGroupLabel names the group a time belongs to in a newest-first list: Today,
// Yesterday, Earlier this week, then month and year.
func dayGroupLabel(t, now time.Time) string {
	days := daysBetween(t, now)
	switch {
	case days <= 0:
		return "Today"
	case days == 1:
		return "Yesterday"
	case days < 7:
		return "Earlier this week"
	default:
		return t.UTC().Format("January 2006")
	}
}

// dayText names t's day: "Today", "Yesterday", or its date (with the year if not this year).
func dayText(t, now time.Time) string {
	switch daysBetween(t, now) {
	case 0:
		return "Today"
	case 1:
		return "Yesterday"
	}
	return dateText(t, now)
}

// dateText is the date alone, "Sat 17 Oct", for rows already grouped under Today or Yesterday.
func dateText(t, now time.Time) string {
	if t.UTC().Year() != now.UTC().Year() {
		return t.UTC().Format(dayYearLayout)
	}
	return t.UTC().Format(dayLayout)
}

// dayAndTimeText is "Today, 09:57 UTC" or "Sat 17 Oct, 21:14 UTC".
func dayAndTimeText(t, now time.Time) string {
	return dayText(t, now) + ", " + t.UTC().Format(clockLayout) + " UTC"
}

// savedText is "Saved today, 14:18 UTC" or "Saved Thu 15 Oct, 19:30 UTC".
func savedText(t, now time.Time) string {
	day := dayText(t, now)
	if daysBetween(t, now) <= 1 {
		day = strings.ToLower(day)
	}
	return "Saved " + day + ", " + t.UTC().Format(clockLayout) + " UTC"
}

// daysBetween counts calendar days (UTC) from t to now.
func daysBetween(t, now time.Time) int {
	day := func(x time.Time) time.Time {
		x = x.UTC()
		return time.Date(x.Year(), x.Month(), x.Day(), 0, 0, 0, 0, time.UTC)
	}
	return int(day(now).Sub(day(t)).Hours() / 24)
}

// durationText is "2 h 05 min", "48 min" or "< 1 min".
func durationText(d time.Duration) string {
	minutes := int(d.Minutes())
	switch {
	case minutes < 1:
		return "< 1 min"
	case minutes < 60:
		return fmt.Sprintf("%d min", minutes)
	default:
		return fmt.Sprintf("%d h %02d min", minutes/60, minutes%60)
	}
}

// countText writes n with thousands separators: 1,284.
func countText(n int) string {
	digits := strconv.Itoa(n)
	var b strings.Builder
	for i, digit := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(digit)
	}
	return b.String()
}

// charactersText lists up to shown names, then "+N" for the rest.
func charactersText(characters []chatlog.Character, shown int) string {
	if len(characters) == 0 {
		return "No one spoke"
	}
	names := make([]string, 0, shown)
	for _, character := range characters[:min(shown, len(characters))] {
		names = append(names, character.Name)
	}
	text := strings.Join(names, ", ")
	if rest := len(characters) - len(names); rest > 0 {
		text += fmt.Sprintf(" +%d", rest)
	}
	return text
}

// plural is "1 line" or "4 lines".
func plural(n int, singular, pluralForm string) string {
	if n == 1 {
		return "1 " + singular
	}
	return countText(n) + " " + pluralForm
}
