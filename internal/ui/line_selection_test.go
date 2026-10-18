package ui

import "testing"

func TestLineSelection(t *testing.T) {
	type step func(s *lineSelection)
	tests := []struct {
		name      string
		steps     []step
		wantFirst int
		wantLast  int
		wantOK    bool
	}{
		{"nothing selected", nil, 0, 0, false},
		{"press selects one line", []step{func(s *lineSelection) { s.Press(4, false) }}, 4, 4, true},
		{"drag down", []step{
			func(s *lineSelection) { s.Press(2, false) },
			func(s *lineSelection) { s.DragTo(5) },
		}, 2, 5, true},
		{"drag up keeps order", []step{
			func(s *lineSelection) { s.Press(5, false) },
			func(s *lineSelection) { s.DragTo(1) },
		}, 1, 5, true},
		{"shift press extends from anchor", []step{
			func(s *lineSelection) { s.Press(3, false) },
			func(s *lineSelection) { s.Press(7, true) },
			func(s *lineSelection) { s.Press(0, true) },
		}, 0, 3, true},
		{"plain press starts over", []step{
			func(s *lineSelection) { s.Press(3, false) },
			func(s *lineSelection) { s.DragTo(6) },
			func(s *lineSelection) { s.Press(9, false) },
		}, 9, 9, true},
		{"drag without press does nothing", []step{func(s *lineSelection) { s.DragTo(3) }}, 0, 0, false},
		{"select all", []step{func(s *lineSelection) { s.SelectAll(10) }}, 0, 9, true},
		{"select all of nothing", []step{func(s *lineSelection) { s.SelectAll(0) }}, 0, 0, false},
		{"clear", []step{
			func(s *lineSelection) { s.Press(3, false) },
			func(s *lineSelection) { s.Clear() },
		}, 0, 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var selection lineSelection
			for _, step := range test.steps {
				step(&selection)
			}
			first, last, ok := selection.Range()
			if first != test.wantFirst || last != test.wantLast || ok != test.wantOK {
				t.Errorf("got %d..%d ok=%v, want %d..%d ok=%v", first, last, ok, test.wantFirst, test.wantLast, test.wantOK)
			}
			if ok && (!selection.Contains(first) || !selection.Contains(last) || selection.Contains(last+1)) {
				t.Errorf("Contains disagrees with Range %d..%d", first, last)
			}
		})
	}
}

func TestChatLineListCopyText(t *testing.T) {
	list := newChatLineList()
	list.SetLines([]chatLine{{copyText: "a"}, {copyText: "b"}, {copyText: "c"}, {copyText: "d"}}, false)
	if got := list.CopyText(); got != "a\nb\nc\nd" {
		t.Errorf("without selection got %q, want every line", got)
	}
	list.selection.Press(2, false)
	list.selection.DragTo(1)
	if got := list.CopyText(); got != "b\nc" || list.SelectedCount() != 2 {
		t.Errorf("got %q (%d selected), want %q (2)", got, list.SelectedCount(), "b\nc")
	}
	list.SetLines([]chatLine{{copyText: "x"}}, false)
	if list.SelectedCount() != 0 {
		t.Error("new lines kept the old selection")
	}
}
