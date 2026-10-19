package ui

import (
	"image"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// menuItem is one row of a dropdownMenu: a label, an optional detail at the right in Geist
// Mono (a size), or a group heading when it has no click.
type menuItem struct {
	label     string
	detail    string
	isHeading bool
}

// dropdownMenu is a floating list that opens below a button, draws above everything else
// (op.Defer) and closes on a pick or a click anywhere outside it.
type dropdownMenu struct {
	isOpen  bool
	toggle  widget.Clickable
	dismiss widget.Clickable
	list    widget.List
	clicks  []widget.Clickable
}

// Update handles the button and the rows, and returns the index of a picked item.
func (m *dropdownMenu) Update(gtx layout.Context, itemCount int) (int, bool) {
	m.growClicks(itemCount)
	if m.toggle.Clicked(gtx) {
		m.isOpen = !m.isOpen
	}
	if m.dismiss.Clicked(gtx) {
		m.isOpen = false
	}
	for i := range itemCount {
		if m.clicks[i].Clicked(gtx) {
			m.isOpen = false
			return i, true
		}
	}
	return 0, false
}

// growClicks makes sure every row has its click state; items can grow between Update and
// Layout (a preset saved in the same frame).
func (m *dropdownMenu) growClicks(itemCount int) {
	if len(m.clicks) < itemCount {
		m.clicks = append(m.clicks, make([]widget.Clickable, itemCount-len(m.clicks))...)
	}
}

// LayoutButton draws a secondary button with a chevron that opens the menu, and the menu
// below it while open.
func (m *dropdownMenu) LayoutButton(gtx layout.Context, theme *material.Theme, label string, isMono bool, items []menuItem) layout.Dimensions {
	m.growClicks(len(items))
	dims := m.toggle.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		box := surfaceBox{fill: colorSurface, border: colorBorder, radius: radiusControl, height: controlHeightCompact, paddingX: spacingMedium}
		if m.toggle.Hovered() || m.isOpen {
			box.fill = colorElevatedSurface
		}
		return box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			text := styledLabel(theme, textSizeSmall, font.Medium, colorTextPrimary, label)
			if isMono {
				text.Font.Typeface = fontMono
			}
			text.MaxLines = 1
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Rigid(text.Layout),
				layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconChevronDown.Layout(gtx, iconSizeTiny, colorTextTertiary)
				}),
			)
		})
	})
	if m.isOpen {
		m.deferMenu(gtx, theme, dims.Size.Y, items)
	}
	return dims
}

func (m *dropdownMenu) deferMenu(gtx layout.Context, theme *material.Theme, buttonHeight int, items []menuItem) {
	recording := op.Record(gtx.Ops)
	m.layoutDismissLayer(gtx)
	offset := op.Offset(image.Pt(0, buttonHeight+gtx.Dp(spacingTiny+spacingTiny/2))).Push(gtx.Ops)
	m.layoutMenu(gtx, theme, items)
	offset.Pop()
	op.Defer(gtx.Ops, recording.Stop())
}

// layoutDismissLayer covers the whole window (the menu's origin is somewhere inside it).
func (m *dropdownMenu) layoutDismissLayer(gtx layout.Context) {
	const reach = 1 << 14 // far beyond any window size, in both directions
	defer op.Offset(image.Pt(-reach, -reach)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(2*reach, 2*reach))
	m.dismiss.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	})
}

func (m *dropdownMenu) layoutMenu(gtx layout.Context, theme *material.Theme, items []menuItem) layout.Dimensions {
	gtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(menuWidth), gtx.Dp(menuMaxHeight))}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			dims := floatingSurface(gtx)
			defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
			event.Op(gtx.Ops, &m.list) // a solid area: clicks between rows don't close the menu
			return dims
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(spacingTiny).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				m.list.Axis = layout.Vertical
				return styledList(theme, &m.list).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
					return m.layoutRow(gtx, theme, index, items[index])
				})
			})
		}),
	)
}

func (m *dropdownMenu) layoutRow(gtx layout.Context, theme *material.Theme, index int, item menuItem) layout.Dimensions {
	if item.isHeading {
		inset := layout.Inset{Top: spacingSmall, Bottom: spacingTiny, Left: spacingSmall}
		return inset.Layout(gtx, groupLabel(theme, item.label).Layout)
	}
	click := &m.clicks[index]
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(controlHeightSmall))
		gtx.Constraints = layout.Exact(size)
		defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusMenu)).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		if click.Hovered() {
			paint.Fill(gtx.Ops, colorPressed)
		}
		inset := layout.Inset{Left: spacingSmall, Right: spacingSmall}
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := styledLabel(theme, textSizeSmall, font.Normal, colorTextPrimary, item.label)
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := monoText(theme, textSizeCaption, item.detail)
					label.Color = colorTextTertiary
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
			)
		})
	})
}
