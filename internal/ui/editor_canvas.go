package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"gioui.org/f32"
	"gioui.org/io/event"
	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"

	"world-kit/internal/compose"
)

// Zoom limits, in screen pixels per canvas pixel, and the factor one wheel notch applies.
const (
	minZoom          = 0.02
	maxZoom          = 8
	zoomPerNotch     = 1.15
	wheelNotch       = 120 // Windows reports wheel movement in 120ths of a notch
	minLayerSide     = 4   // canvas pixels: layers never shrink to nothing
	nudgeStep        = 1   // canvas pixels per arrow key press
	nudgeStepShifted = 10
)

// gestureKind is what a drag on the canvas does.
type gestureKind int

const (
	gestureNone gestureKind = iota
	gestureMove
	gestureResize
	gestureRotate
	gesturePan
)

// canvasGesture is a drag in progress: what it started from, so every frame computes the
// result from the start instead of piling up small changes (no drift).
type canvasGesture struct {
	kind         gestureKind
	handle       compose.Handle
	start        compose.Layer
	startPointer compose.Vec // canvas pixels
	startScreen  f32.Point
	startPan     f32.Point
	hasChanged   bool // remembered for undo once the first drag moved something
}

// canvasView draws the composition fitted (or zoomed) in its area and turns pointer input
// into selecting, moving, resizing and rotating layers, zooming (Ctrl+wheel) and panning.
type canvasView struct {
	zoom    float32   // screen pixels per canvas pixel; 0 fits the canvas in the area
	pan     f32.Point // the canvas center's offset from the area's center while zoomed
	scale   float32   // last frame's screen pixels per canvas pixel
	origin  f32.Point // last frame's screen position of the canvas' top-left corner
	gesture canvasGesture
	hover   compose.Handle
	isOver  bool // the pointer is over a layer
	guides  compose.Guides

	// Set in Layout from the metric and the frame, for input handling next frame.
	area         image.Point
	canvasSize   image.Point
	hitRadius    float64 // canvas pixels from a handle that still grab it
	rotateOffset float64 // canvas pixels from the top edge to the rotation handle
	snapRadius   float64 // canvas pixels within which a dragged layer snaps
}

func newCanvasView() *canvasView {
	return &canvasView{guides: noGuides()}
}

func noGuides() compose.Guides {
	return compose.Guides{X: math.NaN(), Y: math.NaN()}
}

// Fit goes back to showing the whole canvas.
func (v *canvasView) Fit() {
	v.zoom, v.pan = 0, f32.Point{}
}

// ZoomPercent is the current zoom for the status caption.
func (v *canvasView) ZoomPercent() int {
	return int(math.Round(float64(v.scale) * 100))
}

// Focus sends keyboard shortcuts (Delete, arrows, …) to the canvas.
func (v *canvasView) Focus(gtx layout.Context) {
	gtx.Execute(key.FocusCmd{Tag: v})
}

func (v *canvasView) toCanvas(p f32.Point) compose.Vec {
	if v.scale == 0 {
		return compose.Vec{}
	}
	return compose.Vec{X: float64((p.X - v.origin.X) / v.scale), Y: float64((p.Y - v.origin.Y) / v.scale)}
}

func (v *canvasView) toScreen(p compose.Vec) f32.Point {
	return f32.Pt(v.origin.X+float32(p.X)*v.scale, v.origin.Y+float32(p.Y)*v.scale)
}

// Update handles pointer input from last frame's layout.
func (v *canvasView) Update(gtx layout.Context, doc *editorDoc) {
	for {
		ev, ok := gtx.Event(pointer.Filter{
			Target:  v,
			Kinds:   pointer.Press | pointer.Drag | pointer.Release | pointer.Cancel | pointer.Move | pointer.Leave | pointer.Scroll,
			ScrollX: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
			ScrollY: pointer.ScrollRange{Min: -1 << 20, Max: 1 << 20},
		})
		if !ok {
			return
		}
		if e, ok := ev.(pointer.Event); ok {
			v.handlePointer(gtx, doc, e)
		}
	}
}

func (v *canvasView) handlePointer(gtx layout.Context, doc *editorDoc, e pointer.Event) {
	switch e.Kind {
	case pointer.Press:
		v.Focus(gtx)
		v.beginGesture(doc, e)
	case pointer.Drag:
		v.continueGesture(doc, e)
	case pointer.Release, pointer.Cancel:
		v.gesture = canvasGesture{}
		v.guides = noGuides()
	case pointer.Move:
		v.updateHover(doc, e.Position)
	case pointer.Leave:
		v.hover, v.isOver = compose.HandleNone, false
	case pointer.Scroll:
		v.scroll(e)
	}
}

func (v *canvasView) beginGesture(doc *editorDoc, e pointer.Event) {
	at := v.toCanvas(e.Position)
	v.gesture = canvasGesture{startPointer: at, startScreen: e.Position, startPan: v.pan}
	if e.Buttons.Contain(pointer.ButtonTertiary) {
		v.gesture.kind = gesturePan
		return
	}
	if layer, ok := doc.selected(); ok {
		if handle := layer.HandleAt(at, v.hitRadius, v.rotateOffset); handle != compose.HandleNone {
			v.gesture.kind, v.gesture.handle, v.gesture.start = gestureResize, handle, layer
			if handle == compose.HandleRotate {
				v.gesture.kind = gestureRotate
			}
			return
		}
	}
	index := doc.comp.LayerAt(at)
	if index < 0 {
		doc.selectedID = 0
		if v.zoom > 0 {
			v.gesture.kind = gesturePan
		}
		return
	}
	doc.selectedID = doc.comp.Layers[index].ID
	v.gesture.kind, v.gesture.start = gestureMove, doc.comp.Layers[index]
}

func (v *canvasView) continueGesture(doc *editorDoc, e pointer.Event) {
	g := &v.gesture
	if g.kind == gesturePan {
		v.startZoomIfFitted()
		v.pan = g.startPan.Add(e.Position.Sub(g.startScreen))
		return
	}
	if g.kind == gestureNone {
		return
	}
	if !g.hasChanged {
		doc.remember() // the whole drag is one undo step
		g.hasChanged = true
	}
	at := v.toCanvas(e.Position)
	isShift, isAlt := e.Modifiers.Contain(key.ModShift), e.Modifiers.Contain(key.ModAlt)
	var layer compose.Layer
	switch g.kind {
	case gestureMove:
		layer = compose.Moved(g.start, at.Sub(g.startPointer))
		v.guides = noGuides()
		if !isAlt {
			canvas := compose.Vec{X: float64(doc.comp.Size.X), Y: float64(doc.comp.Size.Y)}
			layer, v.guides = compose.Snapped(layer, canvas, v.snapRadius)
		}
	case gestureResize:
		layer = compose.Resized(g.start, g.handle, at, !isShift, minLayerSide)
	case gestureRotate:
		layer = compose.Rotated(g.start, g.startPointer, at, isShift)
	}
	doc.setSelected(layer)
}

func (v *canvasView) updateHover(doc *editorDoc, p f32.Point) {
	at := v.toCanvas(p)
	v.hover = compose.HandleNone
	if layer, ok := doc.selected(); ok {
		v.hover = layer.HandleAt(at, v.hitRadius, v.rotateOffset)
	}
	v.isOver = doc.comp.LayerAt(at) >= 0
}

// scroll zooms around the pointer with Ctrl held, otherwise pans a zoomed canvas.
func (v *canvasView) scroll(e pointer.Event) {
	if !e.Modifiers.Contain(key.ModShortcut) {
		if v.zoom > 0 {
			v.pan = v.pan.Sub(e.Scroll)
		}
		return
	}
	v.startZoomIfFitted()
	factor := float32(math.Pow(zoomPerNotch, -float64(e.Scroll.Y)/wheelNotch))
	v.zoomAround(e.Position, v.zoom*factor)
}

// startZoomIfFitted turns the fitted view into an explicit zoom at the same scale.
func (v *canvasView) startZoomIfFitted() {
	if v.zoom == 0 {
		v.zoom = v.scale
	}
}

// zoomAround changes the zoom keeping the canvas point under at in place.
func (v *canvasView) zoomAround(at f32.Point, zoom float32) {
	zoom = max(minZoom, min(zoom, maxZoom))
	under := v.toCanvas(at)
	origin := f32.Pt(at.X-float32(under.X)*zoom, at.Y-float32(under.Y)*zoom)
	areaCenter := f32.Pt(float32(v.area.X)/2, float32(v.area.Y)/2)
	halfCanvas := f32.Pt(float32(v.canvasSize.X)*zoom/2, float32(v.canvasSize.Y)*zoom/2)
	v.pan = origin.Sub(areaCenter).Add(halfCanvas)
	v.zoom = zoom
}

// HandleKeys applies the canvas' keyboard shortcuts while it has focus.
func (v *canvasView) HandleKeys(gtx layout.Context, doc *editorDoc) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Focus: v, Name: key.NameDeleteBackward},
			key.Filter{Focus: v, Name: key.NameDeleteForward},
			key.Filter{Focus: v, Name: key.NameEscape},
			key.Filter{Focus: v, Name: key.NameLeftArrow, Optional: key.ModShift},
			key.Filter{Focus: v, Name: key.NameRightArrow, Optional: key.ModShift},
			key.Filter{Focus: v, Name: key.NameUpArrow, Optional: key.ModShift | key.ModShortcut},
			key.Filter{Focus: v, Name: key.NameDownArrow, Optional: key.ModShift | key.ModShortcut},
			key.Filter{Focus: v, Name: "]", Optional: key.ModShortcut},
			key.Filter{Focus: v, Name: "[", Optional: key.ModShortcut},
		)
		if !ok {
			return
		}
		if e, ok := ev.(key.Event); ok && e.State == key.Press {
			applyCanvasKey(doc, e)
		}
	}
}

func applyCanvasKey(doc *editorDoc, e key.Event) {
	index := doc.selectedIndex()
	if index < 0 {
		return
	}
	step := float64(nudgeStep)
	if e.Modifiers.Contain(key.ModShift) {
		step = nudgeStepShifted
	}
	isShortcut := e.Modifiers.Contain(key.ModShortcut)
	switch {
	case e.Name == key.NameEscape:
		doc.selectedID = 0
	case e.Name == key.NameDeleteBackward || e.Name == key.NameDeleteForward:
		deleteSelected(doc)
	case isShortcut && (e.Name == key.NameUpArrow || e.Name == "]"):
		doc.change(func(c *compose.Composition) { c.MoveLayer(index, len(c.Layers)-1) })
	case isShortcut && (e.Name == key.NameDownArrow || e.Name == "["):
		doc.change(func(c *compose.Composition) { c.MoveLayer(index, 0) })
	case e.Name == "]":
		doc.change(func(c *compose.Composition) { c.MoveLayer(index, index+1) })
	case e.Name == "[":
		doc.change(func(c *compose.Composition) { c.MoveLayer(index, index-1) })
	default:
		nudge(doc, e.Name, step)
	}
}

func nudge(doc *editorDoc, name key.Name, step float64) {
	delta := map[key.Name]compose.Vec{
		key.NameLeftArrow: {X: -step}, key.NameRightArrow: {X: step},
		key.NameUpArrow: {Y: -step}, key.NameDownArrow: {Y: step},
	}[name]
	doc.changeSelected(func(l *compose.Layer) { l.Center = l.Center.Add(delta) })
}

func deleteSelected(doc *editorDoc) {
	index := doc.selectedIndex()
	if index < 0 {
		return
	}
	doc.change(func(c *compose.Composition) { c.Layers = append(c.Layers[:index:index], c.Layers[index+1:]...) })
	doc.selectedID = 0
}

// Layout draws the canvas and its layers, the selected layer's frame and handles, and the
// snap guides, filling the area it is given.
func (v *canvasView) Layout(gtx layout.Context, theme *material.Theme, doc *editorDoc) layout.Dimensions {
	area := gtx.Constraints.Max
	v.updateMapping(gtx, area, doc.comp.Size)
	v.area, v.canvasSize = area, doc.comp.Size
	v.hitRadius = float64(gtx.Dp(handleHitSize)) / 2 / float64(v.scale)
	v.rotateOffset = float64(gtx.Dp(rotateHandleOffset)) / float64(v.scale)
	v.snapRadius = float64(gtx.Dp(snapDistance)) / float64(v.scale)
	selected, _ := doc.selected()

	defer clip.Rect{Max: area}.Push(gtx.Ops).Pop()
	v.drawCanvas(gtx, doc)
	v.drawSelection(gtx, doc)
	v.drawGuides(gtx, doc.comp.Size)
	event.Op(gtx.Ops, v)
	v.cursor(selected.Rotation).Add(gtx.Ops)
	v.layoutStatus(gtx, theme, doc.comp.Size)
	return layout.Dimensions{Size: area}
}

// updateMapping works out where the canvas goes this frame: fitted inside the area with a
// margin, or at the zoom and pan the user chose.
func (v *canvasView) updateMapping(gtx layout.Context, area, canvas image.Point) {
	if canvas.X <= 0 || canvas.Y <= 0 {
		return
	}
	margin := float32(gtx.Dp(canvasMargin))
	scale := v.zoom
	if scale == 0 {
		fit := min((float32(area.X)-2*margin)/float32(canvas.X), (float32(area.Y)-2*margin)/float32(canvas.Y))
		scale = max(minZoom, fit)
	}
	center := f32.Pt(float32(area.X)/2, float32(area.Y)/2).Add(v.pan)
	v.scale = scale
	v.origin = center.Sub(f32.Pt(float32(canvas.X)*scale/2, float32(canvas.Y)*scale/2))
}

func (v *canvasView) canvasRect(size image.Point) image.Rectangle {
	lo := v.toScreen(compose.Vec{})
	hi := v.toScreen(compose.Vec{X: float64(size.X), Y: float64(size.Y)})
	return image.Rect(int(math.Round(float64(lo.X))), int(math.Round(float64(lo.Y))), int(math.Round(float64(hi.X))), int(math.Round(float64(hi.Y))))
}

func (v *canvasView) drawCanvas(gtx layout.Context, doc *editorDoc) {
	rect := v.canvasRect(doc.comp.Size)
	shadow := rect.Add(image.Pt(0, gtx.Dp(spacingTiny)))
	paint.FillShape(gtx.Ops, colorShadow, clip.Rect(shadow).Op())
	if layer, ok := doc.selected(); ok && !layer.IsHidden {
		v.drawLayer(gtx, doc, layer, offCanvasOpacity) // its parts outside the canvas, faintly
	}
	canvasClip := clip.Rect(rect).Push(gtx.Ops)
	v.drawBackground(gtx, rect, doc.comp.Background)
	for _, layer := range doc.comp.Layers {
		if !layer.IsHidden {
			v.drawLayer(gtx, doc, layer, 1)
		}
	}
	canvasClip.Pop()
	border := clip.Stroke{Path: clip.Rect(rect.Inset(-gtx.Dp(borderWidth))).Path(), Width: float32(gtx.Dp(borderWidth))}
	paint.FillShape(gtx.Ops, colorBorderStrong, border.Op())
}

func (v *canvasView) drawBackground(gtx layout.Context, rect image.Rectangle, background compose.Background) {
	switch background {
	case compose.BackgroundBlack:
		paint.FillShape(gtx.Ops, color.NRGBA{A: 0xFF}, clip.Rect(rect).Op())
	case compose.BackgroundWhite:
		paint.FillShape(gtx.Ops, color.NRGBA{R: 0xFF, G: 0xFF, B: 0xFF, A: 0xFF}, clip.Rect(rect).Op())
	default:
		defer op.Offset(rect.Min).Push(gtx.Ops).Pop()
		drawChecker(gtx, rect.Size())
	}
}

// drawLayer draws one layer's preview image where its transform puts it, faded by its
// opacity times fade. Layers still decoding show as a plain placeholder.
func (v *canvasView) drawLayer(gtx layout.Context, doc *editorDoc, layer compose.Layer, fade float64) {
	preview, isLoaded := doc.preview(layer.ID)
	size := layer.Natural
	if isLoaded {
		size = preview.size
	}
	if size.X <= 0 || size.Y <= 0 {
		return
	}
	m := layer.Transform()
	kx, ky := float64(layer.Natural.X)/float64(size.X), float64(layer.Natural.Y)/float64(size.Y)
	s := float64(v.scale)
	affine := f32.NewAffine2D(
		float32(s*m.A*kx), float32(s*m.B*ky), float32(s*m.C)+v.origin.X,
		float32(s*m.D*kx), float32(s*m.E*ky), float32(s*m.F)+v.origin.Y,
	)
	defer op.Affine(affine).Push(gtx.Ops).Pop()
	if opacity := float32(layer.Opacity * fade); opacity < 1 {
		defer paint.PushOpacity(gtx.Ops, opacity).Pop()
	}
	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	if !isLoaded {
		paint.Fill(gtx.Ops, colorElevatedSurface)
		return
	}
	preview.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

// drawSelection outlines the selected layer and draws its eight resize handles and the
// rotation handle on its stem.
func (v *canvasView) drawSelection(gtx layout.Context, doc *editorDoc) {
	layer, ok := doc.selected()
	if !ok {
		return
	}
	corners := layer.Corners()
	var outline clip.Path
	outline.Begin(gtx.Ops)
	outline.MoveTo(v.toScreen(corners[0]))
	for _, corner := range corners[1:] {
		outline.LineTo(v.toScreen(corner))
	}
	outline.Close()
	stroke := float32(gtx.Dp(borderWidth))
	paint.FillShape(gtx.Ops, colorPrimary, clip.Stroke{Path: outline.End(), Width: stroke}.Op())

	offset := v.rotateOffset
	top := v.toScreen(layer.HandlePosition(compose.HandleNorth, offset))
	knob := v.toScreen(layer.HandlePosition(compose.HandleRotate, offset))
	var stem clip.Path
	stem.Begin(gtx.Ops)
	stem.MoveTo(top)
	stem.LineTo(knob)
	paint.FillShape(gtx.Ops, colorPrimary, clip.Stroke{Path: stem.End(), Width: stroke}.Op())

	half := gtx.Dp(handleSize) / 2
	drawHandle := func(at f32.Point, shape func(image.Rectangle) clip.Op) {
		center := image.Pt(int(at.X), int(at.Y))
		box := image.Rectangle{Min: center.Sub(image.Pt(half, half)), Max: center.Add(image.Pt(half, half))}
		paint.FillShape(gtx.Ops, colorPrimary, shape(box.Inset(-gtx.Dp(borderWidth))))
		paint.FillShape(gtx.Ops, colorTextPrimary, shape(box))
	}
	ellipse := func(r image.Rectangle) clip.Op { return clip.Ellipse(r).Op(gtx.Ops) }
	square := func(r image.Rectangle) clip.Op { return clip.Rect(r).Op() }
	drawHandle(knob, ellipse)
	for _, handle := range compose.ResizeHandles() {
		drawHandle(v.toScreen(layer.HandlePosition(handle, offset)), square)
	}
}

// drawGuides draws the canvas lines the dragged layer snapped to.
func (v *canvasView) drawGuides(gtx layout.Context, size image.Point) {
	rect := v.canvasRect(size)
	width := gtx.Dp(borderWidth)
	if !math.IsNaN(v.guides.X) {
		x := int(v.toScreen(compose.Vec{X: v.guides.X}).X)
		paint.FillShape(gtx.Ops, colorPrimary, clip.Rect(image.Rect(x, rect.Min.Y, x+width, rect.Max.Y)).Op())
	}
	if !math.IsNaN(v.guides.Y) {
		y := int(v.toScreen(compose.Vec{Y: v.guides.Y}).Y)
		paint.FillShape(gtx.Ops, colorPrimary, clip.Rect(image.Rect(rect.Min.X, y, rect.Max.X, y+width)).Op())
	}
}

// cursor shows what a press would do: grab a layer, resize along a handle's (rotated)
// direction, rotate, or pan.
func (v *canvasView) cursor(selectedRotation float64) pointer.Cursor {
	switch v.gesture.kind {
	case gestureMove, gesturePan:
		return pointer.CursorGrabbing
	case gestureResize:
		return resizeCursor(v.gesture.handle, v.gesture.start.Rotation)
	case gestureRotate:
		return pointer.CursorCrosshair
	}
	switch {
	case v.hover == compose.HandleRotate:
		return pointer.CursorCrosshair
	case v.hover != compose.HandleNone:
		return resizeCursor(v.hover, selectedRotation)
	case v.isOver:
		return pointer.CursorGrab
	}
	return pointer.CursorDefault
}

// resizeCursor picks the two-way arrow closest to the handle's direction once rotated.
func resizeCursor(handle compose.Handle, rotation float64) pointer.Cursor {
	dx, dy := handle.Direction()
	angle := math.Atan2(float64(dy), float64(dx)) + rotation
	step := int(math.Round(angle/(math.Pi/4))) % 4
	if step < 0 {
		step += 4
	}
	return [4]pointer.Cursor{
		pointer.CursorEastWestResize, pointer.CursorNorthWestSouthEastResize,
		pointer.CursorNorthSouthResize, pointer.CursorNorthEastSouthWestResize,
	}[step]
}

// layoutStatus draws "1920 × 1080 · 42%" in the area's bottom-left corner.
func (v *canvasView) layoutStatus(gtx layout.Context, theme *material.Theme, size image.Point) {
	label := monoText(theme, textSizeCaption, fmt.Sprintf("%d × %d · %d%%", size.X, size.Y, v.ZoomPercent()))
	label.Color = colorTextTertiary
	recording := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	dims := label.Layout(gtx)
	call := recording.Stop()
	margin := gtx.Dp(spacingSmall)
	defer op.Offset(image.Pt(margin, gtx.Constraints.Max.Y-dims.Size.Y-margin)).Push(gtx.Ops).Pop()
	call.Add(gtx.Ops)
}

// IsDragging reports whether a move, resize, rotate or pan is in progress.
func (v *canvasView) IsDragging() bool {
	return v.gesture.kind != gestureNone
}
