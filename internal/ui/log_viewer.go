package ui

import (
	"fmt"
	"log/slog"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
)

// kindToggle is one on/off filter for a kind of chat line.
type kindToggle struct {
	kind  chatlog.Kind
	label string
	click widget.Clickable
	isOn  bool
}

// logViewer shows one session's lines, filtered by name and kind, as selectable text.
type logViewer struct {
	session     chatlog.Session
	records     []chatlog.Record
	loader      *backgroundLoad[[]chatlog.Record]
	errorText   string
	back        widget.Clickable
	copy        widget.Clickable
	nameField   widget.Editor
	kindToggles []*kindToggle
	lines       *chatLineList
	hasCopied   bool
}

func newLogViewer(session chatlog.Session, library LogLibrary, invalidate func()) *logViewer {
	v := &logViewer{
		session:   session,
		loader:    newBackgroundLoad[[]chatlog.Record](invalidate),
		nameField: widget.Editor{SingleLine: true},
		lines:     newChatLineList(),
		kindToggles: []*kindToggle{
			{kind: chatlog.KindMe, label: "/me"},
			{kind: chatlog.KindDo, label: "/do"},
			{kind: chatlog.KindOOC, label: "/b (OOC)"},
		},
	}
	v.loader.Start(func() ([]chatlog.Record, error) { return library.Records(session.Path) })
	return v
}

// BackClicked reports whether the user asked to return to the session list.
func (v *logViewer) BackClicked(gtx layout.Context) bool {
	return v.back.Clicked(gtx)
}

// Layout handles input, then draws the header, filters and the session text.
func (v *logViewer) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	v.receiveRecords()
	v.handleInput(gtx)
	return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutHeader(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutFilters(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutSummary(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutText(gtx, theme) }),
		)
	})
}

func (v *logViewer) receiveRecords() {
	result, ok := v.loader.Result()
	if !ok {
		return
	}
	if result.err != nil {
		slog.Error("read chat session", "path", v.session.Path, "error", result.err)
		v.errorText = "Couldn't read this log."
	}
	v.records = result.value
	v.applyFilter()
}

func (v *logViewer) handleInput(gtx layout.Context) {
	isFilterChanged := false
	for {
		event, ok := v.nameField.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.ChangeEvent); ok {
			isFilterChanged = true
		}
	}
	for _, toggle := range v.kindToggles {
		if toggle.click.Clicked(gtx) {
			toggle.isOn = !toggle.isOn
			isFilterChanged = true
		}
	}
	if isFilterChanged {
		v.applyFilter()
	}
	isCopyRequested := v.lines.Update(gtx)
	if v.copy.Clicked(gtx) || isCopyRequested {
		copyToClipboard(gtx, v.lines.CopyText())
		v.hasCopied = true
	}
}

func (v *logViewer) applyFilter() {
	visible := v.filter().Apply(v.records)
	v.lines.SetLines(newChatLines(visible))
	v.hasCopied = false
}

func (v *logViewer) filter() chatlog.Filter {
	filter := chatlog.Filter{Name: v.nameField.Text()}
	for _, toggle := range v.kindToggles {
		if toggle.isOn {
			filter.Kinds = append(filter.Kinds, toggle.kind)
		}
	}
	return filter
}

func (v *logViewer) layoutHeader(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return secondaryButton(&v.back, "Back").Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pageTitle(theme, sessionTitle(v.session)).Layout),
				layout.Rigid(caption(theme, sessionNumberText(v.session)+" · "+sessionUpdatedText(v.session)).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return primaryButton(&v.copy, "Copy").Layout(gtx, theme)
		}),
	)
}

func (v *logViewer) layoutFilters(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return textField(gtx, theme, &v.nameField, "Filter by name, e.g. John Doe")
		}),
	}
	for _, toggle := range v.kindToggles {
		children = append(children,
			layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return toggleButton(&toggle.click, toggle.label, toggle.isOn).Layout(gtx, theme)
			}),
		)
	}
	return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx, children...)
}

func (v *logViewer) layoutSummary(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if v.errorText != "" {
		return errorCaption(gtx, theme, v.errorText)
	}
	return caption(theme, v.summaryText()).Layout(gtx)
}

func (v *logViewer) summaryText() string {
	if v.loader.IsLoading() && v.records == nil {
		return "Loading…"
	}
	summary := fmt.Sprintf("Showing %d of %d lines.", v.lines.Len(), len(v.records))
	if count := v.lines.SelectedCount(); count > 0 {
		summary += fmt.Sprintf(" %d selected.", count)
	}
	if v.hasCopied {
		summary += " Copied to the clipboard."
	}
	return summary
}

// layoutText draws the session's lines on a panel that fills the remaining height.
func (v *logViewer) layoutText(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min = gtx.Constraints.Max
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return outlinedSurface(gtx, colorSurface)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			return layout.UniformInset(spacingMedium).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return v.layoutLines(gtx, theme)
			})
		}),
	)
}

func (v *logViewer) layoutLines(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if v.lines.Len() == 0 && v.records != nil {
		return caption(theme, "No lines match these filters.").Layout(gtx)
	}
	return v.lines.Layout(gtx, theme)
}

