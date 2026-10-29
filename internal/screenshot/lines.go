package screenshot

import (
	"slices"
	"unicode"
)

// LineCount returns how many lines the document has, empty ones included.
func (d *Document) LineCount() int {
	return len(d.lines)
}

// MoveLine moves the line at from so it ends up at index to; the other lines keep their
// order. Indices past the document are clamped.
func (d *Document) MoveLine(from, to int) {
	last := len(d.lines) - 1
	from, to = max(0, min(from, last)), max(0, min(to, last))
	if from == to {
		return
	}
	line := d.lines[from]
	d.lines = slices.Insert(slices.Delete(d.lines, from, from+1), to, line)
}

// SelectedLines returns the first and last line sel touches. A selection that ends at the
// very start of a line doesn't touch that line, as when whole lines are dragged over.
func (d *Document) SelectedLines(sel Selection) (first, last int) {
	from, to := sel.Ordered()
	from, to = d.Clamp(from), d.Clamp(to)
	if to.Index == 0 && to.Line > from.Line {
		to.Line--
	}
	return from.Line, to.Line
}

// CanMergeLines reports whether lines first..last can be joined into one: at least two of
// them have text, and every one with text is speech by the same speaker said the same way
// ("Mason Fletcher says: "). Empty lines between them don't matter.
func (d *Document) CanMergeLines(first, last int) bool {
	speaker, count := "", 0
	for _, chars := range d.textLines(first, last) {
		start, end, ok := speechPrefix(chars)
		if !ok {
			return false
		}
		prefix := string(runesOf(chars[start:end]))
		if count > 0 && prefix != speaker {
			return false
		}
		speaker = prefix
		count++
	}
	return count >= 2
}

// MergeLines joins lines first..last into the first one with text: each later line adds
// its words after a space, without its timestamp and speaker, so "Mason Fletcher says: X."
// and "Mason Fletcher says: Y." become "Mason Fletcher says: X. Y.". Characters keep their
// styles. It returns the caret after the merged text, and false when the lines can't be
// merged (see CanMergeLines).
func (d *Document) MergeLines(first, last int) (Position, bool) {
	first, last = max(0, first), min(last, len(d.lines)-1)
	if !d.CanMergeLines(first, last) {
		return Position{}, false
	}
	lines := d.textLines(first, last)
	merged := trimTrailingSpaces(lines[0])
	for _, chars := range lines[1:] {
		_, end, _ := speechPrefix(chars)
		words := trimTrailingSpaces(trimLeadingSpaces(chars[end:]))
		if len(words) == 0 {
			continue
		}
		space := words[0]
		space.R = ' '
		merged = append(append(merged, space), words...)
	}
	kept := d.firstTextLine(first, last)
	kept.chars = merged
	d.lines = slices.Concat(d.lines[:first], []docLine{kept}, d.lines[last+1:])
	return Position{Line: first, Index: len(merged)}, true
}

// textLines returns copies of the characters of lines first..last that have visible text.
func (d *Document) textLines(first, last int) [][]Char {
	var lines [][]Char
	for _, line := range d.linesBetween(first, last) {
		if len(trimLeadingSpaces(line.chars)) > 0 {
			lines = append(lines, slices.Clone(line.chars))
		}
	}
	return lines
}

// firstTextLine returns the first of lines first..last with visible text.
func (d *Document) firstTextLine(first, last int) docLine {
	lines := d.linesBetween(first, last)
	for _, line := range lines {
		if len(trimLeadingSpaces(line.chars)) > 0 {
			return line
		}
	}
	return lines[0]
}

// linesBetween returns lines first..last, clamped to the document.
func (d *Document) linesBetween(first, last int) []docLine {
	first, last = max(0, first), min(last, len(d.lines)-1)
	if first > last {
		return nil
	}
	return d.lines[first : last+1]
}

// timestampLength is the length of a leading "[HH:MM:SS] " timestamp.
const timestampLength = len("[00:00:00] ")

// speechPrefix finds a speech line's speaker part, "Mason Fletcher says: ", as character
// indices start..end; start skips a leading timestamp, so lines said at different times
// still match. Lines that start with "*", "(" or "[" (emotes, OOC, [INFO]) aren't speech,
// matching how chatlog recognises says, shouts and whispers.
func speechPrefix(chars []Char) (start, end int, ok bool) {
	runes := runesOf(chars)
	if hasTimestamp(runes) {
		start = timestampLength
	}
	if start >= len(runes) || slices.Contains([]rune("*(["), runes[start]) {
		return 0, 0, false
	}
	for i := start; i+1 < len(runes); i++ {
		if runes[i] == ':' && runes[i+1] == ' ' {
			return start, i + 2, true
		}
	}
	return 0, 0, false
}

// hasTimestamp reports whether runes start with "[HH:MM:SS] ".
func hasTimestamp(runes []rune) bool {
	if len(runes) < timestampLength {
		return false
	}
	for i, want := range "[00:00:00] " {
		isMatch := runes[i] == want
		if want == '0' {
			isMatch = unicode.IsDigit(runes[i])
		}
		if !isMatch {
			return false
		}
	}
	return true
}

func trimLeadingSpaces(chars []Char) []Char {
	for len(chars) > 0 && unicode.IsSpace(chars[0].R) {
		chars = chars[1:]
	}
	return chars
}

func trimTrailingSpaces(chars []Char) []Char {
	for len(chars) > 0 && unicode.IsSpace(chars[len(chars)-1].R) {
		chars = chars[:len(chars)-1]
	}
	return chars
}
