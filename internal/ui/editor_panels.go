package ui

import (
	"fmt"
	"image"
	"math"

	"gioui.org/f32"
	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/compose"
)

// Canvas backgrounds in segmented-control order.
var canvasBackgrounds = []compose.Background{compose.BackgroundTransparent, compose.BackgroundBlack, compose.BackgroundWhite}

// canvasBackgroundLabels name canvasBackgrounds for the segmented control.
var canvasBackgroundLabels = []string{"None", "Black", "White"}

// layerInspector edits the selected layer (position, size, rotation, opacity) or, with
// nothing selected, the canvas (size and background).
type layerInspector struct {
	x, y, width, height, rotation, opacityPercent *numberField
	canvasWidth, canvasHeight                     *numberField
	opacity                                       slider
	isOpacityDragging                             bool
	isAspectFree                                  bool // W and H change independently
	aspectLock, rotate90, fit, actualSize         widget.Clickable
	duplicate, remove                             widget.Clickable
	background                                    *segmentedControl
}

func newLayerInspector() *layerInspector {
	return &layerInspector{
		x: newCompactNumberField(), y: newCompactNumberField(), width: newCompactNumberField(), height: newCompactNumberField(),
		rotation: newCompactNumberField(), opacityPercent: newCompactNumberField(),
		canvasWidth: newCompactNumberField(), canvasHeight: newCompactNumberField(),
		background: &segmentedControl{labels: canvasBackgroundLabels, clicks: make([]widget.Clickable, len(canvasBackgroundLabels)), isCompact: true},
	}
}

// canvasTarget is the number fields' target for canvas edits; layer IDs start at 1.
const canvasTarget = 0

// Update applies the inspector's edits to doc. Fields of the half that isn't shown drop
// their edits, so a number typed for the canvas never lands later, or on a layer.
func (i *layerInspector) Update(gtx layout.Context, doc *editorDoc) {
	layer, ok := doc.selected()
	if !ok {
		for _, field := range []*numberField{i.x, i.y, i.width, i.height, i.rotation, i.opacityPercent} {
			field.Abandon()
		}
		i.updateCanvas(gtx, doc)
		return
	}
	i.canvasWidth.Abandon()
	i.canvasHeight.Abandon()
	i.updateButtons(gtx, doc, layer)
	i.updatePlacement(gtx, doc, layer)
	i.updateOpacity(gtx, doc, layer)
}

func (i *layerInspector) updateCanvas(gtx layout.Context, doc *editorDoc) {
	size := doc.comp.Size
	if w, ok := i.canvasWidth.Update(gtx, float64(size.X), canvasTarget); ok {
		doc.change(func(c *compose.Composition) { c.Resize(compose.ClampCanvasSize(image.Pt(int(w), size.Y))) })
	}
	if h, ok := i.canvasHeight.Update(gtx, float64(size.Y), canvasTarget); ok {
		doc.change(func(c *compose.Composition) { c.Resize(compose.ClampCanvasSize(image.Pt(size.X, int(h)))) })
	}
	i.background.selected = int(doc.comp.Background)
	if i.background.Update(gtx) {
		doc.change(func(c *compose.Composition) { c.Background = canvasBackgrounds[i.background.selected] })
	}
}

func (i *layerInspector) updateButtons(gtx layout.Context, doc *editorDoc, layer compose.Layer) {
	if i.aspectLock.Clicked(gtx) {
		i.isAspectFree = !i.isAspectFree
	}
	if i.rotate90.Clicked(gtx) {
		doc.changeSelected(func(l *compose.Layer) { l.Rotation = compose.NormalizeAngle(l.Rotation + math.Pi/2) })
	}
	if i.fit.Clicked(gtx) {
		canvas := compose.Vec{X: float64(doc.comp.Size.X), Y: float64(doc.comp.Size.Y)}
		doc.changeSelected(func(l *compose.Layer) {
			l.Center, l.Size, l.Rotation = canvas.Scale(0.5), compose.FitInside(l.Natural, canvas), 0
		})
	}
	if i.actualSize.Clicked(gtx) {
		doc.changeSelected(func(l *compose.Layer) {
			l.Size, l.Rotation = compose.Vec{X: float64(l.Natural.X), Y: float64(l.Natural.Y)}, 0
		})
	}
	if i.duplicate.Clicked(gtx) {
		duplicateSelected(doc)
	}
	if i.remove.Clicked(gtx) {
		deleteSelected(doc)
	}
}

// duplicateSelected copies the selected layer just above it, nudged so it shows.
func duplicateSelected(doc *editorDoc) {
	layer, ok := doc.selected()
	if !ok {
		return
	}
	doc.remember()
	index := doc.selectedIndex()
	images := doc.images[layer.ID]
	layer.Center = layer.Center.Add(compose.Vec{X: placementNudge, Y: placementNudge})
	doc.addLayer(layer)
	doc.comp.MoveLayer(len(doc.comp.Layers)-1, index+1)
	if images.err == nil && len(images.versions) > 0 {
		doc.images[doc.selectedID] = images // same pixels; the reload finishes later anyway
	}
}

// placementNudge is how far (canvas pixels) a duplicate is moved from its original.
const placementNudge = 16

// updatePlacement applies typed position, size and rotation.
func (i *layerInspector) updatePlacement(gtx layout.Context, doc *editorDoc, layer compose.Layer) {
	lo, _ := layer.Bounds()
	if x, ok := i.x.Update(gtx, lo.X, layer.ID); ok {
		doc.changeSelected(func(l *compose.Layer) { l.Center.X += x - lo.X })
	}
	if y, ok := i.y.Update(gtx, lo.Y, layer.ID); ok {
		doc.changeSelected(func(l *compose.Layer) { l.Center.Y += y - lo.Y })
	}
	if w, ok := i.width.Update(gtx, layer.Size.X, layer.ID); ok && w >= minLayerSide {
		doc.changeSelected(func(l *compose.Layer) {
			if !i.isAspectFree {
				l.Size.Y *= w / l.Size.X
			}
			l.Size.X = w
		})
	}
	if h, ok := i.height.Update(gtx, layer.Size.Y, layer.ID); ok && h >= minLayerSide {
		doc.changeSelected(func(l *compose.Layer) {
			if !i.isAspectFree {
				l.Size.X *= h / l.Size.Y
			}
			l.Size.Y = h
		})
	}
	degrees := layer.Rotation * 180 / math.Pi
	if r, ok := i.rotation.Update(gtx, degrees, layer.ID); ok {
		doc.changeSelected(func(l *compose.Layer) { l.Rotation = compose.NormalizeAngle(r * math.Pi / 180) })
	}
}

// updateOpacity applies the slider (one undo step per drag) and the typed percent.
func (i *layerInspector) updateOpacity(gtx layout.Context, doc *editorDoc, layer compose.Layer) {
	if !i.opacity.Dragging() {
		i.opacity.value.Value = float32(layer.Opacity)
	}
	if i.opacity.Update(gtx) {
		if !i.isOpacityDragging {
			doc.remember()
		}
		i.isOpacityDragging = i.opacity.Dragging()
		value := math.Round(float64(i.opacity.value.Value)*100) / 100
		doc.setSelected(withOpacityValue(layer, value))
	}
	if !i.opacity.Dragging() {
		i.isOpacityDragging = false
	}
	if p, ok := i.opacityPercent.Update(gtx, layer.Opacity*100, layer.ID); ok {
		doc.changeSelected(func(l *compose.Layer) { *l = withOpacityValue(*l, p/100) })
	}
}

func withOpacityValue(layer compose.Layer, opacity float64) compose.Layer {
	layer.Opacity = max(0, min(opacity, 1))
	return layer
}

// Layout draws the inspector card for the selection, or for the canvas: dense, like a
// design tool's properties panel, so the canvas keeps the room.
func (i *layerInspector) Layout(gtx layout.Context, theme *material.Theme, doc *editorDoc) layout.Dimensions {
	return card(gtx, panelPadding, func(gtx layout.Context) layout.Dimensions {
		if layer, ok := doc.selected(); ok {
			return i.layoutLayer(gtx, theme, layer)
		}
		return i.layoutCanvas(gtx, theme)
	})
}

func (i *layerInspector) layoutCanvas(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(panelHeading(theme, "Canvas")),
		layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inspectorRow(gtx, theme, "Size", fieldPair(theme, i.canvasWidth, "W", i.canvasHeight, "H", ""))
		}),
		layout.Rigid(layout.Spacer{Height: panelRowGap}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return inspectorRow(gtx, theme, "Fill", func(gtx layout.Context) layout.Dimensions { return i.background.Layout(gtx, theme) })
		}),
	)
}

func (i *layerInspector) layoutLayer(gtx layout.Context, theme *material.Theme, layer compose.Layer) layout.Dimensions {
	lockIcon := iconLock
	if i.isAspectFree {
		lockIcon = iconUnlock
	}
	row := func(label string, controls layout.Widget) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: panelRowGap}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return inspectorRow(gtx, theme, label, controls)
			})
		})
	}
	withIcon := func(controls layout.Widget, click *widget.Clickable, icon strokeIcon) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				layout.Flexed(1, controls),
				layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
				layout.Rigid(panelIconButton(theme, click, icon)),
			)
		}
	}
	opacity := func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
			layout.Flexed(1, i.opacity.Layout),
			layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
			layout.Rigid(fixedField(theme, i.opacityPercent, "", "%")),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return i.layoutTitle(gtx, theme, layer) }),
		layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
		row("Position", fieldPair(theme, i.x, "X", i.y, "Y", "")),
		row("Size", withIcon(fieldPair(theme, i.width, "W", i.height, "H", ""), &i.aspectLock, lockIcon)),
		row("Rotate", withIcon(fieldPair(theme, i.rotation, "", nil, "", "°"), &i.rotate90, iconRotate)),
		row("Opacity", opacity),
		layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			button := func(click *widget.Clickable, label string) layout.FlexChild {
				return layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return secondaryButton(click, label).withHeight(panelControlHeight).Layout(gtx, theme)
				})
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				button(&i.fit, "Fit canvas"),
				layout.Rigid(layout.Spacer{Width: spacingTiny + spacingTiny/2}.Layout),
				button(&i.actualSize, "Real size"),
			)
		}),
	)
}

func (i *layerInspector) layoutTitle(gtx layout.Context, theme *material.Theme, layer compose.Layer) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			title := styledLabel(theme, textSizeSmall, font.SemiBold, colorTextPrimary, layer.Name)
			title.MaxLines = 1
			return title.Layout(gtx)
		}),
		layout.Rigid(panelIconButton(theme, &i.duplicate, iconCopy)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return iconButton(&i.remove, iconTrash).withKind(buttonGhost).withHeight(panelControlHeight).asDanger().Layout(gtx, theme)
		}),
	)
}

// panelHeading is a panel section's name: small, uppercase, like the page's group labels.
func panelHeading(theme *material.Theme, text string) layout.Widget {
	return groupLabel(theme, text).Layout
}

// panelIconButton is a ghost icon button sized for the editor's panels.
func panelIconButton(theme *material.Theme, click *widget.Clickable, icon strokeIcon) layout.Widget {
	return iconButton(click, icon).withKind(buttonGhost).withHeight(panelControlHeight).Widget(theme)
}

// inspectorRow is a label in a fixed column, then its controls.
func inspectorRow(gtx layout.Context, theme *material.Theme, label string, controls layout.Widget) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(panelLabelWidth)
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			label := caption(theme, label)
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
		layout.Flexed(1, controls),
	)
}

// fieldPair lays out two number fields side by side, sharing the width; with no second
// field, the first takes half the width so single fields line up with pairs.
func fieldPair(theme *material.Theme, first *numberField, firstPrefix string, second *numberField, secondPrefix, unit string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return first.Layout(gtx, theme, firstPrefix, unit) }),
			layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				if second == nil {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Max.X, 0)}
				}
				return second.Layout(gtx, theme, secondPrefix, unit)
			}),
		)
	}
}

// fixedField lays out a number field inspectorFieldWidth wide.
func fixedField(theme *material.Theme, field *numberField, prefix, unit string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = gtx.Dp(inspectorFieldWidth)
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return field.Layout(gtx, theme, prefix, unit)
	}
}

// layerRowState is one row's widget state, kept by layer ID.
type layerRowState struct {
	click      widget.Clickable
	visibility widget.Clickable
	raise      widget.Clickable
	lower      widget.Clickable
	hover      hoverArea
}

// layerList lists the layers front-most first; a row selects its layer and, while
// hovered, hides it or moves it up or down.
type layerList struct {
	list widget.List
	rows map[int]*layerRowState
}

func newLayerList() *layerList {
	return &layerList{list: widget.List{List: layout.List{Axis: layout.Vertical}}, rows: map[int]*layerRowState{}}
}

func (l *layerList) row(id int) *layerRowState {
	state := l.rows[id]
	if state == nil {
		state = &layerRowState{}
		l.rows[id] = state
	}
	return state
}

// Update handles the rows' clicks.
func (l *layerList) Update(gtx layout.Context, doc *editorDoc) {
	for index, layer := range doc.comp.Layers {
		state := l.row(layer.ID)
		if state.click.Clicked(gtx) {
			doc.selectedID = layer.ID
		}
		if state.visibility.Clicked(gtx) {
			doc.change(func(c *compose.Composition) { c.Layers[index].IsHidden = !c.Layers[index].IsHidden })
		}
		if state.raise.Clicked(gtx) {
			doc.change(func(c *compose.Composition) { c.MoveLayer(index, index+1) })
		}
		if state.lower.Clicked(gtx) {
			doc.change(func(c *compose.Composition) { c.MoveLayer(index, index-1) })
		}
	}
}

// Layout draws the card: "Layers" and the count, then the rows, scrolling.
func (l *layerList) Layout(gtx layout.Context, theme *material.Theme, doc *editorDoc) layout.Dimensions {
	gtx.Constraints.Min.Y = gtx.Constraints.Max.Y
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return outlinedSurface(gtx, colorSurface, colorDivider, radiusCard)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.UniformInset(spacingSmall).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return l.layoutHeader(gtx, theme, len(doc.comp.Layers)) }),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return l.layoutRows(gtx, theme, doc) }),
				)
			})
		}),
	)
}

func (l *layerList) layoutHeader(gtx layout.Context, theme *material.Theme, count int) layout.Dimensions {
	inset := layout.Inset{Top: spacingTiny, Bottom: spacingSmall, Left: spacingTiny, Right: spacingTiny}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}
		return row.Layout(gtx,
			layout.Flexed(1, panelHeading(theme, "Layers")),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := monoText(theme, textSizeCaption, fmt.Sprint(count))
				label.Color = colorTextTertiary
				return label.Layout(gtx)
			}),
		)
	})
}

func (l *layerList) layoutRows(gtx layout.Context, theme *material.Theme, doc *editorDoc) layout.Dimensions {
	layers := doc.comp.Layers
	if len(layers) == 0 {
		return layout.UniformInset(spacingTiny).Layout(gtx, caption(theme, "No layers yet. Add images from the toolbar.").Layout)
	}
	return styledList(theme, &l.list).Layout(gtx, len(layers), func(gtx layout.Context, index int) layout.Dimensions {
		layer := layers[len(layers)-1-index] // front-most first
		row := layerRow{layer: layer, state: l.row(layer.ID), image: doc.images[layer.ID], isSelected: layer.ID == doc.selectedID}
		return layout.Inset{Bottom: spacingTiny / 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return row.Layout(gtx, theme)
		})
	})
}

// layerRow is one layer in the list: thumbnail, name over kind and size, and on hover
// (or while hidden) the visibility toggle and up/down.
type layerRow struct {
	layer      compose.Layer
	state      *layerRowState
	image      loadedImage
	isSelected bool
}

func (r layerRow) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return r.state.hover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(layerRowHeight))
		gtx.Constraints = layout.Exact(size)
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return r.state.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					r.drawBackground(gtx, size)
					return layout.Dimensions{Size: size}
				})
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints = layout.Exact(size)
				return layout.Inset{Left: spacingSmall, Right: spacingTiny / 2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return r.layoutContent(gtx, theme)
				})
			}),
		)
	})
}

func (r layerRow) drawBackground(gtx layout.Context, size image.Point) {
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusMenu)).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	if r.isSelected || r.state.hover.isHovered {
		paint.Fill(gtx.Ops, colorElevatedSurface)
	}
	if r.isSelected {
		barHeight := gtx.Dp(activeIndicatorHeight)
		top := (size.Y - barHeight) / 2
		bar := image.Rect(0, top, gtx.Dp(activeIndicatorWidth), top+barHeight)
		paint.FillShape(gtx.Ops, colorPrimary, clip.UniformRRect(bar, gtx.Dp(activeIndicatorWidth)/2).Op(gtx.Ops))
	}
}

func (r layerRow) layoutContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	nameColor := colorTextPrimary
	if r.layer.IsHidden {
		nameColor = colorTextTertiary
	}
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.layoutThumb(gtx) }),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := styledLabel(theme, textSizeCaption, font.Medium, nameColor, r.layer.Name)
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := styledLabel(theme, textSizeTiny, font.Normal, colorTextTertiary, layerCaption(r.layer, r.image))
					label.MaxLines = 1
					if r.image.err != nil {
						label.Color = colorDanger
					}
					return label.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.layoutActions(gtx, theme) }),
	)
}

// layerCaption says what a layer is: "Chat image · 465 × 86 · 60%", "Loading…" or why it
// can't be shown.
func layerCaption(layer compose.Layer, loaded loadedImage) string {
	switch {
	case loaded.err != nil:
		return "File missing or unreadable"
	case len(loaded.versions) == 0:
		return "Loading…"
	}
	kind := "Screenshot"
	if layer.Kind == compose.KindChat {
		kind = "Chat image"
	}
	text := fmt.Sprintf("%s · %d × %d", kind, int(math.Round(layer.Size.X)), int(math.Round(layer.Size.Y)))
	if layer.Opacity < 1 {
		text += fmt.Sprintf(" · %d%%", int(math.Round(layer.Opacity*100)))
	}
	return text
}

// layoutThumb draws the layer's thumbnail fitted in a small checkerboard square.
func (r layerRow) layoutThumb(gtx layout.Context) layout.Dimensions {
	side := gtx.Dp(layerThumbSize)
	size := image.Pt(side, side)
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusSmall)).Push(gtx.Ops).Pop()
	drawChecker(gtx, size)
	if len(r.image.versions) > versionThumb {
		drawFittedImage(gtx, r.image.versions[versionThumb], size)
	}
	return layout.Dimensions{Size: size}
}

// drawFittedImage draws version scaled to fit inside box, centered.
func drawFittedImage(gtx layout.Context, version imageVersion, box image.Point) {
	if version.size.X == 0 || version.size.Y == 0 {
		return
	}
	scale := min(float32(box.X)/float32(version.size.X), float32(box.Y)/float32(version.size.Y))
	shown := f32.Pt(float32(version.size.X)*scale, float32(version.size.Y)*scale)
	offset := f32.Pt((float32(box.X)-shown.X)/2, (float32(box.Y)-shown.Y)/2)
	affine := f32.AffineId().Scale(f32.Point{}, f32.Pt(scale, scale)).Offset(offset)
	defer op.Affine(affine).Push(gtx.Ops).Pop()
	defer clip.Rect{Max: version.size}.Push(gtx.Ops).Pop()
	version.op.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
}

func (r layerRow) layoutActions(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	isHovered := r.state.hover.isHovered
	if !isHovered && !r.layer.IsHidden {
		return layout.Dimensions{}
	}
	visibilityIcon := iconEye
	if r.layer.IsHidden {
		visibilityIcon = iconEyeOff
	}
	ghost := func(click *widget.Clickable, icon strokeIcon) layout.FlexChild {
		return layout.Rigid(iconButton(click, icon).withKind(buttonGhost).withHeight(panelControlHeight).Widget(theme))
	}
	children := []layout.FlexChild{ghost(&r.state.visibility, visibilityIcon)}
	if isHovered {
		children = append(children, ghost(&r.state.raise, iconChevronUp), ghost(&r.state.lower, iconChevronDown))
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}
