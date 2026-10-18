package ui

import (
	"fmt"
	"log/slog"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
)

// logViewer shows one session's lines, filtered by action and character, as colored lines.
type logViewer struct {
	session           chatlog.Session
	records           []chatlog.Record
	loader            *backgroundLoad[[]chatlog.Record]
	errorText         string
	back              widget.Clickable
	copy              widget.Clickable
	filterButton      widget.Clickable
	isFilterPanelOpen bool
	filters           *logFilters
	lines             *chatLineList
	hasCopied         bool
}

func newLogViewer(session chatlog.Session, library LogLibrary, invalidate func()) *logViewer {
	v := &logViewer{
		session: session,
		loader:  newBackgroundLoad[[]chatlog.Record](invalidate),
		filters: newLogFilters(),
		lines:   newChatLineList(),
	}
	v.loader.Start(func() ([]chatlog.Record, error) { return library.Records(session.Path) })
	return v
}

// BackClicked reports whether the user asked to return to the session list.
func (v *logViewer) BackClicked(gtx layout.Context) bool {
	return v.back.Clicked(gtx)
}

// Layout handles input, then draws the header, the filter panel when open and the session text.
func (v *logViewer) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	v.receiveRecords()
	v.handleInput(gtx)
	return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutHeader(gtx, theme) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutFilterPanel(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
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
	v.filters.SetCharacters(chatlog.Characters(v.records))
	v.applyFilter()
}

func (v *logViewer) handleInput(gtx layout.Context) {
	if v.filterButton.Clicked(gtx) {
		v.isFilterPanelOpen = !v.isFilterPanelOpen
	}
	if v.filters.Update(gtx) {
		v.applyFilter()
	}
	isCopyRequested := v.lines.Update(gtx)
	if v.copy.Clicked(gtx) || isCopyRequested {
		copyToClipboard(gtx, v.lines.CopyText())
		v.hasCopied = true
	}
}

func (v *logViewer) applyFilter() {
	visible := v.filters.Filter().Apply(v.records)
	v.lines.SetLines(newChatLines(visible, v.filters.ShowsTimestamps()))
	v.hasCopied = false
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
			activeCount := v.filters.ActiveCount()
			return toggleButton(&v.filterButton, filterButtonLabel(activeCount), activeCount > 0).Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return primaryButton(&v.copy, "Copy").Layout(gtx, theme)
		}),
	)
}

// filterButtonLabel names the button and how many filter options are on.
func filterButtonLabel(activeCount int) string {
	if activeCount == 0 {
		return "Filters"
	}
	return fmt.Sprintf("Filters (%d)", activeCount)
}

func (v *logViewer) layoutFilterPanel(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if !v.isFilterPanelOpen {
		return layout.Dimensions{}
	}
	return layout.Inset{Top: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return v.filters.Layout(gtx, theme)
	})
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

