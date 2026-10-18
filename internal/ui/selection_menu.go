package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// selectionMenu is the floating horizontal bar that pops up next to selected chat lines:
// how many are selected, Copy, Extract image and a × that clears the selection. It is
// placed against the selection each frame, so it follows the lines while the list scrolls.
type selectionMenu struct {
	copy    widget.Clickable
	extract widget.Clickable
	clear   widget.Clickable
	isOpen  bool
	anchorX int // pointer x where the selection ended, in list pixels
}

func (m *selectionMenu) Open(x int) {
	m.isOpen = true
	m.anchorX = x
}

func (m *selectionMenu) Close() {
	m.isOpen = false
}

// ExtractClicked reports whether Extract image was clicked, closing the menu if so.
func (m *selectionMenu) ExtractClicked(gtx layout.Context) bool {
	if !m.extract.Clicked(gtx) {
		return false
	}
	m.Close()
	return true
}

// CopyClicked reports whether Copy was clicked. The menu stays open, so the user can
// still extract the same lines.
func (m *selectionMenu) CopyClicked(gtx layout.Context) bool {
	return m.copy.Clicked(gtx)
}

// ClearClicked reports whether × was clicked.
func (m *selectionMenu) ClearClicked(gtx layout.Context) bool {
	return m.clear.Clicked(gtx)
}

// swallowPresses drops presses on the bar's padding so they don't start a new selection
// in the list below.
func (m *selectionMenu) swallowPresses(gtx layout.Context) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: m, Kinds: pointer.Press}); !ok {
			return
		}
	}
}

// Layout draws the bar at its natural size for count selected lines.
func (m *selectionMenu) Layout(gtx layout.Context, theme *material.Theme, count int) layout.Dimensions {
	m.swallowPresses(gtx)
	gtx.Constraints.Min = image.Point{}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			dims := floatingSurface(gtx)
			defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
			event.Op(gtx.Ops, m)
			pointer.CursorDefault.Add(gtx.Ops)
			return dims
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(spacingTiny).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return m.layoutItems(gtx, theme, count)
			})
		}),
	)
}

func (m *selectionMenu) layoutItems(gtx layout.Context, theme *material.Theme, count int) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			inset := layout.Inset{Left: spacingSmall, Right: spacingMedium - spacingTiny/2}
			return inset.Layout(gtx, secondaryText(theme, plural(count, "line", "lines")).Layout)
		}),
		layout.Rigid(verticalDivider),
		layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
		layout.Rigid(ghostButton(&m.copy, "Copy").withIcon(iconCopy).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
		layout.Rigid(primaryButton(&m.extract, "Extract image").withIcon(iconImage).withHeight(controlHeightSmall).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
		layout.Rigid(iconButton(&m.clear, iconClose).withKind(buttonGhost).withHeight(controlHeightSmall).Widget(theme)),
	)
}

// verticalDivider is a short 1 dp line separating groups in a toolbar.
func verticalDivider(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Dp(borderWidth), gtx.Dp(dividerHeight))
	paint.FillShape(gtx.Ops, colorBorderStrong, clip.Rect{Max: size}.Op())
	return layout.Dimensions{Size: size}
}

// selectionMenuOrigin places a menu of menuSize above the selection (or below it when
// there is no room above), centered on anchorX, and kept inside the view, so it never
// covers the selected lines unless they fill the view.
func selectionMenuOrigin(view, menuSize image.Point, anchorX, selectionTop, selectionBottom, gap int) image.Point {
	x := anchorX - menuSize.X/2
	y := selectionTop - gap - menuSize.Y
	if y < 0 {
		y = selectionBottom + gap
	}
	return image.Pt(
		max(0, min(x, view.X-menuSize.X)),
		max(0, min(y, view.Y-menuSize.Y)),
	)
}
