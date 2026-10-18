package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// selectionMenu is the small horizontal menu that pops up next to selected chat lines.
// It is placed against the selection each frame, so it follows the lines while the list
// scrolls.
type selectionMenu struct {
	extract widget.Clickable
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

// ExtractClicked reports whether Extract was clicked, closing the menu if so.
func (m *selectionMenu) ExtractClicked(gtx layout.Context) bool {
	if !m.extract.Clicked(gtx) {
		return false
	}
	m.Close()
	return true
}

// swallowPresses drops presses on the menu's padding so they don't start a new selection
// in the list below.
func (m *selectionMenu) swallowPresses(gtx layout.Context) {
	for {
		if _, ok := gtx.Event(pointer.Filter{Target: m, Kinds: pointer.Press}); !ok {
			return
		}
	}
}

// Layout draws the menu's surface and items at its natural size.
func (m *selectionMenu) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	m.swallowPresses(gtx)
	gtx.Constraints.Min = image.Point{}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			dims := outlinedSurface(gtx, colorElevatedSurface)
			defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
			event.Op(gtx.Ops, m)
			pointer.CursorDefault.Add(gtx.Ops)
			return dims
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(spacingTiny).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return menuItem{click: &m.extract, label: "Extract"}.Layout(gtx, theme)
					}),
				)
			})
		}),
	)
}

// menuItem is one text action in a popup menu; it highlights on hover.
type menuItem struct {
	click *widget.Clickable
	label string
}

func (i menuItem) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return i.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Min
				shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(cornerRadius))
				defer shape.Push(gtx.Ops).Pop()
				pointer.CursorPointer.Add(gtx.Ops)
				if i.click.Hovered() {
					paint.Fill(gtx.Ops, colorBorder)
				}
				return layout.Dimensions{Size: size}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Dp(menuItemHeight)
				inset := layout.Inset{Left: spacingMedium, Right: spacingMedium}
				return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(theme, textSizeBody, i.label)
						label.Font.Weight = font.Medium
						label.MaxLines = 1
						label.Color = colorTextPrimary
						return label.Layout(gtx)
					})
				})
			}),
		)
	})
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
