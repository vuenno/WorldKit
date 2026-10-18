package ui

import "testing"

func TestChatLineListSurvivesScrollingPastTheEdges(t *testing.T) {
	list := newChatLineList()
	list.SetLines(make([]chatLine, 5), true)
	for i := range list.rowHeights {
		list.rowHeights[i] = 20
	}
	list.viewSize.Y = 60
	tests := []struct {
		name  string
		first int
		y     float32
		want  int
	}{
		// A drag above the top of a list already at its top scrolls First to -1.
		{"scrolled above the top", -1, -7, 0},
		{"scrolled past the end", 9, 500, 4},
		{"in range", 1, 25, 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			list.list.Position.First = test.first
			if got := list.lineAt(test.y); got != test.want {
				t.Errorf("lineAt(%v) = %d, want %d", test.y, got, test.want)
			}
			list.lineBounds(test.want) // must not panic either
		})
	}
}
