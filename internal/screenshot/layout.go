package screenshot

import (
	"slices"
	"unicode"

	"golang.org/x/image/font/sfnt"
)

// placedChar is a character where Render draws it on its row.
type placedChar struct {
	char    Char
	index   int     // its index in its line, for Positions
	x       float64 // its pen position, in pixels right of the row's text origin
	advance float64 // to the next character's pen: its width, kerning and letter spacing
	face    *chatFace
	glyph   sfnt.GlyphIndex // with face nil, no font has the character: it takes no space
}

// textRow is one drawn row of text: a whole line, or part of a wrapped one.
type textRow struct {
	line  int // index of the line it belongs to
	chars []placedChar
	width float64 // pixels, to the end of the last character's advance
}

// layoutRows places every line's characters, wrapping lines wider than maxWidth pixels
// where the game would: at spaces, and after a hyphen inside a word. The space a line
// wraps at isn't drawn. A single word wider than the limit is split between characters,
// as the chat's word-break: break-word does. An empty line is one empty row.
func layoutRows(faces faceSet, lines []Line, maxWidth float64) []textRow {
	var rows []textRow
	for lineIndex, line := range lines {
		chars := lineChars(line)
		for _, indices := range wrapChars(faces, chars, maxWidth) {
			rows = append(rows, placeRow(faces, chars, indices, lineIndex))
		}
	}
	return rows
}

// fits reports whether a row is no wider than maxWidth, measured as Chromium does in
// layout units.
func (r textRow) fits(maxWidth float64) bool {
	return layoutUnit(r.width) <= maxWidth
}

// wrapChars splits a line into rows of character indices.
func wrapChars(faces faceSet, chars []Char, maxWidth float64) [][]int {
	words := splitWords(chars)
	var rows [][]int
	current := words[0]
	for _, word := range words[1:] {
		candidate := slices.Concat(current, word)
		if placeRow(faces, chars, candidate, 0).fits(maxWidth) {
			current = candidate
			continue
		}
		rows = append(rows, current)
		current = word
		if chars[word[0]].R == ' ' {
			current = word[1:] // a wrapped row doesn't start with the space it wrapped at
		}
	}
	return breakLongWords(faces, chars, append(rows, current), maxWidth)
}

// breakLongWords splits rows still wider than maxWidth (a single word longer than the
// limit) between characters, so the image never grows past its width limit.
func breakLongWords(faces faceSet, chars []Char, rows [][]int, maxWidth float64) [][]int {
	var fitted [][]int
	for _, row := range rows {
		if placeRow(faces, chars, row, 0).fits(maxWidth) {
			fitted = append(fitted, row)
			continue
		}
		start := 0
		for end := 1; end <= len(row); end++ {
			isTooWide := !placeRow(faces, chars, row[start:end], 0).fits(maxWidth)
			if isTooWide && end-1 > start { // at least one character per row
				fitted = append(fitted, row[start:end-1])
				start = end - 1
			}
		}
		fitted = append(fitted, row[start:])
	}
	return fitted
}

// splitWords groups character indices into the pieces a line may wrap between: a word
// after a space starts with that space, and a word may also end after a hyphen that has a
// letter on both sides ("L-C-C").
func splitWords(chars []Char) [][]int {
	words := [][]int{nil}
	for i, c := range chars {
		if c.R == ' ' || isBreakAfterHyphen(chars, i-1) {
			words = append(words, nil)
		}
		last := len(words) - 1
		words[last] = append(words[last], i)
	}
	if len(words) > 1 && words[0] == nil { // a line starting with a space
		words = words[1:]
	}
	return words
}

func isBreakAfterHyphen(chars []Char, i int) bool {
	if i < 1 || i+1 >= len(chars) || chars[i].R != '-' {
		return false
	}
	return unicode.IsLetter(chars[i-1].R) && unicode.IsLetter(chars[i+1].R)
}

// placeRow lays the characters at indices out left to right, as Chromium does: each
// advances by its unhinted width plus the chat's letter spacing, and neighbors drawn with
// the same face are kerned.
func placeRow(faces faceSet, chars []Char, indices []int, line int) textRow {
	row := textRow{line: line, chars: make([]placedChar, len(indices))}
	x := 0.0
	for i, index := range indices {
		char := chars[index]
		placed := placedChar{char: char, index: index, x: x}
		if face, glyph, ok := faces.of(char.IsItalic).lookup(char.R); ok {
			placed.face, placed.glyph = face, glyph
			placed.advance = face.advance(glyph) + chatLetterSpacingPx
			if i > 0 && row.chars[i-1].face == face {
				kern := face.kern(row.chars[i-1].glyph, glyph)
				row.chars[i-1].advance += kern
				placed.x += kern
			}
		}
		row.chars[i] = placed
		x = placed.x + placed.advance
	}
	row.width = x
	return row
}
