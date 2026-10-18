package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// layoutWrapped lays children out left to right, `gap` apart, starting a new row (also `gap`
// below) whenever the next child doesn't fit. Gio has no built-in wrapping layout.
func layoutWrapped(gtx layout.Context, gap unit.Dp, children []layout.Widget) layout.Dimensions {
	gapPx := gtx.Dp(gap)
	maxWidth := gtx.Constraints.Max.X
	childGtx := gtx
	childGtx.Constraints = layout.Constraints{Max: gtx.Constraints.Max}

	var x, y, rowHeight, width int
	for _, child := range children {
		macro := op.Record(gtx.Ops)
		dims := child(childGtx)
		call := macro.Stop()
		if x > 0 && x+dims.Size.X > maxWidth {
			x, y, rowHeight = 0, y+rowHeight+gapPx, 0
		}
		offset := op.Offset(image.Pt(x, y)).Push(gtx.Ops)
		call.Add(gtx.Ops)
		offset.Pop()
		width = max(width, x+dims.Size.X)
		rowHeight = max(rowHeight, dims.Size.Y)
		x += dims.Size.X + gapPx
	}
	return layout.Dimensions{Size: image.Pt(width, y+rowHeight)}
}
