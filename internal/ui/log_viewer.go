package ui

import (
	"image"
	"log/slog"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
	"world-kit/internal/screenshot"
)

// viewerOptions is what the log viewer needs to show one session.
type viewerOptions struct {
	session     chatlog.Session
	isLive      bool // capture is still writing to this session
	library     LogLibrary
	screenshots ScreenshotStore
	settings    SettingsStore // where extract presets are kept
	invalidate  func()
}

// logViewer shows one session's lines in their in-game colors, with filters on top. Lines
// can be selected to copy them or extract them as a screenshot image.
type logViewer struct {
	options   viewerOptions
	records   []chatlog.Record
	loader    *backgroundLoad[[]chatlog.Record]
	errorText string
	back      widget.Clickable
	copyAll   widget.Clickable
	filters   *logFilters
	lines     *chatLineList
	extract   *extractDialog
	saver     *backgroundLoad[screenshot.Shot]
	savedShot string // path of the last saved screenshot, for the toast's View
	toast     toast
}

func newLogViewer(options viewerOptions) *logViewer {
	v := &logViewer{
		options: options,
		loader:  newBackgroundLoad[[]chatlog.Record](options.invalidate),
		filters: newLogFilters(),
		lines:   newChatLineList(),
		extract: newExtractDialog(options.invalidate, options.settings),
		saver:   newBackgroundLoad[screenshot.Shot](options.invalidate),
	}
	v.loader.Start(func() ([]chatlog.Record, error) { return options.library.Records(options.session.Path) })
	return v
}

// BackClicked reports whether the user asked to return to the session list.
func (v *logViewer) BackClicked(gtx layout.Context) bool {
	return v.back.Clicked(gtx)
}

// ViewShotClicked reports the screenshot to open when the user clicks View on the
// "Screenshot saved" toast.
func (v *logViewer) ViewShotClicked(gtx layout.Context) (string, bool) {
	if !v.toast.ActionClicked(gtx) {
		return "", false
	}
	return v.savedShot, true
}

// Layout handles input, then draws the header, filters and lines, and the extract dialog
// on top while it is open.
func (v *logViewer) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	v.receiveRecords()
	v.receiveSavedShot(gtx)
	v.handleInput(gtx)
	dims := v.layoutContent(gtx, theme)
	v.extract.Layout(gtx, theme)
	return dims
}

func (v *logViewer) layoutContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	inset := layout.Inset{Left: spacingPage, Right: spacingPage, Top: spacingSmall, Bottom: spacingXLarge}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutHeader(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingLarge}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.filters.LayoutToolbar(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutSummary(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutPanel(gtx, theme) }),
		)
	})
}

func (v *logViewer) receiveRecords() {
	result, ok := v.loader.Result()
	if !ok {
		return
	}
	if result.err != nil {
		slog.Error("read chat session", "path", v.options.session.Path, "error", result.err)
		v.errorText = "Couldn't read this log."
	}
	v.records = result.value
	v.filters.SetCharacters(chatlog.Characters(v.records))
	v.applyFilter()
}

func (v *logViewer) receiveSavedShot(gtx layout.Context) {
	result, ok := v.saver.Result()
	if !ok {
		return
	}
	if result.err != nil {
		slog.Error("save chat screenshot", "error", result.err)
		v.toast.ShowError(gtx.Now, "Couldn't save the screenshot.")
		return
	}
	v.savedShot = result.value.Path
	v.toast.Show(gtx.Now, "Screenshot saved", "View")
}

func (v *logViewer) handleInput(gtx layout.Context) {
	if v.filters.Update(gtx) {
		v.applyFilter()
	}
	if v.lines.Update(gtx) {
		v.copy(gtx, v.lines.CopyText(), v.lines.SelectedCount())
	}
	if v.copyAll.Clicked(gtx) {
		v.copy(gtx, v.lines.AllText(), v.lines.Len())
	}
	if v.lines.ExtractClicked(gtx) {
		v.extract.Open(v.lines.SelectedLines())
	}
	if v.extract.Update(gtx) == extractSaveRequested {
		v.saveShot(v.extract.Lines(), v.extract.Style())
	}
}

func (v *logViewer) copy(gtx layout.Context, text string, lineCount int) {
	copyToClipboard(gtx, text)
	if lineCount == 0 {
		lineCount = v.lines.Len() // Ctrl+C with nothing selected copies every shown line
	}
	v.toast.Show(gtx.Now, "Copied "+plural(lineCount, "line", "lines"), "")
}

// saveShot saves lines as a screenshot image in the background.
func (v *logViewer) saveShot(lines []screenshot.Line, style screenshot.Style) {
	v.saver.Start(func() (screenshot.Shot, error) { return v.options.screenshots.Save(lines, style) })
}

func (v *logViewer) applyFilter() {
	var visible []chatlog.Record
	for _, record := range v.filters.Filter().Apply(v.records) {
		if v.filters.Matches(record) {
			visible = append(visible, record)
		}
	}
	showsTimestamps := v.filters.ShowsTimestamps()
	v.lines.SetLines(newChatLines(visible, showsTimestamps), showsTimestamps)
}

func (v *logViewer) layoutHeader(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return iconButton(&v.back, iconArrowLeft).Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutTitle(gtx, theme) }),
				layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
				layout.Rigid(secondaryText(theme, v.metaText()).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return secondaryButton(&v.copyAll, "Copy all").withIcon(iconCopy).Layout(gtx, theme)
		}),
	)
}

func (v *logViewer) layoutTitle(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(screenTitle(theme, dayAndTimeText(v.options.session.Started, gtx.Now)).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !v.options.isLive {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: spacingMedium - spacingTiny/2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return statusPill{label: "Live", color: colorLive}.Layout(gtx, theme)
			})
		}),
	)
}

func (v *logViewer) metaText() string {
	session := v.options.session
	text := sessionNumberText(session)
	if v.records != nil {
		text += " · " + plural(chatlog.Summarize(v.records).Lines, "line", "lines")
	}
	return text + " · updated " + session.Updated.UTC().Format(clockLayout)
}

func (v *logViewer) layoutSummary(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	switch {
	case v.errorText != "":
		return errorCaption(gtx, theme, v.errorText)
	case v.loader.IsLoading() && v.records == nil:
		return caption(theme, "Loading…").Layout(gtx)
	default:
		return v.filters.LayoutSummary(gtx, theme, v.lines.Len(), len(v.records))
	}
}

// layoutPanel draws the lines on the dark log panel, with the toast in its bottom-right corner.
func (v *logViewer) layoutPanel(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return outlinedSurface(gtx, colorSidebar, colorDivider, radiusCard)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.Inset{Top: spacingMedium, Bottom: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutLines(gtx, theme)
			})
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return v.layoutToast(gtx, theme)
		}),
	)
}

func (v *logViewer) layoutLines(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if v.lines.Len() == 0 && v.records != nil {
		return layout.Center.Layout(gtx, secondaryText(theme, "No lines match these filters.").Layout)
	}
	return v.lines.Layout(gtx, theme)
}

// layoutToast pins the toast to the panel's bottom-right corner.
func (v *logViewer) layoutToast(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dims := v.toast.Layout(gtx, theme)
	toast := recording.Stop()
	margin := gtx.Dp(spacingLarge)
	origin := image.Pt(gtx.Constraints.Max.X-dims.Size.X-margin, gtx.Constraints.Max.Y-dims.Size.Y-margin)
	defer op.Offset(origin).Push(gtx.Ops).Pop()
	toast.Add(gtx.Ops)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}
