package screenshot

import (
	"slices"

	"golang.org/x/image/math/fixed"
)

// placedChar is a character where Render draws it on its row.
type placedChar struct {
	char    Char
	index   int           // its index in its line, for Positions
	x       fixed.Int26_6 // from the row's left edge
	advance fixed.Int26_6
}

// textRow is one drawn row of text: a whole line, or part of a wrapped one.
type textRow struct {
	line  int // index of the line it belongs to
	chars []placedChar
	width int // pixels
}

// layoutRows places every line's characters, wrapping lines wider than maxWidth pixels at
// spaces. The space a line wraps at isn't drawn. A single word wider than the limit stays
// on its own row rather than being cut. An empty line is one empty row.
func layoutRows(faces *faces, lines []Line, maxWidth int) []textRow {
	var rows []textRow
	for lineIndex, line := range lines {
		chars := lineChars(line)
		for _, indices := range wrapChars(faces, chars, maxWidth) {
			rows = append(rows, placeRow(faces, chars, indices, lineIndex))
		}
	}
	return rows
}

// wrapChars splits a line into rows of character indices.
func wrapChars(faces *faces, chars []Char, maxWidth int) [][]int {
	words := splitWords(chars)
	var rows [][]int
	current := words[0]
	for _, word := range words[1:] {
		candidate := slices.Concat(current, word)
		if placeRow(faces, chars, candidate, 0).width <= maxWidth {
			current = candidate
			continue
		}
		rows = append(rows, current)
		current = word[1:] // a wrapped row doesn't start with the space it wrapped at
	}
	return append(rows, current)
}

// splitWords groups character indices into words; every word after the first starts with
// the space before it.
func splitWords(chars []Char) [][]int {
	words := [][]int{nil}
	for i, c := range chars {
		if c.R == ' ' {
			words = append(words, []int{i})
			continue
		}
		last := len(words) - 1
		words[last] = append(words[last], i)
	}
	return words
}

// placeRow lays the characters at indices out left to right, kerning neighbors that share
// a face.
func placeRow(faces *faces, chars []Char, indices []int, line int) textRow {
	row := textRow{line: line, chars: make([]placedChar, len(indices))}
	var x fixed.Int26_6
	for i, index := range indices {
		char := chars[index]
		face := faces.of(char.IsItalic)
		if i > 0 && chars[indices[i-1]].IsItalic == char.IsItalic {
			x += face.Kern(chars[indices[i-1]].R, char.R)
		}
		advance, _ := face.GlyphAdvance(char.R) // a missing glyph advances 0 and draws nothing
		row.chars[i] = placedChar{char: char, index: index, x: x, advance: advance}
		x += advance
	}
	row.width = x.Ceil()
	return row
}
