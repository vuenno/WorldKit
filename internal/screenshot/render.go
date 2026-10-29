// Package screenshot turns chat lines into transparent PNG images that players lay over game
// screenshots, and keeps the saved images in one folder.
package screenshot

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
)

// Line is one chat line to draw, as styled spans (see NewLine for chat text with markup).
type Line struct {
	Spans []Span
	Color color.NRGBA // the line's own chat color
}

// Style is how a whole screenshot is drawn, beyond its characters' own styles.
type Style struct {
	Look                 // the game's chat font and scale
	Backdrop color.NRGBA // the box behind each row of text; zero (transparent): no box
	// WidthLimit is the widest the image may be, in pixels (MinWidthLimit to
	// MaxWidthLimit); longer lines wrap to fit. Zero: DefaultWidthLimit, the in-game chat
	// box's width.
	WidthLimit int
}

// The width limits a screenshot can be drawn with, in pixels.
const (
	MinWidthLimit = 200
	MaxWidthLimit = 1800
)

// DefaultWidthLimit is the in-game chat box's width as an image width: lines wrap where
// they do in game, whatever the chat scale.
func DefaultWidthLimit() int {
	return chatTextWidthPx + 2*marginPx
}

// ClampWidthLimit returns limit within MinWidthLimit..MaxWidthLimit, and DefaultWidthLimit
// when limit is zero.
func ClampWidthLimit(limit int) int {
	if limit == 0 {
		return DefaultWidthLimit()
	}
	return max(MinWidthLimit, min(limit, MaxWidthLimit))
}

// The look follows the in-game chat (see chat.go): white bold text with a soft black
// shadow on a transparent background, so it stays readable over any screenshot.
const (
	paddingPx = 5
	// backdropPaddingPx is how far a backdrop box reaches past its row's text on the left
	// and right, as in the Chatlog Magician (padding 0 5px).
	backdropPaddingPx = 5
	marginPx          = paddingPx + shadowReachPx // from the image edge to the text
)

// ErrNoLines is returned when there is nothing to draw.
var ErrNoLines = errors.New("no lines to draw")

// Render draws lines top to bottom, wrapping long ones, on a transparent image just big
// enough to hold them, as the game's chat draws them. With a backdrop, each row gets a box
// hugging its text. The picture also records where each character landed, for editing on
// the image.
func Render(lines []Line, style Style) (*Picture, error) {
	if len(lines) == 0 {
		return nil, ErrNoLines
	}
	geometry := geometryFor(style.Look)
	faces := facesFor(geometry.look.Font, geometry.drawPx)
	widthLimit := ClampWidthLimit(style.WidthLimit)
	rows := layoutRows(faces, lines, float64(widthLimit-2*marginPx))
	picture := &Picture{Look: geometry.look, rows: rows, grid: chatGrid(geometry)}
	textRight := 0.0
	for _, row := range rows {
		textRight = max(textRight, textOriginX+row.width)
	}
	width := min(int(math.Ceil(textRight))+marginPx, widthLimit)
	height := int(math.Ceil(picture.grid.rowTop(len(rows)))) + marginPx
	picture.Image = image.NewRGBA(image.Rect(0, 0, width, height))
	if style.Backdrop.A > 0 {
		drawBackdrop(picture.Image, rows, picture.grid, style.Backdrop)
	}
	ascent, descent := faces.regular.verticalMetrics()
	baselineOffset := geometry.baselineOffset(ascent, descent)
	for index, row := range rows {
		baseline := int(math.Floor(picture.grid.rowTop(index) + baselineOffset + 0.5))
		if err := drawRow(picture.Image, row, baseline); err != nil {
			return nil, err
		}
	}
	return picture, nil
}

// textOriginX is where each row's text starts: the margin, plus the fraction of a pixel
// the in-game chat's text starts at.
const textOriginX = float64(marginPx) + chatTextLeftFraction

// RowGrid is where a screenshot's rows of text are, in image pixels: the first row's top
// and every row's height. Rows can start between pixels, as they do in game.
type RowGrid struct {
	Top, Height float64
	// Reach is how far the text's shadow (or outline) reaches past its letters.
	Reach int
}

// chatGrid is the grid Render draws a look on.
func chatGrid(geometry chatGeometry) RowGrid {
	return RowGrid{Top: float64(marginPx) + chatTextTopFraction, Height: geometry.lineHeight, Reach: shadowReachPx}
}

func (g RowGrid) rowTop(index int) float64 {
	return g.Top + float64(index)*g.Height
}

// rowSpan is the whole pixels row index covers: from its top to the next row's top.
func (g RowGrid) rowSpan(index int) (top, bottom int) {
	return int(math.Round(g.rowTop(index))), int(math.Round(g.rowTop(index + 1)))
}

// drawRow draws a row's text with its baseline at y baseline: run by run (characters of
// one color and style, as the game's chat puts them in one element), each run's shadow and
// then its glyphs.
func drawRow(canvas *image.RGBA, row textRow, baseline int) error {
	for start := 0; start < len(row.chars); {
		end := start + 1
		for end < len(row.chars) && sameRun(row.chars[start].char, row.chars[end].char) {
			end++
		}
		glyphs, err := placeGlyphs(row.chars[start:end], baseline)
		if err != nil {
			return err
		}
		drawShadow(canvas, glyphs)
		drawGlyphs(canvas, glyphs, row.chars[start].char.Color)
		start = end
	}
	return nil
}

func sameRun(a, b Char) bool {
	return a.Color == b.Color && a.IsItalic == b.IsItalic
}

// placeGlyphs draws each character's glyph where it lands: its x rounded to the nearest
// quarter pixel, as Chromium does.
func placeGlyphs(chars []placedChar, baseline int) ([]placedGlyph, error) {
	var glyphs []placedGlyph
	for _, placed := range chars {
		if placed.face == nil {
			continue
		}
		quarters := int(math.Floor((textOriginX+placed.x)*subpixelSteps + 0.5))
		penX := floorDiv(quarters, subpixelSteps)
		mask, err := glyphMask(placed.face, placed.glyph, quarters-penX*subpixelSteps)
		if err != nil {
			return nil, fmt.Errorf("draw %q: %w", placed.char.R, err)
		}
		if !mask.Rect.Empty() {
			glyphs = append(glyphs, placedGlyph{mask: mask, origin: image.Pt(penX, baseline)})
		}
	}
	return glyphs, nil
}

func floorDiv(a, b int) int {
	q := a / b
	if a%b != 0 && (a < 0) != (b < 0) {
		q--
	}
	return q
}

// drawGlyphs draws the glyphs in c, each through c's coverage table.
func drawGlyphs(canvas *image.RGBA, glyphs []placedGlyph, c color.NRGBA) {
	table := coverageTable(c)
	paint := image.NewUniform(c)
	for _, g := range glyphs {
		tinted := image.NewAlpha(g.bounds())
		for i, v := range g.mask.Pix {
			tinted.Pix[i] = table[v]
		}
		draw.DrawMask(canvas, tinted.Rect, paint, image.Point{}, tinted, tinted.Rect.Min, draw.Over)
	}
}

// drawBackdrop fills one box per row with text, each as wide as that row's text plus
// padding. Rows sit flush, so a block of lines reads as one stepped shape, like the
// Chatlog Magician's per-line background.
func drawBackdrop(canvas *image.RGBA, rows []textRow, grid RowGrid, backdrop color.NRGBA) {
	fill := image.NewUniform(backdrop)
	for index, row := range rows {
		if len(row.chars) == 0 {
			continue
		}
		top, bottom := grid.rowSpan(index)
		left := int(math.Floor(textOriginX)) - backdropPaddingPx
		right := int(math.Ceil(textOriginX+row.width)) + backdropPaddingPx
		draw.Draw(canvas, image.Rect(left, top, right, bottom), fill, image.Point{}, draw.Src)
	}
}

// RowBoxes finds the backdrop boxes of an image drawn on grid: one per row that has text,
// placed as Render places them. It lets a viewer show a backdrop behind screenshots saved
// without one.
func RowBoxes(img image.Image, grid RowGrid) []image.Rectangle {
	bounds := img.Bounds()
	var boxes []image.Rectangle
	for index := 0; ; index++ {
		top, bottom := grid.rowSpan(index)
		top, bottom = top+bounds.Min.Y, bottom+bounds.Min.Y
		if bottom > bounds.Max.Y-marginPx || grid.Height <= 0 {
			return boxes
		}
		left, right, hasInk := inkRange(img, top, bottom)
		if !hasInk {
			continue
		}
		// The ink includes the shadow, which reaches past the text.
		box := image.Rect(left+grid.Reach-backdropPaddingPx, top, right-grid.Reach+backdropPaddingPx, bottom)
		boxes = append(boxes, box.Intersect(bounds))
	}
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
