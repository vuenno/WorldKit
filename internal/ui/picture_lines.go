package ui

import (
	"image"

	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"world-kit/internal/screenshot"
)

// lineDrag is a chat line being moved by its grip, the handle beside it on the picture.
type lineDrag struct {
	isDragging bool
	isHovered  bool
	line       int     // the line being moved
	y          float32 // the pointer, in editor pixels
}

// canReorder reports whether lines can be moved: while editing a picture of two or more.
func (e *pictureEditor) canReorder() bool {
	return e.isEditable && e.picture != nil && e.picture.LineCount() > 1
}

// handleLineDrag follows the grips and reports whether a line was dropped somewhere new.
func (e *pictureEditor) handleLineDrag(gtx layout.Context) bool {
	isMoved := false
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target: &e.lineDrag,
			Kinds:  pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Enter | pointer.Leave,
		})
		if !ok {
			return isMoved
		}
		if pe, ok := ev.(pointer.Event); ok {
			isMoved = e.dragLine(pe) || isMoved
		}
	}
}

// dragLine applies one pointer event on the grips and reports whether a line moved.
func (e *pictureEditor) dragLine(pe pointer.Event) bool {
	drag := &e.lineDrag
	drag.y = pe.Position.Y
	switch pe.Kind {
	case pointer.Enter, pointer.Move:
		drag.isHovered = true
	case pointer.Leave:
		drag.isHovered = false
	case pointer.Press:
		if e.canReorder() {
			drag.isDragging, drag.line = true, e.lineAt(drag.y)
		}
	case pointer.Release:
		if drag.isDragging {
			drag.isDragging = false
			return e.dropLine(drag.line, e.dropGap(drag.y))
		}
	case pointer.Cancel:
		drag.isDragging = false
	}
	return false
}

// lineAt returns the line drawn at height y in the editor, or the nearest one.
func (e *pictureEditor) lineAt(y float32) int {
	imageY := (y - float32(e.imageRect.Min.Y)) / e.scale
	return e.picture.PositionAt(image.Pt(0, int(imageY))).Line
}

// screenLineBounds is where the line is drawn, in editor pixels.
func (e *pictureEditor) screenLineBounds(line int) (image.Rectangle, bool) {
	bounds, ok := e.picture.LineBounds(line)
	return e.toScreen(bounds).Add(e.imageRect.Min), ok
}

// dropGap returns the gap a line dropped at height y goes into: 0 above the first line,
// LineCount below the last.
func (e *pictureEditor) dropGap(y float32) int {
	count := e.picture.LineCount()
	for line := range count {
		bounds, _ := e.screenLineBounds(line)
		if y < float32(bounds.Min.Y+bounds.Max.Y)/2 {
			return line
		}
	}
	return count
}

// gapY is the height of a gap between lines, in editor pixels.
func (e *pictureEditor) gapY(gap int) int {
	if gap < e.picture.LineCount() {
		bounds, _ := e.screenLineBounds(gap)
		return bounds.Min.Y
	}
	bounds, _ := e.screenLineBounds(gap - 1)
	return bounds.Max.Y
}

// dropLine moves line into gap and reports whether that changed the order.
func (e *pictureEditor) dropLine(line, gap int) bool {
	to := gap
	if gap > line {
		to-- // the line leaves its old place first
	}
	if to == line || line >= e.doc.LineCount() {
		return false
	}
	e.moveLine(line, to)
	return true
}

// moveCaretLine moves the caret's line one up or down (Alt+↑/↓) and reports whether it moved.
func (e *pictureEditor) moveCaretLine(isDown bool) bool {
	from := e.selection.Caret.Line
	to := from - 1
	if isDown {
		to = from + 1
	}
	if to < 0 || to >= e.doc.LineCount() {
		return false
	}
	e.moveLine(from, to)
	return true
}

// moveLine moves a line to index to, keeping the current version for undo. The caret goes
// with the line.
func (e *pictureEditor) moveLine(from, to int) {
	index := 0
	if e.selection.Caret.Line == from {
		index = e.selection.Caret.Index
	}
	e.remember()
	e.doc.MoveLine(from, to)
	caret := e.doc.Clamp(screenshot.Position{Line: to, Index: index})
	e.selection = screenshot.Selection{Anchor: caret, Caret: caret}
}

// CanMergeSelection reports whether the lines the selection touches can be merged.
func (e *pictureEditor) CanMergeSelection() bool {
	if e.doc == nil || e.selection.IsEmpty() {
		return false
	}
	first, last := e.doc.SelectedLines(e.selection)
	return e.doc.CanMergeLines(first, last)
}

// MergeSelection joins the selected lines of one speaker into one (see
// screenshot.Document.MergeLines), keeping the current version for undo.
func (e *pictureEditor) MergeSelection() bool {
	if !e.CanMergeSelection() {
		return false
	}
	first, last := e.doc.SelectedLines(e.selection)
	e.remember()
	caret, _ := e.doc.MergeLines(first, last)
	e.selection = screenshot.Selection{Anchor: caret, Caret: caret}
	return true
}

// layoutGrips draws a grip beside each line, in the margin left of the picture, and takes
// the pointer there.
func (e *pictureEditor) layoutGrips(gtx layout.Context) {
	area := clip.Rect{Min: image.Pt(0, e.imageRect.Min.Y), Max: image.Pt(e.imageRect.Min.X, e.imageRect.Max.Y)}.Push(gtx.Ops)
	event.Op(gtx.Ops, &e.lineDrag)
	cursor := pointer.CursorGrab
	if e.lineDrag.isDragging {
		cursor = pointer.CursorGrabbing
	}
	cursor.Add(gtx.Ops)
	area.Pop()

	active := -1
	switch {
	case e.lineDrag.isDragging:
		active = e.lineDrag.line
	case e.lineDrag.isHovered:
		active = e.lineAt(e.lineDrag.y)
	}
	side := gtx.Dp(iconSizeTiny)
	x := e.imageRect.Min.X - gtx.Dp(spacingTiny) - side
	for line := range e.picture.LineCount() {
		bounds, _ := e.screenLineBounds(line)
		c := colorTextTertiary
		if line == active {
			c = colorTextPrimary
		}
		stack := op.Offset(image.Pt(x, (bounds.Min.Y+bounds.Max.Y-side)/2)).Push(gtx.Ops)
		iconGrip.Layout(gtx, iconSizeTiny, c)
		stack.Pop()
	}
}

// layoutLineDrag tints the line being dragged and marks where it would land.
func (e *pictureEditor) layoutLineDrag(gtx layout.Context) {
	drag := e.lineDrag
	if !drag.isDragging {
		return
	}
	if bounds, ok := e.screenLineBounds(drag.line); ok {
		paint.FillShape(gtx.Ops, withOpacity(colorPrimary, lineDragOpacity), clip.Rect(bounds).Op())
	}
	gap := e.dropGap(drag.y)
	if gap == drag.line || gap == drag.line+1 {
		return // dropping here changes nothing
	}
	thickness := gtx.Dp(dropIndicatorHeight)
	top := e.gapY(gap) - thickness/2
	bar := image.Rect(e.imageRect.Min.X-gtx.Dp(lineGripWidth), top, e.imageRect.Max.X, top+thickness)
	paint.FillShape(gtx.Ops, colorPrimary, clip.Rect(bar).Op())
}
