package screenshot

import (
	"image/color"
	"unicode"
)

// Span is a run of a line's text drawn one way.
type Span struct {
	Text     string
	Color    color.NRGBA
	IsItalic bool
}

// Markup understood in chat lines, as players type it in game:
//
//   - /text/ draws text in italic; the slashes are removed.
//   - *text* draws text in the /me color, as players mark actions inside speech
//     ("I'm leaving *grabs his coat*"); the asterisks are removed.
//
// A marker opens only at the start of a word and closes only at the end of one, so
// "and/or", "1/2" or the leading "* " of a /me line stay plain text.
const (
	italicMarker = '/'
	emoteMarker  = '*'
)

// styledRune is one character of a line with the style its markup gives it.
type styledRune struct {
	r         rune
	isEmote   bool
	isItalic  bool
	isRemoved bool // a marker that is not drawn
}

// NewLine parses text's markup into a line drawn in lineColor, with *…* in emoteColor.
func NewLine(text string, lineColor, emoteColor color.NRGBA) Line {
	return Line{Spans: ParseMarkup(text, lineColor, emoteColor), Color: lineColor}
}

// ParseMarkup splits text into spans: lineColor is the line's own color and emoteColor the
// color *…* is drawn in.
func ParseMarkup(text string, lineColor, emoteColor color.NRGBA) []Span {
	var runes []styledRune
	for _, r := range text {
		runes = append(runes, styledRune{r: r})
	}
	markPairs(runes, italicMarker)
	markPairs(runes, emoteMarker)
	var spans []Span
	for _, styled := range runes {
		if styled.isRemoved {
			continue
		}
		span := Span{Text: string(styled.r), Color: lineColor, IsItalic: styled.isItalic}
		if styled.isEmote {
			span.Color = emoteColor
		}
		spans = appendSpan(spans, span)
	}
	return spans
}

// appendSpan adds span, joining it to the last one when both look the same.
func appendSpan(spans []Span, span Span) []Span {
	if n := len(spans); n > 0 && spans[n-1].Color == span.Color && spans[n-1].IsItalic == span.IsItalic {
		spans[n-1].Text += span.Text
		return spans
	}
	return append(spans, span)
}

// markPairs finds each opening marker and the nearest closing one after it, styles the
// runes between them and removes the two markers.
func markPairs(runes []styledRune, marker rune) {
	for open := 0; open < len(runes); open++ {
		if !opensSpan(runes, open, marker) {
			continue
		}
		closing := findClosing(runes, open, marker)
		if closing < 0 {
			continue
		}
		for i := open; i <= closing; i++ {
			runes[i].isItalic = runes[i].isItalic || marker == italicMarker
			runes[i].isEmote = runes[i].isEmote || marker == emoteMarker
		}
		runes[open].isRemoved, runes[closing].isRemoved = true, true
		open = closing
	}
}

func findClosing(runes []styledRune, open int, marker rune) int {
	for i := open + 2; i < len(runes); i++ {
		if closesSpan(runes, i, marker) {
			return i
		}
	}
	return -1
}

// opensSpan reports whether runes[i] is marker at the start of a word: after nothing, a
// space or punctuation, and followed by visible text.
func opensSpan(runes []styledRune, i int, marker rune) bool {
	if runes[i].r != marker || runes[i].isRemoved || i+1 >= len(runes) {
		return false
	}
	next := runes[i+1].r
	if unicode.IsSpace(next) || next == marker {
		return false
	}
	return i == 0 || !isWordRune(runes[i-1].r)
}

// closesSpan reports whether runes[i] is marker at the end of a word: after visible text,
// and followed by nothing, a space or punctuation.
func closesSpan(runes []styledRune, i int, marker rune) bool {
	if runes[i].r != marker || runes[i].isRemoved {
		return false
	}
	previous := runes[i-1].r
	if unicode.IsSpace(previous) || previous == marker {
		return false
	}
	return i == len(runes)-1 || (!isWordRune(runes[i+1].r) && runes[i+1].r != marker)
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}
