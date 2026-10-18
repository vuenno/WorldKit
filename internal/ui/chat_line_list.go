package ui

import (
	"image"
	"io"
	"strings"

	"gioui.org/io/clipboard"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// chatLineList is a scrollable list of colored chat lines where whole lines can be selected:
// press and drag across lines, Shift+click to extend, Ctrl+A for all, Ctrl+C to copy.
// Once a selection is made, a small menu pops up over it (selectionMenu).
// Lines are drawn one label each so every line keeps its own color; a text editor would
// allow partial-line selection but can only draw one color.
type chatLineList struct {
	lines      []chatLine
	list       widget.List
	selection  lineSelection
	menu       selectionMenu
	rowHeights []int // last laid-out height of each line, 0 if not on screen yet
	viewSize   image.Point
}

func newChatLineList() *chatLineList {
	return &chatLineList{list: widget.List{List: layout.List{Axis: layout.Vertical}}}
}

// SetLines replaces the lines and clears the selection.
func (l *chatLineList) SetLines(lines []chatLine) {
	l.lines = lines
	l.rowHeights = make([]int, len(lines))
	l.selection.Clear()
	l.menu.Close()
}

// Len returns the number of lines.
func (l *chatLineList) Len() int {
	return len(l.lines)
}

// SelectedCount returns how many lines are selected.
func (l *chatLineList) SelectedCount() int {
	return len(l.SelectedLines())
}

// SelectedLines returns the selected lines, or nil when none are selected.
func (l *chatLineList) SelectedLines() []chatLine {
	first, last, ok := l.selection.Range()
	if !ok {
		return nil
	}
	return l.lines[first : last+1]
}

// ExtractClicked reports whether Extract was clicked in the selection menu.
func (l *chatLineList) ExtractClicked(gtx layout.Context) bool {
	return l.menu.ExtractClicked(gtx)
}

// CopyText returns the selected lines, or every line when none are selected.
func (l *chatLineList) CopyText() string {
	if selected := l.SelectedLines(); selected != nil {
		return joinChatLines(selected)
	}
	return joinChatLines(l.lines)
}

// Update handles mouse selection and shortcuts. It reports whether Ctrl+C asked for a copy.
func (l *chatLineList) Update(gtx layout.Context) bool {
	isCopyRequested := false
	for {
		ev, ok := gtx.Event(
			pointer.Filter{Target: l, Kinds: pointer.Press | pointer.Drag | pointer.Release},
			key.FocusFilter{Target: l},
			key.Filter{Focus: l, Name: "C", Required: key.ModShortcut},
			key.Filter{Focus: l, Name: "A", Required: key.ModShortcut},
		)
		if !ok {
			return isCopyRequested
		}
		switch ev := ev.(type) {
		case pointer.Event:
			l.handlePointer(gtx, ev)
		case key.Event:
			isCopyRequested = l.handleKey(ev) || isCopyRequested
		}
	}
}

func (l *chatLineList) handlePointer(gtx layout.Context, ev pointer.Event) {
	if len(l.lines) == 0 {
		return
	}
	line := l.lineAt(ev.Position.Y)
	switch ev.Kind {
	case pointer.Press:
		if ev.Buttons != pointer.ButtonPrimary {
			return
		}
		gtx.Execute(key.FocusCmd{Tag: l})
		l.menu.Close()
		l.selection.Press(line, ev.Modifiers.Contain(key.ModShift))
	case pointer.Drag:
		l.scrollWhileDraggingPastEdge(ev.Position.Y)
		l.selection.DragTo(line)
	case pointer.Release:
		if l.SelectedLines() != nil {
			l.menu.Open(int(ev.Position.X))
		}
	}
}

func (l *chatLineList) handleKey(ev key.Event) bool {
	if ev.State != key.Press {
		return false
	}
	if ev.Name == "A" {
		l.selection.SelectAll(len(l.lines))
		l.menu.Open(l.viewSize.X / 2)
		return false
	}
	return ev.Name == "C"
}

// scrollWhileDraggingPastEdge lets a drag select lines that are scrolled out of view.
func (l *chatLineList) scrollWhileDraggingPastEdge(y float32) {
	switch {
	case y < 0:
		l.list.List.ScrollBy(-1)
	case y > float32(l.viewSize.Y):
		l.list.List.ScrollBy(1)
	}
}

// lineAt returns the line drawn at y (relative to the list's top) in the last frame,
// clamped to the first and last line.
func (l *chatLineList) lineAt(y float32) int {
	position := l.list.Position
	line := min(position.First, len(l.lines)-1)
	top := -position.Offset
	for line < len(l.lines)-1 && l.rowHeights[line] > 0 && y >= float32(top+l.rowHeights[line]) {
		top += l.rowHeights[line]
		line++
	}
	return line
}

// lineBounds returns the top and bottom y of line as drawn in the last frame, clamped to
// the view: a line scrolled out above sits at 0, one below at the view's bottom.
func (l *chatLineList) lineBounds(line int) (top, bottom int) {
	if line < l.list.Position.First {
		return 0, 0
	}
	position := l.list.Position
	y := -position.Offset
	for index := position.First; index < line; index++ {
		y += l.rowHeights[index]
		if l.rowHeights[index] == 0 || y >= l.viewSize.Y {
			return l.viewSize.Y, l.viewSize.Y
		}
	}
	return max(0, y), min(y+l.rowHeights[line], l.viewSize.Y)
}

// Layout draws the visible lines with a scrollbar, then the selection menu over them.
func (l *chatLineList) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	l.viewSize = gtx.Constraints.Max
	dims := l.layoutLines(gtx, theme)
	l.layoutMenu(gtx, theme)
	return dims
}

// layoutLines draws the lines and registers the selection area.
func (l *chatLineList) layoutLines(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	// The list draws inside this area, so presses on lines reach the selection handler
	// as well as the list's own scroll handling.
	defer clip.Rect(image.Rectangle{Max: gtx.Constraints.Max}).Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, l)
	pointer.CursorText.Add(gtx.Ops)

	list := material.List(theme, &l.list)
	list.Indicator.Color = colorBorder
	list.Indicator.HoverColor = colorTextSecondary
	return list.Layout(gtx, len(l.lines), func(gtx layout.Context, index int) layout.Dimensions {
		dims := l.lines[index].Layout(gtx, theme, l.selection.Contains(index))
		l.rowHeights[index] = dims.Size.Y
		return dims
	})
}

// layoutMenu draws the selection menu next to the selected lines. It is drawn outside the
// lines' area, so clicks on it don't also reach the selection handler.
func (l *chatLineList) layoutMenu(gtx layout.Context, theme *material.Theme) {
	first, last, ok := l.selection.Range()
	if !l.menu.isOpen || !ok {
		return
	}
	recording := op.Record(gtx.Ops)
	menuDims := l.menu.Layout(gtx, theme)
	menu := recording.Stop()

	top, _ := l.lineBounds(first)
	_, bottom := l.lineBounds(last)
	origin := selectionMenuOrigin(l.viewSize, menuDims.Size, l.menu.anchorX, top, bottom, gtx.Dp(spacingTiny))
	defer op.Offset(origin).Push(gtx.Ops).Pop()
	menu.Add(gtx.Ops)
}

// copyToClipboard puts text on the system clipboard.
func copyToClipboard(gtx layout.Context, text string) {
	gtx.Execute(clipboard.WriteCmd{Type: "application/text", Data: io.NopCloser(strings.NewReader(text))})
}
