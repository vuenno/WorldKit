package ui

import (
	"image"
	"image/color"

	"gioui.org/io/pointer"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
)

// Window button glyphs on a 10×10 grid, drawn with 1 dp lines like Windows' own.
const (
	glyphMinimize strokeIcon = "M0 5 H10"
	glyphMaximize strokeIcon = "M0 0 H10 V10 H0 Z"
	glyphRestore  strokeIcon = "M0 2 H8 V10 H0 Z M2 2 V0 H10 V8 H8"
	glyphClose    strokeIcon = "M0 0 L10 10 M10 0 L0 10"
)

// titleBar replaces the Windows title bar (the window is created undecorated): an empty
// strip to drag the window by, then minimize, maximize/restore and close. Windows still
// handles snapping, resizing at the edges and double-click to maximize, because the strip
// is marked as the window's move area.
type titleBar struct {
	minimize    widget.Clickable
	maximize    widget.Clickable
	close       widget.Clickable
	isMaximized bool
	perform     func(system.Action)
}

// SetMaximized keeps the maximize button's glyph in step with the window.
func (t *titleBar) SetMaximized(isMaximized bool) {
	t.isMaximized = isMaximized
}

func (t *titleBar) update(gtx layout.Context) {
	if t.minimize.Clicked(gtx) {
		t.perform(system.ActionMinimize)
	}
	if t.maximize.Clicked(gtx) {
		if t.isMaximized {
			t.perform(system.ActionUnmaximize)
		} else {
			t.perform(system.ActionMaximize)
		}
	}
	if t.close.Clicked(gtx) {
		t.perform(system.ActionClose)
	}
}

func (t *titleBar) Layout(gtx layout.Context) layout.Dimensions {
	t.update(gtx)
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, gtx.Dp(titleBarHeight))
	gtx.Constraints.Max.Y = gtx.Constraints.Min.Y
	maximizeGlyph := glyphMaximize
	if t.isMaximized {
		maximizeGlyph = glyphRestore
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Flexed(1, moveArea),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return windowButton{click: &t.minimize, glyph: glyphMinimize}.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return windowButton{click: &t.maximize, glyph: maximizeGlyph}.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return windowButton{click: &t.close, glyph: glyphClose, isClose: true}.Layout(gtx)
		}),
	)
}

// moveArea fills its minimum size with a region Windows treats as the title bar. Keep
// buttons out of it: Windows takes clicks there before the app sees them.
func moveArea(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Min
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	system.ActionInputOp(system.ActionMove).Add(gtx.Ops)
	return layout.Dimensions{Size: size}
}

// windowButton is one caption button: 46 dp wide like Windows', grey on hover, red for close.
type windowButton struct {
	click   *widget.Clickable
	glyph   strokeIcon
	isClose bool
}

func (b windowButton) Layout(gtx layout.Context) layout.Dimensions {
	return b.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(windowButtonWidth), gtx.Constraints.Min.Y)
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		glyphColor := colorTextSecondary
		if b.click.Hovered() {
			paint.Fill(gtx.Ops, b.hoverFill())
			glyphColor = colorTextPrimary
		}
		side := gtx.Dp(windowGlyphSize)
		offset := image.Pt((size.X-side)/2, (size.Y-side)/2)
		gtx.Constraints = layout.Exact(image.Pt(side, side))
		drawGlyph(gtx, b.glyph, offset, glyphColor)
		return layout.Dimensions{Size: size}
	})
}

func (b windowButton) hoverFill() color.NRGBA {
	if b.isClose {
		return colorCloseHover
	}
	return colorElevatedSurface
}

// drawGlyph strokes a 10-unit glyph at offset with a 1 dp line.
func drawGlyph(gtx layout.Context, glyph strokeIcon, offset image.Point, c color.NRGBA) {
	defer op.Offset(offset).Push(gtx.Ops).Pop()
	scale := float32(gtx.Dp(windowGlyphSize)) / 10
	drawStrokePath(gtx, string(glyph), scale, float32(gtx.Dp(windowGlyphStroke)), c)
}
