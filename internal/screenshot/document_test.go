package screenshot

import (
	"slices"
	"testing"
)

// plainText returns the document's lines as text, one string per line.
func plainText(d *Document) []string {
	var texts []string
	for _, line := range d.Lines() {
		text := ""
		for _, span := range line.Spans {
			text += span.Text
		}
		texts = append(texts, text)
	}
	return texts
}

func at(line, index int) Position { return Position{Line: line, Index: index} }

func span(from, to Position) Selection { return Selection{Anchor: from, Caret: to} }

func TestDocumentReplace(t *testing.T) {
	tests := []struct {
		name      string
		sel       Selection
		text      string
		want      []string
		wantCaret Position
	}{
		{"type in the middle", span(at(0, 5), at(0, 5)), "big ", []string{"John big Doe", "Jane Roe"}, at(0, 9)},
		{"replace a word", span(at(0, 5), at(0, 8)), "Smith", []string{"John Smith", "Jane Roe"}, at(0, 10)},
		{"backwards selection", span(at(0, 8), at(0, 5)), "X", []string{"John X", "Jane Roe"}, at(0, 6)},
		{"delete across lines joins them", span(at(0, 4), at(1, 4)), "", []string{"John Roe"}, at(0, 4)},
		{"newline splits a line", span(at(0, 4), at(0, 5)), "\n", []string{"John", "Doe", "Jane Roe"}, at(1, 0)},
		{"type at the end", span(at(1, 8), at(1, 8)), "!", []string{"John Doe", "Jane Roe!"}, at(1, 9)},
		{"positions past the end are clamped", span(at(5, 99), at(5, 99)), "?", []string{"John Doe", "Jane Roe?"}, at(1, 9)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := NewDocument([]Line{sayLine("John Doe"), sayLine("Jane Roe")})
			caret := d.Replace(test.sel, test.text)
			if got := plainText(d); !slices.Equal(got, test.want) {
				t.Errorf("text = %q, want %q", got, test.want)
			}
			if caret != test.wantCaret {
				t.Errorf("caret = %v, want %v", caret, test.wantCaret)
			}
		})
	}
}

func TestDocumentTypedTextTakesStyleBefore(t *testing.T) {
	d := NewDocument([]Line{NewLine("I /really/", say, emote)})
	d.Replace(span(at(0, 8), at(0, 8)), "!")
	want := []Span{{"I ", say, false}, {"really!", say, true}}
	if got := d.Lines()[0].Spans; !slices.Equal(got, want) {
		t.Errorf("spans = %v, want %v", got, want)
	}
}

func TestDocumentRestyle(t *testing.T) {
	tests := []struct {
		name string
		edit func(d *Document)
		want []Span
	}{
		{"color a word", func(d *Document) { d.SetColor(span(at(0, 0), at(0, 4)), red) },
			[]Span{{"John", red, false}, {" waves", say, false}}},
		{"color one letter", func(d *Document) { d.SetColor(span(at(0, 1), at(0, 2)), red) },
			[]Span{{"J", say, false}, {"o", red, false}, {"hn waves", say, false}}},
		{"italic a word", func(d *Document) { d.ToggleItalic(span(at(0, 5), at(0, 10))) },
			[]Span{{"John ", say, false}, {"waves", say, true}}},
		{"italic twice is upright", func(d *Document) {
			d.ToggleItalic(span(at(0, 5), at(0, 10)))
			d.ToggleItalic(span(at(0, 5), at(0, 10)))
		}, []Span{{"John waves", say, false}}},
		{"italic over a mixed range makes it all italic", func(d *Document) {
			d.ToggleItalic(span(at(0, 5), at(0, 10)))
			d.ToggleItalic(d.All())
		}, []Span{{"John waves", say, true}}},
		{"reset restores the automatic color", func(d *Document) {
			d.SetColor(d.All(), red)
			d.ResetColor(span(at(0, 0), at(0, 4)))
		}, []Span{{"John", say, false}, {" waves", red, false}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := NewDocument([]Line{sayLine("John waves")})
			test.edit(d)
			if got := d.Lines()[0].Spans; !slices.Equal(got, test.want) {
				t.Errorf("spans = %v, want %v", got, test.want)
			}
		})
	}
}

func TestDocumentRestyleAcrossLines(t *testing.T) {
	d := NewDocument([]Line{sayLine("ab"), sayLine("cd")})
	d.SetColor(span(at(1, 1), at(0, 1)), red)
	if got, want := d.Lines()[0].Spans, []Span{{"a", say, false}, {"b", red, false}}; !slices.Equal(got, want) {
		t.Errorf("line 0 = %v, want %v", got, want)
	}
	if got, want := d.Lines()[1].Spans, []Span{{"c", red, false}, {"d", say, false}}; !slices.Equal(got, want) {
		t.Errorf("line 1 = %v, want %v", got, want)
	}
}

func TestDocumentMove(t *testing.T) {
	d := NewDocument([]Line{sayLine("ab"), sayLine("cd")})
	tests := []struct {
		from  Position
		delta int
		want  Position
	}{
		{at(0, 1), -1, at(0, 0)},
		{at(0, 0), -1, at(0, 0)},
		{at(1, 0), -1, at(0, 2)},
		{at(0, 2), 1, at(1, 0)},
		{at(1, 2), 1, at(1, 2)},
	}
	for _, test := range tests {
		if got := d.Move(test.from, test.delta); got != test.want {
			t.Errorf("Move(%v, %d) = %v, want %v", test.from, test.delta, got, test.want)
		}
	}
}

func TestDocumentNonEmptyLines(t *testing.T) {
	d := NewDocument([]Line{sayLine("a"), sayLine("  "), sayLine(""), sayLine("b")})
	if got := len(d.NonEmptyLines()); got != 2 {
		t.Errorf("NonEmptyLines = %d lines, want 2", got)
	}
	if got := len(d.Lines()); got != 4 {
		t.Errorf("Lines = %d lines, want all 4", got)
	}
}

func TestDocumentCloneIsIndependent(t *testing.T) {
	d := NewDocument([]Line{sayLine("ab")})
	clone := d.Clone()
	d.SetColor(d.All(), red)
	d.Replace(d.All(), "x")
	if got := plainText(clone); !slices.Equal(got, []string{"ab"}) || clone.Lines()[0].Spans[0].Color != say {
		t.Errorf("clone changed: %v", clone.Lines())
	}
}
