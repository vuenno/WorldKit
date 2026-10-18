package ui

import (
	"fmt"
	"log/slog"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
	"world-kit/internal/screenshot"
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
	screenshots       ScreenshotStore
	saver             *backgroundLoad[screenshot.Shot]
	hasSavedShot      bool
	saveErrorText     string
}

func newLogViewer(session chatlog.Session, library LogLibrary, screenshots ScreenshotStore, invalidate func()) *logViewer {
	v := &logViewer{
		session:     session,
		loader:      newBackgroundLoad[[]chatlog.Record](invalidate),
		filters:     newLogFilters(),
		lines:       newChatLineList(),
		screenshots: screenshots,
		saver:       newBackgroundLoad[screenshot.Shot](invalidate),
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
	v.receiveSavedShot()
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
	slog.Debug("log opened", "path", v.session.Path, "lines", len(v.records))
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
	if v.lines.ExtractClicked(gtx) {
		v.extractSelection()
	}
}

// extractSelection saves the selected lines as a screenshot image in the background.
func (v *logViewer) extractSelection() {
	slog.Debug("extract", "lines", v.lines.SelectedCount())
	lines := screenshotLines(v.lines.SelectedLines())
	v.hasSavedShot = false
	v.saveErrorText = ""
	v.saver.Start(func() (screenshot.Shot, error) { return v.screenshots.Save(lines) })
}

func (v *logViewer) receiveSavedShot() {
	result, ok := v.saver.Result()
	if !ok {
		return
	}
	if result.err != nil {
		slog.Error("save chat screenshot", "error", result.err)
		v.saveErrorText = "Couldn't save the screenshot."
		return
	}
	v.hasSavedShot = true
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
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(caption(theme, v.summaryText()).Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return errorCaption(gtx, theme, v.saveErrorText)
		}),
	)
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
	if v.saver.IsLoading() {
		summary += " Saving screenshot…"
	} else if v.hasSavedShot {
		summary += " Screenshot saved to Log Screenshots."
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
