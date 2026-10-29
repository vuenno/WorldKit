package ui

import (
	"image"
	"image/color"
	"log/slog"

	"gioui.org/f32"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/screenshot"
)

// extractOutcome tells the log viewer what happened in the extract dialog.
type extractOutcome int

const (
	extractPending       extractOutcome = iota // still deciding, or not open
	extractSaveRequested                       // Save image: read Lines and Style
)

// extractDialog opens when Extract image is clicked. It shows a live preview of the image
// and the backdrop choice, and asks whether to save the lines as they are or edit them
// first. Editing happens on the preview itself (pictureEditor): select text on the image
// and pick Italic or a color, or click and type.
type extractDialog struct {
	isOpen     bool
	isEditing  bool
	dialog     dialog
	editor     pictureEditor
	backdrop   *segmentedControl
	chatFont   *segmentedControl // screenshot.ChatFonts, in order
	chatScale  *numberStepper
	widthLimit *numberStepper
	settings   SettingsStore // where extract presets are kept
	presetMenu dropdownMenu
	presetName string // the preset the choices came from; "" once changed
	naming     *presetNaming
	hasOpened  bool
	save       widget.Clickable
	edit       widget.Clickable
	cancel     widget.Clickable
	italic     widget.Clickable
	resetColor widget.Clickable
	mergeLines widget.Clickable
	swatches   []widget.Clickable // one per textSwatches color
	body       widget.List
	renderer   *backgroundLoad[*screenshot.Picture]
	errText    string
}

// newExtractDialog creates the dialog; its presets are read from and saved to settings.
func newExtractDialog(invalidate func(), settings SettingsStore) *extractDialog {
	return &extractDialog{
		settings: settings,
		naming:   newPresetNaming(),
		dialog:   dialog{width: extractDialogWidth},
		backdrop: newSegmentedControl(backdropLabels...),
		chatFont: newSegmentedControl(chatFontLabels()...),
		chatScale: &numberStepper{
			value: screenshot.DefaultChatScale,
			min:   screenshot.MinChatScale, max: screenshot.MaxChatScale, increment: screenshot.ChatScaleStep, unit: "%",
		},
		widthLimit: &numberStepper{
			value: screenshot.DefaultWidthLimit(),
			min:   screenshot.MinWidthLimit, max: screenshot.MaxWidthLimit, increment: widthLimitStep, unit: "px",
		},
		swatches: make([]widget.Clickable, len(textSwatches)),
		body:     widget.List{List: layout.List{Axis: layout.Vertical}},
		renderer: newBackgroundLoad[*screenshot.Picture](invalidate),
	}
}

// Open shows the dialog for lines, unedited, their markup (/italic/, *action*) applied.
// The look (background, chat font and scale, max width) stays as last chosen; the first
// time it comes from the default preset, if one is set in Settings.
func (d *extractDialog) Open(lines []chatLine) {
	d.isOpen, d.isEditing, d.errText = true, false, ""
	d.naming.Reset()
	if !d.hasOpened {
		d.hasOpened = true
		d.applyPresetNamed(d.settings.Get().DefaultExtractPreset)
	}
	shotLines := make([]screenshot.Line, len(lines))
	for i, line := range lines {
		shotLines[i] = screenshot.NewLine(line.copyText, line.color, colorChatEmote)
	}
	d.editor.SetDocument(screenshot.NewDocument(shotLines))
	d.renderPreview()
}

// Lines returns what to save: every line with visible text, as edited.
func (d *extractDialog) Lines() []screenshot.Line {
	return d.editor.doc.NonEmptyLines()
}

// Style returns the chosen look: backdrop, chat font and scale, and max width.
func (d *extractDialog) Style() screenshot.Style {
	return screenshot.Style{Look: d.look(), Backdrop: backdropColor(d.backdrop.selected), WidthLimit: d.widthLimit.value}
}

// look is the chosen chat font and scale.
func (d *extractDialog) look() screenshot.Look {
	return screenshot.Look{Font: screenshot.ChatFonts[d.chatFont.selected], ChatScale: d.chatScale.value}
}

// chatFontLabels names the chat fonts for the Font control, in screenshot.ChatFonts order.
func chatFontLabels() []string {
	labels := make([]string, len(screenshot.ChatFonts))
	for i, chatFont := range screenshot.ChatFonts {
		labels[i] = chatFont.String()
	}
	return labels
}

// renderPreview draws every line, empty ones included, so rows on the picture match the
// editor's lines and an emptied line can still be clicked and typed into.
func (d *extractDialog) renderPreview() {
	lines, style := d.editor.doc.Lines(), d.Style()
	d.renderer.Start(func() (*screenshot.Picture, error) { return screenshot.Render(lines, style) })
}

func (d *extractDialog) receivePreview() {
	result, ok := d.renderer.Result()
	if !ok {
		return
	}
	if result.err != nil {
		slog.Error("draw screenshot preview", "error", result.err)
		d.errText = "Couldn't draw the preview."
		return
	}
	d.errText = ""
	d.editor.SetPicture(result.value)
}

// Update handles the dialog's input and reports whether Save image was clicked, which
// closes it, as Cancel does.
func (d *extractDialog) Update(gtx layout.Context) extractOutcome {
	if !d.isOpen {
		return extractPending
	}
	d.receivePreview()
	isChanged := d.updatePresets(gtx)
	isLookChanged := d.backdrop.Update(gtx)
	isLookChanged = d.chatFont.Update(gtx) || isLookChanged
	isLookChanged = d.chatScale.Update(gtx) || isLookChanged
	isLookChanged = d.widthLimit.Update(gtx) || isLookChanged
	if isLookChanged {
		d.presetName = "" // the look no longer matches the preset
		isChanged = true
	}
	isChanged = d.editor.Update(gtx) || isChanged
	isChanged = d.updateTools(gtx) || isChanged
	if isChanged {
		d.renderPreview()
	}
	if d.edit.Clicked(gtx) {
		d.isEditing = true
		d.editor.isEditable = true
		d.editor.Focus(gtx)
	}
	if d.cancel.Clicked(gtx) {
		d.isOpen = false
	}
	if d.save.Clicked(gtx) && len(d.Lines()) > 0 {
		d.isOpen = false
		return extractSaveRequested
	}
	return extractPending
}

// widthLimitStep is how far one click moves Max width, in image pixels.
const widthLimitStep = 50

// updateTools applies Italic, a color, Default or Merge lines to the editor's selection.
func (d *extractDialog) updateTools(gtx layout.Context) bool {
	if d.mergeLines.Clicked(gtx) && d.editor.MergeSelection() {
		d.editor.Focus(gtx)
		return true
	}
	change := d.clickedTool(gtx)
	if change == nil {
		return false
	}
	d.editor.Restyle(change)
	d.editor.Focus(gtx) // keep typing where the caret was
	return true
}

func (d *extractDialog) clickedTool(gtx layout.Context) func(*screenshot.Document, screenshot.Selection) {
	if d.italic.Clicked(gtx) {
		return (*screenshot.Document).ToggleItalic
	}
	if d.resetColor.Clicked(gtx) {
		return (*screenshot.Document).ResetColor
	}
	for i := range d.swatches {
		if d.swatches[i].Clicked(gtx) {
			c := textSwatches[i]
			return func(doc *screenshot.Document, sel screenshot.Selection) { doc.SetColor(sel, c) }
		}
	}
	return nil
}

func (d *extractDialog) message() string {
	switch {
	case !d.isEditing:
		return "Save " + plural(len(d.Lines()), "line", "lines") + " as they are, or edit them first?"
	case len(d.Lines()) == 0:
		return "Every line is empty. Click the image and type to save something."
	default:
		return "Select text on the image, then pick Italic or a color. Click and type to change the text."
	}
}

// Layout draws the dialog while it is open: the scrolling preview, backdrop and editing
// tools, with the buttons always in view below them.
func (d *extractDialog) Layout(gtx layout.Context, theme *material.Theme) {
	if !d.isOpen {
		return
	}
	d.dialog.Layout(gtx, theme, "Extract image", d.message(), func(gtx layout.Context) layout.Dimensions {
		buttonsHeight := gtx.Dp(spacingXLarge + controlHeight)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-buttonsHeight)
				return d.layoutBody(gtx, theme)
			}),
			layout.Rigid(layout.Spacer{Height: spacingXLarge}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.layoutButtons(gtx, theme) }),
		)
	})
}

// Body rows: the tools while editing, the preview, then the look: preset, background, chat
// font and scale, max width, and Save as preset.
const (
	extractRowTools = iota
	extractRowPreview
	extractRowPreset
	extractRowBackdrop
	extractRowChatFont
	extractRowChatScale
	extractRowWidthLimit
	extractRowPresetNaming
)

func (d *extractDialog) layoutBody(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return styledList(theme, &d.body).Layout(gtx, extractRowPresetNaming+1, func(gtx layout.Context, index int) layout.Dimensions {
		switch index {
		case extractRowTools:
			if !d.isEditing {
				return layout.Dimensions{}
			}
			return layout.Inset{Bottom: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return d.layoutTools(gtx, theme)
			})
		case extractRowPreview:
			return d.layoutPreview(gtx, theme)
		case extractRowPreset:
			return d.layoutPresetRow(gtx, theme)
		case extractRowBackdrop:
			return layout.Inset{Top: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutOptionRow(gtx, theme, "Background", d.backdrop.Layout, "Dark and Light put a box behind each line.")
			})
		case extractRowChatFont:
			return layout.Inset{Top: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutOptionRow(gtx, theme, "Font", d.chatFont.Layout, "The font your chat is set to in game.")
			})
		case extractRowChatScale:
			return layout.Inset{Top: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutOptionRow(gtx, theme, "Chat scale", d.chatScale.Layout, "Your in-game Chat scale. Bigger text makes a bigger image.")
			})
		case extractRowWidthLimit:
			return layout.Inset{Top: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutOptionRow(gtx, theme, "Max width", d.widthLimit.Layout, d.widthLimitHint())
			})
		default:
			return layout.Inset{Top: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return d.naming.Layout(gtx, theme)
			})
		}
	})
}

// widthLimitHint explains Max width, and how to break lines by hand while editing.
func (d *extractDialog) widthLimitHint() string {
	if d.isEditing {
		return "Longer lines wrap to fit. Press Enter for a break of your own."
	}
	return "Longer lines wrap to fit. Use Edit to break lines yourself."
}

// layoutPreview draws the picture editor on a checkerboard, as the image will be saved.
func (d *extractDialog) layoutPreview(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	padding := gtx.Dp(spacingLarge)
	height := gtx.Dp(screenshotPreviewHeight)
	if d.editor.picture != nil {
		shown := d.editor.shownSize(gtx, gtx.Constraints.Max.X)
		height = max(height, shown.Y+2*padding)
	}
	size := image.Pt(gtx.Constraints.Max.X, height)
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusCard)).Push(gtx.Ops).Pop()
	drawChecker(gtx, size)
	gtx.Constraints = layout.Exact(size)
	switch {
	case d.errText != "":
		return layout.Center.Layout(gtx, secondaryText(theme, d.errText).Layout)
	case d.editor.picture == nil:
		return layout.Center.Layout(gtx, caption(theme, "Drawing…").Layout)
	}
	return d.editor.Layout(gtx)
}

// fittedSize is size scaled down to maxWidth when wider, keeping its aspect ratio.
func fittedSize(size image.Point, maxWidth int) image.Point {
	if size.X <= maxWidth || size.X == 0 {
		return size
	}
	return image.Pt(maxWidth, size.Y*maxWidth/size.X)
}

// layoutImageFitted draws img (size pixels) at shown size: 1:1 when they match, so text
// stays crisp, otherwise scaled.
func layoutImageFitted(gtx layout.Context, img paint.ImageOp, size, shown image.Point) layout.Dimensions {
	if shown == size {
		return layoutImageAtPixelSize(gtx, img, size)
	}
	defer clip.Rect{Max: shown}.Push(gtx.Ops).Pop()
	scale := float32(shown.X) / float32(size.X)
	defer op.Affine(f32.AffineId().Scale(f32.Point{}, f32.Pt(scale, scale))).Push(gtx.Ops).Pop()
	img.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: shown}
}

// layoutOptionRow draws one of the dialog's image options: a label in a fixed column (so
// the controls line up), the control, then a one-line hint.
func layoutOptionRow(gtx layout.Context, theme *material.Theme, label string, control func(layout.Context, *material.Theme) layout.Dimensions, hint string) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(optionLabelWidth)
			return secondaryText(theme, label).Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return control(gtx, theme) }),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := caption(theme, hint)
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
	)
}

// layoutTools draws Italic, the color swatches, Default (the automatic colors) and Merge
// lines, with a hint on how they apply.
func (d *extractDialog) layoutTools(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(secondaryButton(&d.italic, "Italic").withIcon(iconItalic).withHeight(controlHeightSmall).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Rigid(verticalDivider),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
	}
	for i := range d.swatches {
		children = append(children,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return layoutSwatch(gtx, &d.swatches[i], textSwatches[i]) }),
			layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
		)
	}
	children = append(children,
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(ghostButton(&d.resetColor, "Default").withHeight(controlHeightSmall).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Rigid(verticalDivider),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			merge := secondaryButton(&d.mergeLines, "Merge lines").withIcon(iconMerge).withHeight(controlHeightSmall)
			return layoutEnabled(gtx, d.editor.CanMergeSelection(), merge.Widget(theme))
		}),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
		}),
		layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
		layout.Rigid(caption(theme, "Drag over the text to select it (Ctrl+A: all). With nothing selected, tools change all the text. Ctrl+Z undoes.").Layout),
		layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
		layout.Rigid(caption(theme, "Drag a line by its grip (or Alt+Up/Down) to move it. Select lines one person said, then Merge lines joins them.").Layout),
	)
}

// layoutSwatch draws a color dot that applies its color when clicked, ringed on hover.
func layoutSwatch(gtx layout.Context, click *widget.Clickable, c color.NRGBA) layout.Dimensions {
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		side := gtx.Dp(swatchSize)
		bounds := image.Rect(0, 0, side, side)
		if click.Hovered() {
			paint.FillShape(gtx.Ops, colorTextPrimary, clip.Ellipse(bounds).Op(gtx.Ops))
		}
		paint.FillShape(gtx.Ops, c, clip.Ellipse(bounds.Inset(gtx.Dp(swatchRingWidth))).Op(gtx.Ops))
		defer clip.Rect(bounds).Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return layout.Dimensions{Size: bounds.Max}
	})
}

func (d *extractDialog) layoutButtons(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(primaryButton(&d.save, "Save image").withIcon(iconImage).Widget(theme)),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if d.isEditing {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: spacingSmall}.Layout(gtx, secondaryButton(&d.edit, "Edit").withIcon(iconPencil).Widget(theme))
		}),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(secondaryButton(&d.cancel, "Cancel").Widget(theme)),
	)
}
