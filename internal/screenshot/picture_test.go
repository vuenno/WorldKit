package screenshot

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func render(t *testing.T, lines []Line, style Style) *Picture {
	t.Helper()
	picture, err := Render(lines, style)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	return picture
}

// rowMiddle is the y halfway down the picture's row.
func rowMiddle(picture *Picture, row int) int {
	top, bottom := picture.grid.rowSpan(row)
	return (top + bottom) / 2
}

func TestRenderBackdrop(t *testing.T) {
	lines := []Line{sayLine("A long first line of text"), sayLine("short")}
	tests := []struct {
		name     string
		backdrop color.NRGBA
	}{
		{"transparent", color.NRGBA{}},
		{"dark", black},
		{"light", color.NRGBA{R: 0xD9, G: 0xDC, B: 0xE1, A: 0xFF}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			picture := render(t, lines, Style{Backdrop: test.backdrop})
			img := picture.Image
			// Corners stay transparent: the backdrop hugs the text, it doesn't fill the image.
			if corner := img.RGBAAt(0, 0); corner.A != 0 {
				t.Errorf("corner = %v, want transparent", corner)
			}
			// Right of the short second row is outside its box.
			if pixel := img.RGBAAt(img.Bounds().Dx()-marginPx, rowMiddle(picture, 1)); pixel.A != 0 {
				t.Errorf("beside the short row = %v, want transparent", pixel)
			}
			// Left of the text, inside the first row's padding, is the backdrop.
			top, _ := picture.grid.rowSpan(0)
			inPadding := img.RGBAAt(marginPx-backdropPaddingPx+1, top+1)
			if want := test.backdrop.A; inPadding.A != want {
				t.Errorf("row padding alpha = %d, want %d", inPadding.A, want)
			}
		})
	}
}

func TestRenderEmptyLineHasNoBackdrop(t *testing.T) {
	picture := render(t, []Line{sayLine("text"), sayLine("")}, Style{Backdrop: black})
	if pixel := picture.Image.RGBAAt(marginPx, rowMiddle(picture, 1)); pixel.A != 0 {
		t.Errorf("empty row = %v, want transparent", pixel)
	}
}

func TestRowBoxesMatchRender(t *testing.T) {
	lines := []Line{sayLine("A long first line of text"), sayLine("short")}
	transparent := render(t, lines, Style{})
	backed := render(t, lines, Style{Backdrop: black}).Image
	boxes := RowBoxes(transparent.Image, transparent.grid)
	if len(boxes) != 2 {
		t.Fatalf("RowBoxes = %v, want 2 boxes", boxes)
	}
	for _, box := range boxes {
		// A pixel or two of side bearing may differ; the box must sit on the drawn one.
		inner := image.Rect(box.Min.X+2, box.Min.Y, box.Max.X-2, box.Max.Y)
		for _, corner := range []image.Point{inner.Min, {inner.Max.X - 1, inner.Max.Y - 1}} {
			if backed.RGBAAt(corner.X, corner.Y).A == 0 {
				t.Errorf("box %v corner %v is outside the rendered backdrop", box, corner)
			}
		}
	}
	if boxes[1].Dx() >= boxes[0].Dx() {
		t.Errorf("short row box %v not narrower than %v", boxes[1], boxes[0])
	}
}

func TestPicturePositionAt(t *testing.T) {
	picture := render(t, []Line{sayLine("Hello there"), sayLine("Hi"), sayLine("")}, Style{})
	tests := []struct {
		name  string
		point image.Point
		want  Position
	}{
		{"left of the text", image.Pt(0, rowMiddle(picture, 0)), at(0, 0)},
		{"above the image", image.Pt(0, -50), at(0, 0)},
		{"right of the text", image.Pt(10_000, rowMiddle(picture, 0)), at(0, 11)},
		{"second line end", image.Pt(10_000, rowMiddle(picture, 1)), at(1, 2)},
		{"empty line", image.Pt(30, rowMiddle(picture, 2)), at(2, 0)},
		{"below the image", image.Pt(30, 10_000), at(2, 0)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := picture.PositionAt(test.point); got != test.want {
				t.Errorf("PositionAt(%v) = %v, want %v", test.point, got, test.want)
			}
		})
	}
}

func TestPictureCaretRoundTrip(t *testing.T) {
	picture := render(t, []Line{sayLine("Hello there")}, Style{})
	previousX := -1
	for index := 0; index <= 11; index++ {
		caret, ok := picture.CaretBounds(at(0, index))
		if !ok {
			t.Fatalf("CaretBounds(%d) not found", index)
		}
		if caret.Min.X <= previousX {
			t.Errorf("caret %d at x %d, not right of %d", index, caret.Min.X, previousX)
		}
		previousX = caret.Min.X
		if got := picture.PositionAt(image.Pt(caret.Min.X, caret.Min.Y+1)); got != at(0, index) {
			t.Errorf("PositionAt(caret %d) = %v", index, got)
		}
	}
}

func TestPictureWrappedLine(t *testing.T) {
	text := strings.Repeat("word ", 300)
	picture := render(t, []Line{sayLine(text)}, Style{})
	end := at(0, len(text))
	caret, ok := picture.CaretBounds(end)
	if secondRowTop, _ := picture.grid.rowSpan(1); !ok || caret.Min.Y < secondRowTop {
		t.Errorf("end caret = %v, %v; want on a later row", caret, ok)
	}
	if got := len(picture.SelectionBounds(Selection{Caret: end})); got < 2 {
		t.Errorf("selecting the wrapped line gives %d boxes, want one per row", got)
	}
}

func TestPictureSelectionBounds(t *testing.T) {
	picture := render(t, []Line{sayLine("Hello there"), sayLine("Hi")}, Style{})
	tests := []struct {
		name string
		sel  Selection
		want int
	}{
		{"nothing", span(at(0, 3), at(0, 3)), 0},
		{"one word", span(at(0, 0), at(0, 5)), 1},
		{"across lines", span(at(1, 1), at(0, 6)), 2},
	}
	for _, test := range tests {
		if got := len(picture.SelectionBounds(test.sel)); got != test.want {
			t.Errorf("%s: %d boxes, want %d", test.name, got, test.want)
		}
	}
}

func TestRenderItalicFallsBackWhenArialIsMissing(t *testing.T) {
	t.Setenv("WINDIR", t.TempDir())
	render(t, []Line{sayLine("I /really/ mean it")}, Style{})
}
