// Package screenshot turns chat lines into transparent PNG images that players lay over game
// screenshots, and keeps the saved images in one folder.
package screenshot

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Line is one chat line to draw, in its in-game color.
type Line struct {
	Text  string
	Color color.NRGBA
}

// The look follows the in-game chat: bold text with a black outline on a transparent
// background, so it stays readable over any screenshot.
const (
	fontSizePx     = 14
	lineHeightPx   = 18
	paddingPx      = 6
	outlinePx      = 1
	maxTextWidthPx = 900 // longer lines wrap, like the in-game chat box
)

var outlineColor = color.NRGBA{A: 0xFF}

// ErrNoLines is returned when there is nothing to draw.
var ErrNoLines = errors.New("no lines to draw")

// Render draws lines top to bottom, wrapping long ones, on a transparent image just big
// enough to hold them.
func Render(lines []Line) (*image.RGBA, error) {
	if len(lines) == 0 {
		return nil, ErrNoLines
	}
	face, err := newFace()
	if err != nil {
		return nil, err
	}
	defer face.Close()

	rows := wrapLines(face, lines, maxTextWidthPx)
	width := 0
	for _, row := range rows {
		width = max(width, font.MeasureString(face, row.Text).Ceil())
	}
	margin := paddingPx + outlinePx
	canvas := image.NewRGBA(image.Rect(0, 0, width+2*margin, len(rows)*lineHeightPx+2*margin))
	baselineOffset := (lineHeightPx + face.Metrics().CapHeight.Ceil()) / 2
	for index, row := range rows {
		origin := fixed.P(margin, margin+index*lineHeightPx+baselineOffset)
		drawOutlinedText(canvas, face, row, origin)
	}
	return canvas, nil
}

func newFace() (font.Face, error) {
	face, err := opentype.NewFace(loadFont(), &opentype.FaceOptions{Size: fontSizePx, DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return nil, fmt.Errorf("create screenshot font face: %w", err)
	}
	return face, nil
}

// loadFont returns Arial Bold, the font the in-game chat look uses. Its license doesn't
// allow bundling it, but every Windows install has it, so it is read from the Windows fonts
// folder. If that fails, the bundled Go Bold keeps Extract working.
func loadFont() *opentype.Font {
	path := filepath.Join(windowsDir(), "Fonts", "arialbd.ttf")
	parsed, err := parseFontFile(path)
	if err == nil {
		return parsed
	}
	slog.Warn("load screenshot font, using Go Bold instead", "path", path, "error", err)
	parsed, err = opentype.Parse(gobold.TTF)
	if err != nil {
		panic(fmt.Sprintf("parse bundled Go Bold font: %v", err)) // the bundled font is always valid
	}
	return parsed
}

func parseFontFile(path string) (*opentype.Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return parsed, nil
}

func windowsDir() string {
	if dir := os.Getenv("WINDIR"); dir != "" {
		return dir
	}
	return `C:\Windows`
}

// drawOutlinedText draws the text in black at every neighboring offset, then in its own
// color on top.
func drawOutlinedText(canvas *image.RGBA, face font.Face, line Line, origin fixed.Point26_6) {
	outline := &font.Drawer{Dst: canvas, Src: image.NewUniform(outlineColor), Face: face}
	for dy := -outlinePx; dy <= outlinePx; dy++ {
		for dx := -outlinePx; dx <= outlinePx; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			outline.Dot = origin.Add(fixed.P(dx, dy))
			outline.DrawString(line.Text)
		}
	}
	text := &font.Drawer{Dst: canvas, Src: image.NewUniform(line.Color), Face: face, Dot: origin}
	text.DrawString(line.Text)
}

// wrapLines splits lines wider than maxWidth at spaces; each piece keeps its line's color.
// A single word wider than maxWidth stays on its own row rather than being cut.
func wrapLines(face font.Face, lines []Line, maxWidth int) []Line {
	var rows []Line
	for _, line := range lines {
		for _, text := range wrapText(face, line.Text, maxWidth) {
			rows = append(rows, Line{Text: text, Color: line.Color})
		}
	}
	return rows
}

func wrapText(face font.Face, text string, maxWidth int) []string {
	words := strings.Split(text, " ")
	var rows []string
	current := words[0]
	for _, word := range words[1:] {
		candidate := current + " " + word
		if font.MeasureString(face, candidate).Ceil() <= maxWidth {
			current = candidate
			continue
		}
		rows = append(rows, current)
		current = word
	}
	return append(rows, current)
}
