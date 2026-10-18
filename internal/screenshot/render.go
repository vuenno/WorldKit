// Package screenshot turns chat lines into transparent PNG images that players lay over game
// screenshots, and keeps the saved images in one folder.
package screenshot

import (
	"errors"
	"image"
	"image/color"
	"image/draw"

	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Line is one chat line to draw, as styled spans (see NewLine for chat text with markup).
type Line struct {
	Spans []Span
	Color color.NRGBA // the line's own chat color
}

// Style is how a whole screenshot is drawn, beyond its characters' own styles.
type Style struct {
	Backdrop color.NRGBA // the box behind each row of text; zero (transparent): no box
}

// The look follows the in-game chat: bold text with a black outline on a transparent
// background, so it stays readable over any screenshot.
const (
	fontSizePx     = 14
	lineHeightPx   = 18
	paddingPx      = 6
	outlinePx      = 1
	maxTextWidthPx = 900 // longer lines wrap, like the in-game chat box
	// backdropPaddingPx is how far a backdrop box reaches past its row's text on the left
	// and right, as in the Chatlog Magician (padding 0 5px).
	backdropPaddingPx = 5
	marginPx          = paddingPx + outlinePx // from the image edge to the text
)

var outlineColor = color.NRGBA{A: 0xFF}

// ErrNoLines is returned when there is nothing to draw.
var ErrNoLines = errors.New("no lines to draw")

// Render draws lines top to bottom, wrapping long ones, on a transparent image just big
// enough to hold them. With a backdrop, each row gets a box hugging its text. The picture
// also records where each character landed, for editing on the image.
func Render(lines []Line, style Style) (*Picture, error) {
	if len(lines) == 0 {
		return nil, ErrNoLines
	}
	faces, err := newFaces()
	if err != nil {
		return nil, err
	}
	defer faces.Close()

	rows := layoutRows(faces, lines)
	width := 0
	for _, row := range rows {
		width = max(width, row.width)
	}
	canvas := image.NewRGBA(image.Rect(0, 0, width+2*marginPx, len(rows)*lineHeightPx+2*marginPx))
	if style.Backdrop.A > 0 {
		drawBackdrop(canvas, rows, style.Backdrop)
	}
	baselineOffset := (lineHeightPx + faces.regular.Metrics().CapHeight.Ceil()) / 2
	for index, row := range rows {
		origin := fixed.P(marginPx, marginPx+index*lineHeightPx+baselineOffset)
		drawOutlinedRow(canvas, faces, row, origin)
	}
	return &Picture{Image: canvas, rows: rows}, nil
}

// drawBackdrop fills one box per row with text, each as wide as that row's text plus
// padding. Rows sit flush, so a block of lines reads as one stepped shape, like the
// Chatlog Magician's per-line background.
func drawBackdrop(canvas *image.RGBA, rows []textRow, backdrop color.NRGBA) {
	fill := image.NewUniform(backdrop)
	for index, row := range rows {
		if len(row.chars) > 0 {
			draw.Draw(canvas, rowBox(index, row.width), fill, image.Point{}, draw.Src)
		}
	}
}

// rowBox is the backdrop box of the row at index whose text is width pixels wide.
func rowBox(index, width int) image.Rectangle {
	top := marginPx + index*lineHeightPx
	return image.Rect(marginPx-backdropPaddingPx, top, marginPx+width+backdropPaddingPx, top+lineHeightPx)
}

// RowBoxes finds the backdrop boxes of an image Render drew: one per row that has text,
// placed as Render places them. It lets a viewer show a backdrop behind screenshots saved
// without one.
func RowBoxes(img image.Image) []image.Rectangle {
	bounds := img.Bounds()
	var boxes []image.Rectangle
	for top := bounds.Min.Y + marginPx; top+lineHeightPx <= bounds.Max.Y-marginPx; top += lineHeightPx {
		left, right, hasInk := inkRange(img, top, top+lineHeightPx)
		if !hasInk {
			continue
		}
		// The ink includes the outline, which reaches outlinePx past the text.
		box := image.Rect(left+outlinePx-backdropPaddingPx, top, right-outlinePx+backdropPaddingPx, top+lineHeightPx)
		boxes = append(boxes, box.Intersect(bounds))
	}
	return boxes
}

// inkRange returns the leftmost and one-past-rightmost x of non-transparent pixels in the
// rows top..bottom.
func inkRange(img image.Image, top, bottom int) (left, right int, hasInk bool) {
	bounds := img.Bounds()
	left, right = bounds.Max.X, bounds.Min.X
	for y := top; y < bottom; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if _, _, _, alpha := img.At(x, y).RGBA(); alpha == 0 {
				continue
			}
			left, right = min(left, x), max(right, x+1)
		}
	}
	return left, right, left < right
}

// drawOutlinedRow draws the row in black at every neighboring offset, then in its own
// colors on top.
func drawOutlinedRow(canvas *image.RGBA, faces *faces, row textRow, origin fixed.Point26_6) {
	for dy := -outlinePx; dy <= outlinePx; dy++ {
		for dx := -outlinePx; dx <= outlinePx; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			drawRow(canvas, faces, row, origin.Add(fixed.P(dx, dy)), func(Char) color.NRGBA { return outlineColor })
		}
	}
	drawRow(canvas, faces, row, origin, func(c Char) color.NRGBA { return c.Color })
}

// drawRow draws each of the row's characters at its place, in colorOf(char).
func drawRow(canvas *image.RGBA, faces *faces, row textRow, origin fixed.Point26_6, colorOf func(Char) color.NRGBA) {
	for _, placed := range row.chars {
		drawer := &font.Drawer{
			Dst:  canvas,
			Src:  image.NewUniform(colorOf(placed.char)),
			Face: faces.of(placed.char.IsItalic),
			Dot:  origin.Add(fixed.Point26_6{X: placed.x}),
		}
		drawer.DrawString(string(placed.char.R))
	}
}
