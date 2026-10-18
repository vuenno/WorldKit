package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/unit"
)

// layoutWrappedCentered lays children out left to right, `gap` apart, starting a new row
// (also `gap` below) whenever the next child doesn't fit, and centers each child vertically
// in its row, for rows that mix chips and plain text. Gio has no built-in wrapping layout.
func layoutWrappedCentered(gtx layout.Context, gap unit.Dp, children []layout.Widget) layout.Dimensions {
	gapPx := gtx.Dp(gap)
	maxWidth := gtx.Constraints.Max.X
	childGtx := gtx
	childGtx.Constraints = layout.Constraints{Max: gtx.Constraints.Max}

	type placed struct {
		call op.CallOp
		size image.Point
		x    int
	}
	var row []placed
	var x, y, width int
	flush := func() int {
		rowHeight := 0
		for _, child := range row {
			rowHeight = max(rowHeight, child.size.Y)
		}
		for _, child := range row {
			offset := op.Offset(image.Pt(child.x, y+(rowHeight-child.size.Y)/2)).Push(gtx.Ops)
			child.call.Add(gtx.Ops)
			offset.Pop()
		}
		row = row[:0]
		return rowHeight
	}
	for _, child := range children {
		macro := op.Record(gtx.Ops)
		dims := child(childGtx)
		call := macro.Stop()
		if x > 0 && x+dims.Size.X > maxWidth {
			y += flush() + gapPx
			x = 0
		}
		row = append(row, placed{call: call, size: dims.Size, x: x})
		width = max(width, x+dims.Size.X)
		x += dims.Size.X + gapPx
	}
	return layout.Dimensions{Size: image.Pt(width, y+flush())}
}
