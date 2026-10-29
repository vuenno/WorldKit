package screenshot

import (
	"slices"
	"testing"
)

func newTextDocument(texts ...string) *Document {
	lines := make([]Line, len(texts))
	for i, text := range texts {
		lines[i] = sayLine(text)
	}
	return NewDocument(lines)
}

func TestDocumentMoveLine(t *testing.T) {
	tests := []struct {
		name     string
		from, to int
		want     []string
	}{
		{"down one", 0, 1, []string{"B", "A", "C"}},
		{"to the end", 0, 2, []string{"B", "C", "A"}},
		{"to the top", 2, 0, []string{"C", "A", "B"}},
		{"same place", 1, 1, []string{"A", "B", "C"}},
		{"past the end is clamped", 0, 9, []string{"B", "C", "A"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := newTextDocument("A", "B", "C")
			d.MoveLine(test.from, test.to)
			if got := plainText(d); !slices.Equal(got, test.want) {
				t.Errorf("text = %q, want %q", got, test.want)
			}
		})
	}
}

func TestDocumentSelectedLines(t *testing.T) {
	d := newTextDocument("one", "two", "three")
	tests := []struct {
		name                string
		sel                 Selection
		wantFirst, wantLast int
	}{
		{"within a line", span(at(1, 0), at(1, 2)), 1, 1},
		{"across lines", span(at(0, 1), at(2, 1)), 0, 2},
		{"backwards", span(at(2, 1), at(0, 1)), 0, 2},
		{"ending at a line start leaves that line out", span(at(0, 0), at(2, 0)), 0, 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first, last := d.SelectedLines(test.sel)
			if first != test.wantFirst || last != test.wantLast {
				t.Errorf("lines = %d..%d, want %d..%d", first, last, test.wantFirst, test.wantLast)
			}
		})
	}
}

func TestDocumentMergeLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []string // nil: the lines can't be merged
	}{
		{
			"two lines by one speaker",
			[]string{"Mason Fletcher says: X.", "Mason Fletcher says: Y."},
			[]string{"Mason Fletcher says: X. Y."},
		},
		{
			"three lines, timestamps differ",
			[]string{"[09:57:03] Mason Fletcher says: X.", "[09:57:09] Mason Fletcher says: Y.", "[09:58:00] Mason Fletcher says: Z."},
			[]string{"[09:57:03] Mason Fletcher says: X. Y. Z."},
		},
		{
			"empty lines between are dropped",
			[]string{"John Doe shouts: Hey!", "", "John Doe shouts: Wait!"},
			[]string{"John Doe shouts: Hey! Wait!"},
		},
		{"different speakers", []string{"Mason Fletcher says: X.", "John Doe says: Y."}, nil},
		{"same speaker, said differently", []string{"John Doe says: X.", "John Doe shouts: Y."}, nil},
		{"an emote", []string{"* John Doe waves.", "* John Doe smiles."}, nil},
		{"OOC", []string{"(( John Doe: brb ))", "(( John Doe: back ))"}, nil},
		{"one line", []string{"John Doe says: X."}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d := newTextDocument(test.lines...)
			last := len(test.lines) - 1
			if got := d.CanMergeLines(0, last); got != (test.want != nil) {
				t.Fatalf("CanMergeLines = %v, want %v", got, test.want != nil)
			}
			caret, ok := d.MergeLines(0, last)
			if !ok {
				if got := plainText(d); !slices.Equal(got, test.lines) {
					t.Errorf("unmergeable lines changed to %q", got)
				}
				return
			}
			if got := plainText(d); !slices.Equal(got, test.want) {
				t.Errorf("text = %q, want %q", got, test.want)
			}
			if wantCaret := at(0, len([]rune(test.want[0]))); caret != wantCaret {
				t.Errorf("caret = %v, want %v", caret, wantCaret)
			}
		})
	}
}

func TestDocumentMergeLinesKeepsOtherLinesAndStyles(t *testing.T) {
	d := NewDocument([]Line{
		sayLine("John Doe says: before"),
		sayLine("Jane Roe says: /Hi/."),
		NewLine("Jane Roe says: *nods*", say, emote),
		sayLine("John Doe says: after"),
	})
	if _, ok := d.MergeLines(1, 2); !ok {
		t.Fatal("MergeLines failed")
	}
	want := []string{"John Doe says: before", "Jane Roe says: Hi. nods", "John Doe says: after"}
	if got := plainText(d); !slices.Equal(got, want) {
		t.Fatalf("text = %q, want %q", got, want)
	}
	wantSpans := []Span{{"Jane Roe says: ", say, false}, {"Hi", say, true}, {".", say, false}, {" nods", emote, false}}
	if got := d.Lines()[1].Spans; !slices.Equal(got, wantSpans) {
		t.Errorf("spans = %v, want %v", got, wantSpans)
	}
}

func TestPictureLineBounds(t *testing.T) {
	picture := render(t, []Line{sayLine("one"), sayLine("two")}, Style{})
	if got := picture.LineCount(); got != 2 {
		t.Fatalf("LineCount = %d, want 2", got)
	}
	first, ok := picture.LineBounds(0)
	second, ok2 := picture.LineBounds(1)
	if !ok || !ok2 {
		t.Fatal("LineBounds missing a line")
	}
	if first.Max.Y != second.Min.Y || first.Dx() != picture.Image.Bounds().Dx() {
		t.Errorf("bounds = %v, %v: want stacked bands across the image", first, second)
	}
	if _, ok := picture.LineBounds(2); ok {
		t.Error("LineBounds(2) found a line that isn't there")
	}
}
