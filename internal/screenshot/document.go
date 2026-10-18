package screenshot

import (
	"image/color"
	"slices"
	"strings"
)

// Char is one editable character with its look.
type Char struct {
	R         rune
	Color     color.NRGBA
	AutoColor color.NRGBA // its color before any recoloring: what ResetColor restores
	IsItalic  bool
}

// Position is a caret position: before the character at Index on line Line.
type Position struct {
	Line, Index int
}

// Before reports whether p comes before q.
func (p Position) Before(q Position) bool {
	return p.Line < q.Line || (p.Line == q.Line && p.Index < q.Index)
}

// Selection is the text between Anchor, where selecting started, and Caret. An empty
// selection is just a caret.
type Selection struct {
	Anchor, Caret Position
}

// IsEmpty reports whether nothing is selected.
func (s Selection) IsEmpty() bool {
	return s.Anchor == s.Caret
}

// Ordered returns the selection's start and end in text order.
func (s Selection) Ordered() (from, to Position) {
	if s.Caret.Before(s.Anchor) {
		return s.Caret, s.Anchor
	}
	return s.Anchor, s.Caret
}

type docLine struct {
	chars []Char
	color color.NRGBA // the line's own color, for text typed where no character gives a style
}

// Document is screenshot lines as styled characters that can be edited: typed over,
// recolored and italicized character by character.
type Document struct {
	lines []docLine
}

// NewDocument returns a document holding lines; with none, it holds one empty line, so it
// always has somewhere to type.
func NewDocument(lines []Line) *Document {
	d := &Document{lines: make([]docLine, max(1, len(lines)))}
	for i, line := range lines {
		d.lines[i] = docLine{chars: lineChars(line), color: line.Color}
	}
	return d
}

// lineChars splits a line's spans into characters.
func lineChars(line Line) []Char {
	var chars []Char
	for _, span := range line.Spans {
		chars = append(chars, styledChars(span.Text, Char{Color: span.Color, AutoColor: span.Color, IsItalic: span.IsItalic})...)
	}
	return chars
}

func styledChars(text string, style Char) []Char {
	var chars []Char
	for _, r := range text {
		style.R = r
		chars = append(chars, style)
	}
	return chars
}

// Clone returns an independent copy, e.g. to undo to.
func (d *Document) Clone() *Document {
	clone := &Document{lines: slices.Clone(d.lines)}
	for i := range clone.lines {
		clone.lines[i].chars = slices.Clone(clone.lines[i].chars)
	}
	return clone
}

// Lines returns every line, empty ones included, so line numbers match Positions.
func (d *Document) Lines() []Line {
	lines := make([]Line, len(d.lines))
	for i, line := range d.lines {
		lines[i].Color = line.color
		for _, c := range line.chars {
			lines[i].Spans = appendSpan(lines[i].Spans, Span{Text: string(c.R), Color: c.Color, IsItalic: c.IsItalic})
		}
	}
	return lines
}

// NonEmptyLines returns the lines that have visible text: what is saved.
func (d *Document) NonEmptyLines() []Line {
	var lines []Line
	for i, line := range d.Lines() {
		if strings.TrimSpace(string(runesOf(d.lines[i].chars))) != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func runesOf(chars []Char) []rune {
	runes := make([]rune, len(chars))
	for i, c := range chars {
		runes[i] = c.R
	}
	return runes
}

// All selects the whole document.
func (d *Document) All() Selection {
	last := len(d.lines) - 1
	return Selection{Caret: Position{Line: last, Index: len(d.lines[last].chars)}}
}

// Clamp moves p onto the document if it is past a line's or the document's end.
func (d *Document) Clamp(p Position) Position {
	p.Line = max(0, min(p.Line, len(d.lines)-1))
	p.Index = max(0, min(p.Index, len(d.lines[p.Line].chars)))
	return p
}

// Move returns p moved one character left (delta < 0) or right, crossing line ends.
func (d *Document) Move(p Position, delta int) Position {
	p = d.Clamp(p)
	switch {
	case delta < 0 && p.Index > 0:
		p.Index--
	case delta < 0 && p.Line > 0:
		p = Position{Line: p.Line - 1, Index: len(d.lines[p.Line-1].chars)}
	case delta > 0 && p.Index < len(d.lines[p.Line].chars):
		p.Index++
	case delta > 0 && p.Line < len(d.lines)-1:
		p = Position{Line: p.Line + 1}
	}
	return p
}

// LineEdge returns the start (toEnd false) or end of p's line.
func (d *Document) LineEdge(p Position, toEnd bool) Position {
	p = d.Clamp(p)
	if toEnd {
		return Position{Line: p.Line, Index: len(d.lines[p.Line].chars)}
	}
	return Position{Line: p.Line}
}

// Replace puts text in place of the selection and returns the caret after it. Typed text
// takes the style of the character before it; "\n" splits the line.
func (d *Document) Replace(sel Selection, text string) Position {
	from, to := sel.Ordered()
	from, to = d.Clamp(from), d.Clamp(to)
	style := d.styleAt(from)
	head := slices.Clone(d.lines[from.Line].chars[:from.Index])
	tail := slices.Clone(d.lines[to.Line].chars[to.Index:])
	pieces := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	inserted := make([]docLine, len(pieces))
	for i, piece := range pieces {
		inserted[i] = docLine{chars: styledChars(piece, style), color: d.lines[from.Line].color}
	}
	last := len(inserted) - 1
	inserted[0].chars = append(head, inserted[0].chars...)
	caret := Position{Line: from.Line + last, Index: len(inserted[last].chars)}
	inserted[last].chars = append(inserted[last].chars, tail...)
	d.lines = slices.Concat(d.lines[:from.Line], inserted, d.lines[to.Line+1:])
	return caret
}

// styleAt is the style text typed at p takes: the character before p, else the one after
// it, else the line's own color.
func (d *Document) styleAt(p Position) Char {
	chars := d.lines[p.Line].chars
	switch {
	case p.Index > 0:
		return chars[p.Index-1]
	case len(chars) > 0:
		return chars[0]
	default:
		lineColor := d.lines[p.Line].color
		return Char{Color: lineColor, AutoColor: lineColor}
	}
}

// SetColor colors the selected characters.
func (d *Document) SetColor(sel Selection, c color.NRGBA) {
	d.restyle(sel, func(char *Char) { char.Color = c })
}

// ResetColor gives the selected characters back their automatic color.
func (d *Document) ResetColor(sel Selection) {
	d.restyle(sel, func(char *Char) { char.Color = char.AutoColor })
}

// ToggleItalic makes the selected characters italic, or upright when all already are.
func (d *Document) ToggleItalic(sel Selection) {
	isAllItalic := true
	d.restyle(sel, func(char *Char) { isAllItalic = isAllItalic && char.IsItalic })
	d.restyle(sel, func(char *Char) { char.IsItalic = !isAllItalic })
}

func (d *Document) restyle(sel Selection, change func(*Char)) {
	from, to := sel.Ordered()
	from, to = d.Clamp(from), d.Clamp(to)
	for line := from.Line; line <= to.Line; line++ {
		chars := d.lines[line].chars
		start, end := 0, len(chars)
		if line == from.Line {
			start = from.Index
		}
		if line == to.Line {
			end = to.Index
		}
		for i := start; i < end; i++ {
			change(&chars[i])
		}
	}
}
