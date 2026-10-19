package ui

import (
	"errors"
	"fmt"
	"image"
	"log/slog"
	"path/filepath"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/compose"
	"world-kit/internal/filedialog"
	"world-kit/internal/settings"
)

// imageFileTypes are what Add images offers in the Open dialog.
var imageFileTypes = []filedialog.FileType{
	{Name: "Images", Pattern: "*.png;*.jpg;*.jpeg;*.bmp;*.webp"},
	{Name: "All files", Pattern: "*.*"},
}

// pickedImage is a file chosen in the Open dialog, with its size read (not decoded yet).
type pickedImage struct {
	path string
	size image.Point
	err  error
}

// pickedImages is the Open dialog's outcome; isFromImage carries the request along.
type pickedImages struct {
	images      []pickedImage
	isFromImage bool
}

// EditorPage is the Screenshot Editor: pick a canvas size, add game screenshots and chat
// images as layers, arrange them, and export.
type EditorPage struct {
	doc             *editorDoc
	templates       *templatePicker
	canvas          *canvasView
	inspector       *layerInspector
	layers          *layerList
	canvasMenu      dropdownMenu
	addImages       widget.Clickable
	addChat         widget.Clickable
	undo            widget.Clickable
	redo            widget.Clickable
	fit             widget.Clickable
	closeCanvas     widget.Clickable
	export          widget.Clickable
	picker          *backgroundLoad[pickedImages]
	chatPicker      *chatPicker
	exportDialog    *exportDialog
	hasOpenedExport bool
	presetName      string // the preset the canvas size came from; the export starts with it
	exportedPath    string // the last export, for the toast's Show in folder
	closePrompt     confirmPrompt
	toast           toast
	hasWarnedMemory bool
}

// NewEditorPage creates the page. invalidate redraws the window when background work
// (decoding, dialogs, exports) finishes.
func NewEditorPage(invalidate func()) *EditorPage {
	return &EditorPage{
		doc:          newEditorDoc(invalidate),
		templates:    newTemplatePicker(),
		canvas:       newCanvasView(),
		inspector:    newLayerInspector(),
		layers:       newLayerList(),
		picker:       newBackgroundLoad[pickedImages](invalidate),
		chatPicker:   newChatPicker(invalidate),
		exportDialog: newExportDialog(invalidate),
	}
}

// HasWork reports whether the canvas has layers, for the sidebar's dot.
func (p *EditorPage) HasWork() bool {
	return p.doc.hasCanvas && len(p.doc.comp.Layers) > 0
}

// Layout handles input, then draws the template picker or the workspace, and any dialog.
func (p *EditorPage) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	p.doc.receiveImages()
	p.warnAboutMemory(gtx)
	p.receivePicked(gtx)
	p.receiveChatImages(gtx)
	p.receiveExport(gtx, services)
	var dims layout.Dimensions
	if p.doc.hasCanvas {
		p.handleWorkspaceInput(gtx, services)
		dims = p.layoutWorkspace(gtx, theme, services)
	} else {
		dims = p.layoutTemplates(gtx, theme, services)
	}
	p.chatPicker.Layout(gtx, theme)
	p.exportDialog.Layout(gtx, theme, services, p.doc.comp)
	p.layoutClosePrompt(gtx, theme)
	return dims
}

func (p *EditorPage) layoutTemplates(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	presets := services.Settings.Get().ExportPresets
	if choice, ok := p.templates.Update(gtx, presets); ok {
		if choice.isFromImage {
			p.startPicking(services, true)
		} else {
			p.startCanvas(choice.size, choice.preset)
		}
	}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions { return p.templates.Layout(gtx, theme, presets) }),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions { return p.layoutToast(gtx, theme) }),
	)
}

func (p *EditorPage) startCanvas(size image.Point, preset string) {
	p.doc.startCanvas(size)
	p.presetName = preset
	p.canvas.Fit()
}

// startPicking opens the Open dialog off the UI goroutine and reads each file's size.
func (p *EditorPage) startPicking(services Services, isFromImage bool) {
	if p.picker.IsLoading() {
		return
	}
	owner := services.window
	p.picker.Start(func() (pickedImages, error) {
		paths, err := services.Files.OpenFiles(owner, "Add game screenshots", "", imageFileTypes)
		if errors.Is(err, filedialog.ErrCanceled) {
			return pickedImages{isFromImage: isFromImage}, nil
		}
		if err != nil {
			return pickedImages{}, err
		}
		picked := pickedImages{isFromImage: isFromImage, images: make([]pickedImage, len(paths))}
		for i, path := range paths {
			size, err := compose.ReadSize(path)
			picked.images[i] = pickedImage{path: path, size: size, err: err}
		}
		return picked, nil
	})
}

func (p *EditorPage) receivePicked(gtx layout.Context) {
	result, ok := p.picker.Result()
	if !ok {
		return
	}
	if result.err != nil {
		slog.Error("open images", "error", result.err)
		p.toast.ShowError(gtx.Now, "Couldn't open the file picker.")
		return
	}
	var readable []pickedImage
	for _, picked := range result.value.images {
		if picked.err != nil {
			slog.Warn("add image", "path", picked.path, "error", picked.err)
			continue
		}
		readable = append(readable, picked)
	}
	if skipped := len(result.value.images) - len(readable); skipped > 0 {
		p.toast.ShowError(gtx.Now, skippedImagesText(skipped))
	}
	if len(readable) == 0 {
		return
	}
	if result.value.isFromImage {
		p.startCanvas(readable[0].size, "")
	}
	p.addPhotos(gtx, readable)
}

func skippedImagesText(count int) string {
	return fmt.Sprintf("%s couldn't be added: not an image, or over %d px.", plural(count, "file", "files"), compose.MaxSourceSide)
}

func (p *EditorPage) addPhotos(gtx layout.Context, images []pickedImage) {
	p.doc.remember()
	for _, picked := range images {
		center, size := compose.PlacePhoto(p.doc.comp, picked.size)
		p.doc.addLayer(compose.Layer{
			Source: picked.path, Name: filepath.Base(picked.path), Kind: compose.KindPhoto,
			Natural: picked.size, Center: center, Size: size, Opacity: 1,
		})
	}
	p.canvas.Focus(gtx)
}

func (p *EditorPage) receiveChatImages(gtx layout.Context) {
	shots, ok := p.chatPicker.Update(gtx)
	if !ok {
		return
	}
	p.doc.remember()
	for _, shot := range shots {
		center, size := compose.PlaceChat(p.doc.comp, shot.size)
		p.doc.addLayer(compose.Layer{
			Source: shot.shot.Path, Name: "Chat · " + dayAndTimeText(shot.shot.Saved, gtx.Now), Kind: compose.KindChat,
			Natural: shot.size, Center: center, Size: size, Opacity: 1,
		})
	}
	p.canvas.Focus(gtx)
}

func (p *EditorPage) receiveExport(gtx layout.Context, services Services) {
	result, ok := p.exportDialog.Update(gtx, services, p.doc.comp)
	if !ok {
		return
	}
	p.toast.Show(gtx.Now, "Exported "+filepath.Base(result.path), "Show in folder")
	p.exportedPath = result.path
}

// warnAboutMemory suggests fewer or smaller images once the editor holds a lot of them.
func (p *EditorPage) warnAboutMemory(gtx layout.Context) {
	if p.hasWarnedMemory || p.doc.imageBytes() < memoryWarningBytes {
		return
	}
	p.hasWarnedMemory = true
	p.toast.ShowError(gtx.Now, "That's a lot of large images. If WorldKit slows down, remove some layers.")
}

func (p *EditorPage) handleWorkspaceInput(gtx layout.Context, services Services) {
	p.handleShortcuts(gtx, services)
	p.handleCanvasMenu(gtx, services)
	if p.toast.ActionClicked(gtx) && p.exportedPath != "" {
		revealFile(p.exportedPath)
	}
	if p.addImages.Clicked(gtx) {
		p.startPicking(services, false)
	}
	if p.addChat.Clicked(gtx) {
		p.chatPicker.Open(services.Screenshots)
	}
	if p.undo.Clicked(gtx) {
		p.undoStep()
	}
	if p.redo.Clicked(gtx) {
		p.redoStep()
	}
	if p.closeCanvas.Clicked(gtx) {
		p.askToClose()
	}
	if p.fit.Clicked(gtx) {
		p.canvas.Fit()
	}
	if p.export.Clicked(gtx) {
		p.openExport(services)
	}
	if p.closePrompt.Update(gtx) {
		p.doc.close()
	}
	// The inspector goes first: a typed value commits before a canvas click changes the
	// selection.
	p.inspector.Update(gtx, p.doc)
	p.layers.Update(gtx, p.doc)
	if !p.isDialogOpen() { // keys and clicks behind a dialog must not change the canvas
		p.canvas.Update(gtx, p.doc)
		p.canvas.HandleKeys(gtx, p.doc)
	}
}

func (p *EditorPage) openExport(services Services) {
	if len(p.doc.comp.Layers) == 0 {
		return // nothing to export yet; the button is disabled too
	}
	p.exportDialog.Open(services.Settings, p.presetName, !p.hasOpenedExport)
	p.hasOpenedExport = true
	p.presetName = ""
}

// handleShortcuts applies the editor's keyboard shortcuts wherever the focus is (fields
// keep their own Ctrl+Z while being edited).
func (p *EditorPage) handleShortcuts(gtx layout.Context, services Services) {
	for {
		ev, ok := gtx.Event(
			key.Filter{Name: "Z", Required: key.ModShortcut, Optional: key.ModShift},
			key.Filter{Name: "Y", Required: key.ModShortcut},
			key.Filter{Name: "D", Required: key.ModShortcut},
			key.Filter{Name: "E", Required: key.ModShortcut},
			key.Filter{Name: "0", Required: key.ModShortcut},
		)
		if !ok {
			return
		}
		e, ok := ev.(key.Event)
		if !ok || e.State != key.Press || p.isDialogOpen() {
			continue
		}
		switch {
		case e.Name == "Z" && e.Modifiers.Contain(key.ModShift), e.Name == "Y":
			p.redoStep()
		case e.Name == "Z":
			p.undoStep()
		case e.Name == "D":
			duplicateSelected(p.doc)
		case e.Name == "E":
			p.openExport(services)
		case e.Name == "0":
			p.canvas.Fit()
		}
	}
}

func (p *EditorPage) isDialogOpen() bool {
	return p.chatPicker.isOpen || p.exportDialog.isOpen || p.closePrompt.isOpen
}

// canvasMenuItems lists the canvas sizes, the saved presets and New canvas.
func canvasMenuItems(presets []settings.ExportPreset) []menuItem {
	items := []menuItem{{label: "Canvas size", isHeading: true}}
	for _, template := range canvasTemplates {
		items = append(items, menuItem{label: template.name, detail: sizeText(template.size)})
	}
	if len(presets) > 0 {
		items = append(items, menuItem{label: "Your presets", isHeading: true})
		for _, preset := range presets {
			items = append(items, menuItem{label: preset.Name, detail: sizeText(image.Pt(preset.Width, preset.Height))})
		}
	}
	return append(items, menuItem{isHeading: true}, menuItem{label: "New canvas…"})
}

func sizeText(size image.Point) string {
	return fmt.Sprintf("%d × %d", size.X, size.Y)
}

// handleCanvasMenu resizes the canvas (keeping the layers centered) or starts over.
func (p *EditorPage) handleCanvasMenu(gtx layout.Context, services Services) {
	presets := services.Settings.Get().ExportPresets
	items := canvasMenuItems(presets)
	index, ok := p.canvasMenu.Update(gtx, len(items))
	if !ok {
		return
	}
	templateIndex := index - 1
	presetIndex := templateIndex - len(canvasTemplates) - 1
	switch {
	case index == len(items)-1:
		p.askToClose()
	case templateIndex >= 0 && templateIndex < len(canvasTemplates):
		p.resizeCanvas(canvasTemplates[templateIndex].size)
	case presetIndex >= 0 && presetIndex < len(presets):
		preset := presets[presetIndex]
		p.resizeCanvas(image.Pt(preset.Width, preset.Height))
		p.presetName = preset.Name
	}
}

func (p *EditorPage) resizeCanvas(size image.Point) {
	p.doc.change(func(c *compose.Composition) { c.Resize(compose.ClampCanvasSize(size)) })
	p.canvas.Fit()
}

// askToClose goes back to the template picker, asking first when layers would be lost.
func (p *EditorPage) askToClose() {
	if len(p.doc.comp.Layers) == 0 {
		p.doc.close()
		return
	}
	p.closePrompt.isOpen = true
}

func (p *EditorPage) layoutWorkspace(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	inset := layout.Inset{Left: editorPagePadding, Right: editorPagePadding, Top: spacingSmall, Bottom: editorPagePadding}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutToolbar(gtx, theme, services) }),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layoutCanvasArea(gtx, theme) }),
					layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutSideColumn(gtx, theme) }),
				)
			}),
		)
	})
}

func (p *EditorPage) layoutToolbar(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	compact := func(click *widget.Clickable, label string, icon strokeIcon) layout.Widget {
		return secondaryButton(click, label).withIcon(icon).withHeight(controlHeightCompact).Widget(theme)
	}
	square := func(click *widget.Clickable, icon strokeIcon, isEnabled bool) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return layoutEnabled(gtx, isEnabled, iconButton(click, icon).withHeight(controlHeightCompact).Widget(theme))
		}
	}
	items := canvasMenuItems(services.Settings.Get().ExportPresets)
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layoutWrappedCentered(gtx, spacingSmall, []layout.Widget{
				compact(&p.closeCanvas, "Close", iconClose),
				verticalDivider,
				func(gtx layout.Context) layout.Dimensions {
					return p.canvasMenu.LayoutButton(gtx, theme, sizeText(p.doc.comp.Size), true, items)
				},
				verticalDivider,
				compact(&p.addImages, "Add images", iconImage),
				compact(&p.addChat, "Add chat image", iconChat),
				verticalDivider,
				square(&p.undo, iconUndo, p.doc.history.CanUndo()),
				square(&p.redo, iconRedo, p.doc.history.CanRedo()),
				square(&p.fit, iconFit, true),
			})
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutEnabled(gtx, len(p.doc.comp.Layers) > 0,
				primaryButton(&p.export, "Export").withIcon(iconExport).withHeight(controlHeightCompact).Widget(theme))
		}),
	)
}

// layoutEnabled draws w faded while it can't be used. Callers ignore its clicks then.
func layoutEnabled(gtx layout.Context, isEnabled bool, w layout.Widget) layout.Dimensions {
	if isEnabled {
		return w(gtx)
	}
	defer paint.PushOpacity(gtx.Ops, disabledOpacity).Pop()
	gtx = gtx.Disabled()
	return w(gtx)
}

// layoutCanvasArea draws the panel the canvas sits on, the canvas, and toasts.
func (p *EditorPage) layoutCanvasArea(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	size := gtx.Constraints.Max
	gtx.Constraints = layout.Exact(size)
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusCard))
	paint.FillShape(gtx.Ops, colorSidebar, shape.Op(gtx.Ops))
	paint.FillShape(gtx.Ops, colorDivider, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(borderWidth))}.Op())
	defer shape.Push(gtx.Ops).Pop()
	p.canvas.Layout(gtx, theme, p.doc)
	if len(p.doc.comp.Layers) == 0 {
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			label := secondaryText(theme, "Add a game screenshot, then chat images on top.")
			return label.Layout(gtx)
		})
	}
	p.layoutToast(gtx, theme)
	return layout.Dimensions{Size: size}
}

func (p *EditorPage) layoutToast(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.SE.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return p.toast.Layout(gtx, theme)
		})
	})
}

func (p *EditorPage) layoutSideColumn(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Max.X = gtx.Dp(inspectorWidth)
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.inspector.Layout(gtx, theme, p.doc) }),
		layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return p.layers.Layout(gtx, theme, p.doc) }),
	)
}

func (p *EditorPage) layoutClosePrompt(gtx layout.Context, theme *material.Theme) {
	p.closePrompt.Layout(gtx, theme, "Close this canvas?",
		"Its layers are cleared. Export first if you want to keep this picture.", "Close canvas")
}

// confirmPrompt asks before something that can't be undone, with a danger button.
type confirmPrompt struct {
	isOpen  bool
	dialog  dialog
	confirm widget.Clickable
	cancel  widget.Clickable
}

// Update reports whether the user confirmed, closing the prompt either way.
func (c *confirmPrompt) Update(gtx layout.Context) bool {
	if c.cancel.Clicked(gtx) {
		c.isOpen = false
	}
	if c.confirm.Clicked(gtx) && c.isOpen {
		c.isOpen = false
		return true
	}
	return false
}

func (c *confirmPrompt) Layout(gtx layout.Context, theme *material.Theme, title, message, confirmLabel string) {
	if !c.isOpen {
		return
	}
	c.dialog.Layout(gtx, theme, title, message, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(dangerButton(&c.confirm, confirmLabel).Widget(theme)),
			layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
			layout.Rigid(secondaryButton(&c.cancel, "Cancel").Widget(theme)),
		)
	})
}

// undoStep undoes the last change, except mid-drag: the drag would overwrite the restored
// state on its next move.
func (p *EditorPage) undoStep() {
	if !p.canvas.IsDragging() {
		p.doc.undo()
	}
}

// redoStep redoes the last undone change, except mid-drag (see undoStep).
func (p *EditorPage) redoStep() {
	if !p.canvas.IsDragging() {
		p.doc.redo()
	}
}
