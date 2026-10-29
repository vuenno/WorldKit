package ui

import (
	"image/color"
	"slices"
	"testing"

	"world-kit/internal/screenshot"
)

func newTestPictureEditor(texts ...string) *pictureEditor {
	lines := make([]screenshot.Line, len(texts))
	for i, text := range texts {
		lines[i] = screenshot.NewLine(text, color.NRGBA{A: 0xFF}, color.NRGBA{A: 0xFF})
	}
	var e pictureEditor
	e.SetDocument(screenshot.NewDocument(lines))
	return &e
}

func editorTexts(e *pictureEditor) []string {
	var texts []string
	for _, line := range e.doc.Lines() {
		text := ""
		for _, span := range line.Spans {
			text += span.Text
		}
		texts = append(texts, text)
	}
	return texts
}

func TestPictureEditorDropLine(t *testing.T) {
	tests := []struct {
		name      string
		line, gap int
		want      []string
		isMoved   bool
	}{
		{"below the next line", 0, 2, []string{"B", "A", "C"}, true},
		{"below the last line", 0, 3, []string{"B", "C", "A"}, true},
		{"above the first line", 2, 0, []string{"C", "A", "B"}, true},
		{"just above itself", 1, 1, []string{"A", "B", "C"}, false},
		{"just below itself", 1, 2, []string{"A", "B", "C"}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			e := newTestPictureEditor("A", "B", "C")
			if got := e.dropLine(test.line, test.gap); got != test.isMoved {
				t.Errorf("dropLine = %v, want %v", got, test.isMoved)
			}
			if got := editorTexts(e); !slices.Equal(got, test.want) {
				t.Errorf("text = %q, want %q", got, test.want)
			}
		})
	}
}

func TestPictureEditorMoveCaretLineStopsAtTheEdges(t *testing.T) {
	e := newTestPictureEditor("A", "B")
	if e.moveCaretLine(false) {
		t.Error("moved the first line up")
	}
	if !e.moveCaretLine(true) || !slices.Equal(editorTexts(e), []string{"B", "A"}) {
		t.Errorf("text = %q, want the first line moved down", editorTexts(e))
	}
	if e.selection.Caret.Line != 1 {
		t.Errorf("caret on line %d, want it to follow the line to 1", e.selection.Caret.Line)
	}
}

func TestPictureEditorMergeSelectionUndoes(t *testing.T) {
	e := newTestPictureEditor("Mason Fletcher says: X.", "Mason Fletcher says: Y.")
	if e.CanMergeSelection() {
		t.Error("can merge with nothing selected")
	}
	e.selection = e.doc.All()
	if !e.MergeSelection() {
		t.Fatal("MergeSelection failed")
	}
	if got, want := editorTexts(e), []string{"Mason Fletcher says: X. Y."}; !slices.Equal(got, want) {
		t.Errorf("text = %q, want %q", got, want)
	}
	e.undo()
	if got := len(editorTexts(e)); got != 2 {
		t.Errorf("after undo %d lines, want 2", got)
	}
}
