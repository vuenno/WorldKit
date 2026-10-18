package ui

import (
	"image"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"

	"world-kit/internal/screenshot"
)

// maxUndoSteps bounds the undo history; each step is a copy of a few chat lines.
const maxUndoSteps = 200

// pictureEditor shows a rendered screenshot and edits its text right on the image: click
// to place the caret, drag or Shift+click to select, type to change the text, Ctrl+Z to
// undo. The extract dialog's Italic and color tools act on its selection.
type pictureEditor struct {
	doc        *screenshot.Document
	history    []*screenshot.Document // earlier versions, newest last
	selection  screenshot.Selection
	isEditable bool
	isDragging bool
	picture    *screenshot.Picture // the newest render; may trail doc by a keystroke
	image      paint.ImageOp
	imageRect  image.Rectangle // where the image was drawn last frame, in editor pixels
	scale      float32         // screen pixels per image pixel, last frame
}

// SetDocument starts editing doc, read-only until isEditable is set.
func (e *pictureEditor) SetDocument(doc *screenshot.Document) {
	*e = pictureEditor{doc: doc}
}

// SetPicture shows a new render of the document.
func (e *pictureEditor) SetPicture(picture *screenshot.Picture) {
	e.picture = picture
	e.image = paint.NewImageOp(picture.Image)
}

// Focus sends keyboard input to the editor.
func (e *pictureEditor) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: e})
}

// Restyle applies change to the selection, or to all the text when nothing is selected,
// keeping the current version for undo.
func (e *pictureEditor) Restyle(change func(doc *screenshot.Document, sel screenshot.Selection)) {
	sel := e.selection
	if sel.IsEmpty() {
		sel = e.doc.All()
	}
	e.remember()
	change(e.doc, sel)
}

func (e *pictureEditor) remember() {
	e.history = append(e.history, e.doc.Clone())
	if len(e.history) > maxUndoSteps {
		e.history = e.history[1:]
	}
}

func (e *pictureEditor) undo() bool {
	last := len(e.history) - 1
	if last < 0 {
		return false
	}
	e.doc, e.history = e.history[last], e.history[:last]
	caret := e.doc.Clamp(e.selection.Caret)
	e.selection = screenshot.Selection{Anchor: caret, Caret: caret}
	return true
}

// Update handles pointer and keyboard input and reports whether the text or its look
// changed.
func (e *pictureEditor) Update(gtx layout.Context) bool {
	if !e.isEditable {
		return false
	}
	e.handlePointer(gtx)
	return e.handleKeys(gtx)
}

func (e *pictureEditor) handlePointer(gtx layout.Context) {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: e, Kinds: pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel})
		if !ok {
			return
		}
		pe, ok := ev.(pointer.Event)
		if !ok {
			continue
		}
		switch pe.Kind {
		case pointer.Press:
			e.Focus(gtx)
			e.isDragging = true
			e.moveCaret(e.positionAt(pe.Position), pe.Modifiers.Contain(key.ModShift))
		case pointer.Drag:
			if e.isDragging {
				e.moveCaret(e.positionAt(pe.Position), true)
			}
		default:
			e.isDragging = false
		}
	}
}

// positionAt maps a point in the editor to the caret position under it.
func (e *pictureEditor) positionAt(point f32.Point) screenshot.Position {
	if e.picture == nil || e.scale == 0 {
		return e.selection.Caret
	}
	x := (point.X - float32(e.imageRect.Min.X)) / e.scale
	y := (point.Y - float32(e.imageRect.Min.Y)) / e.scale
	return e.picture.PositionAt(image.Pt(int(x), int(y)))
}

// moveCaret moves the caret to pos; extending keeps the anchor, so the selection grows.
func (e *pictureEditor) moveCaret(pos screenshot.Position, isExtending bool) {
	e.selection.Caret = pos
	if !isExtending {
		e.selection.Anchor = pos
	}
}

func (e *pictureEditor) handleKeys(gtx layout.Context) bool {
	isChanged := false
	for {
		ev, ok := gtx.Event(
			key.FocusFilter{Target: e},
			key.Filter{Focus: e, Name: key.NameLeftArrow, Optional: key.ModShift},
			key.Filter{Focus: e, Name: key.NameRightArrow, Optional: key.ModShift},
			key.Filter{Focus: e, Name: key.NameHome, Optional: key.ModShift},
			key.Filter{Focus: e, Name: key.NameEnd, Optional: key.ModShift},
			key.Filter{Focus: e, Name: key.NameDeleteBackward},
			key.Filter{Focus: e, Name: key.NameDeleteForward},
			key.Filter{Focus: e, Name: key.NameReturn},
			key.Filter{Focus: e, Name: "A", Required: key.ModShortcut},
			key.Filter{Focus: e, Name: "Z", Required: key.ModShortcut},
		)
		if !ok {
			return isChanged
		}
		switch ev := ev.(type) {
		case key.EditEvent:
			e.replaceSelection(ev.Text)
			isChanged = true
		case key.Event:
			isChanged = (ev.State == key.Press && e.handleKey(ev)) || isChanged
		}
	}
}

// handleKey runs a key press and reports whether it changed the text.
func (e *pictureEditor) handleKey(ev key.Event) bool {
	caret, isShift := e.selection.Caret, ev.Modifiers.Contain(key.ModShift)
	switch ev.Name {
	case key.NameLeftArrow:
		e.moveCaret(e.doc.Move(caret, -1), isShift)
	case key.NameRightArrow:
		e.moveCaret(e.doc.Move(caret, 1), isShift)
	case key.NameHome:
		e.moveCaret(e.doc.LineEdge(caret, false), isShift)
	case key.NameEnd:
		e.moveCaret(e.doc.LineEdge(caret, true), isShift)
	case "A":
		e.selection = e.doc.All()
	case "Z":
		return e.undo()
	case key.NameReturn:
		e.replaceSelection("\n")
		return true
	case key.NameDeleteBackward, key.NameDeleteForward:
		return e.deleteSelection(ev.Name == key.NameDeleteForward)
	}
	return false
}

func (e *pictureEditor) replaceSelection(text string) {
	e.remember()
	caret := e.doc.Replace(e.selection, text)
	e.selection = screenshot.Selection{Anchor: caret, Caret: caret}
}

// deleteSelection removes the selection, or with none the character before the caret
// (after it when isForward).
func (e *pictureEditor) deleteSelection(isForward bool) bool {
	if e.selection.IsEmpty() {
		delta := -1
		if isForward {
			delta = 1
		}
		e.selection.Anchor = e.doc.Move(e.selection.Caret, delta)
		if e.selection.IsEmpty() {
			return false
		}
	}
	e.replaceSelection("")
	return true
}

// Layout fills the constraints: the picture centered (scaled down to fit when wider), the
// selection behind its text and the caret while focused. While editable, the whole area
// takes clicks, so clicking beside a short line puts the caret at its end.
func (e *pictureEditor) Layout(gtx layout.Context) layout.Dimensions {
	box := gtx.Constraints.Max
	if e.picture == nil {
		return layout.Dimensions{Size: box}
	}
	size := e.picture.Image.Bounds().Size()
	shown := fittedSize(size, box.X-2*gtx.Dp(spacingLarge))
	e.scale = float32(shown.X) / float32(size.X)
	e.imageRect = image.Rectangle{Max: shown}.Add(image.Pt((box.X-shown.X)/2, (box.Y-shown.Y)/2))
	if e.isEditable {
		area := clip.Rect{Max: box}.Push(gtx.Ops)
		event.Op(gtx.Ops, e)
		pointer.CursorText.Add(gtx.Ops)
		area.Pop()
	}
	defer op.Offset(e.imageRect.Min).Push(gtx.Ops).Pop()
	for _, bounds := range e.picture.SelectionBounds(e.selection) {
		paint.FillShape(gtx.Ops, withOpacity(colorPrimary, selectionOpacity), clip.Rect(e.toScreen(bounds)).Op())
	}
	layoutImageFitted(gtx, e.image, size, shown)
	if caret, ok := e.picture.CaretBounds(e.selection.Caret); ok && e.isEditable && gtx.Focused(e) {
		caret = e.toScreen(caret)
		caret.Max.X = caret.Min.X + max(1, gtx.Dp(caretWidth))
		paint.FillShape(gtx.Ops, colorTextPrimary, clip.Rect(caret).Op())
	}
	return layout.Dimensions{Size: box}
}

// toScreen scales a rectangle from image pixels to editor pixels.
func (e *pictureEditor) toScreen(r image.Rectangle) image.Rectangle {
	scale := func(v int) int { return int(float32(v) * e.scale) }
	return image.Rect(scale(r.Min.X), scale(r.Min.Y), scale(r.Max.X), scale(r.Max.Y))
}
