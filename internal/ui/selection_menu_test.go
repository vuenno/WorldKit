package ui

import (
	"image"
	"testing"
)

func TestSelectionMenuOrigin(t *testing.T) {
	view := image.Pt(800, 400)
	menu := image.Pt(100, 40)
	tests := []struct {
		name                          string
		anchorX                       int
		selectionTop, selectionBottom int
		want                          image.Point
	}{
		{"above the selection, centered", 300, 200, 260, image.Pt(250, 156)},
		{"below when no room above", 300, 20, 100, image.Pt(250, 104)},
		{"inside the view when the selection fills it", 300, 0, 400, image.Pt(250, 360)},
		{"kept inside the left edge", 10, 200, 220, image.Pt(0, 156)},
		{"kept inside the right edge", 790, 200, 220, image.Pt(700, 156)},
		{"selection scrolled below the view", 300, 400, 400, image.Pt(250, 356)},
		{"selection scrolled above the view", 300, 0, 0, image.Pt(250, 4)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := selectionMenuOrigin(view, menu, test.anchorX, test.selectionTop, test.selectionBottom, 4)
			if got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
