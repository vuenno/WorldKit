package ui

import (
	"fmt"
	"image"
	"log/slog"
	"path/filepath"
	"time"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
)

// sessionTimeLayout shows session times; the game's clock is UTC, so they are shown as UTC.
const sessionTimeLayout = "Mon 02 Jan 2006, 15:04 UTC"

// LogsPage lists the saved chat sessions and opens one in the log viewer.
type LogsPage struct {
	sessions   []chatlog.Session
	hasLoaded  bool
	errorText  string
	loader     *backgroundLoad[[]chatlog.Session]
	rowClicks  []widget.Clickable
	list       layout.List
	viewer     *logViewer
	lastSaved  time.Time
	invalidate func()
}

// NewLogsPage creates the page. invalidate redraws the window once a background read finishes.
func NewLogsPage(invalidate func()) *LogsPage {
	return &LogsPage{
		loader:     newBackgroundLoad[[]chatlog.Session](invalidate),
		list:       layout.List{Axis: layout.Vertical},
		invalidate: invalidate,
	}
}

// Reload re-reads the session list in the background.
func (p *LogsPage) Reload(library LogLibrary) {
	p.loader.Start(library.Sessions)
}

// Layout handles input, then draws the open session or the session list.
func (p *LogsPage) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	p.receiveSessions()
	if lastSaved := services.Capture.Status().LastSaved; !lastSaved.Equal(p.lastSaved) {
		p.lastSaved = lastSaved
		p.Reload(services.Logs)
	}
	if p.viewer != nil && p.viewer.BackClicked(gtx) {
		p.viewer = nil
	}
	if p.viewer != nil {
		return p.viewer.Layout(gtx, theme)
	}
	p.openClickedSession(gtx, services.Logs)
	return p.layoutSessionList(gtx, theme, services)
}

func (p *LogsPage) receiveSessions() {
	result, ok := p.loader.Result()
	if !ok {
		return
	}
	p.hasLoaded = true
	p.errorText = ""
	if result.err != nil {
		slog.Error("list chat sessions", "error", result.err)
		p.errorText = "Couldn't read the saved logs."
		return
	}
	p.sessions = result.value
	for len(p.rowClicks) < len(p.sessions) {
		p.rowClicks = append(p.rowClicks, widget.Clickable{})
	}
}

func (p *LogsPage) openClickedSession(gtx layout.Context, library LogLibrary) {
	for index, session := range p.sessions {
		if p.rowClicks[index].Clicked(gtx) {
			p.viewer = newLogViewer(session, library, p.invalidate)
			return
		}
	}
}

func (p *LogsPage) layoutSessionList(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(contentMaxWidth))
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pageTitle(theme, "Logs").Layout),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(caption(theme, captureStatusText(services.Capture.Status())).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return errorCaption(gtx, theme, p.errorText)
			}),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutSessions(gtx, theme)
			}),
		)
	})
}

func (p *LogsPage) layoutSessions(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if len(p.sessions) == 0 && p.errorText != "" {
		return layout.Dimensions{}
	}
	if len(p.sessions) == 0 {
		return caption(theme, emptySessionsText(p.hasLoaded)).Layout(gtx)
	}
	return p.list.Layout(gtx, len(p.sessions), func(gtx layout.Context, index int) layout.Dimensions {
		row := sessionRow{click: &p.rowClicks[index], session: p.sessions[index]}
		if index == 0 {
			return row.Layout(gtx, theme)
		}
		return layout.Inset{Top: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return row.Layout(gtx, theme)
		})
	})
}

func emptySessionsText(hasLoaded bool) string {
	if !hasLoaded {
		return "Loading saved logs…"
	}
	return "No chat saved yet. Sessions show up here once GTA World chat is captured."
}

// sessionRow is one clickable session in the list: when it started, when it was last
// written and its number in the archive.
type sessionRow struct {
	click   *widget.Clickable
	session chatlog.Session
}

func (r sessionRow) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return r.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(r.drawBackground),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(spacingMedium).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return r.layoutContent(gtx, theme)
				})
			}),
		)
	})
}

func (r sessionRow) drawBackground(gtx layout.Context) layout.Dimensions {
	fill := colorSurface
	if r.click.Hovered() {
		fill = colorElevatedSurface
	}
	dims := outlinedSurface(gtx, fill)
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (r sessionRow) layoutContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(sectionTitle(theme, sessionTitle(r.session)).Layout),
				layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
				layout.Rigid(caption(theme, sessionUpdatedText(r.session)).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Rigid(caption(theme, sessionNumberText(r.session)).Layout),
	)
}

// sessionTitle names a session by when it started, or by its file when that is unknown.
func sessionTitle(session chatlog.Session) string {
	if session.Started.IsZero() {
		return filepath.Base(session.Path)
	}
	return session.Started.Format(sessionTimeLayout)
}

func sessionUpdatedText(session chatlog.Session) string {
	return "Last updated " + session.Updated.UTC().Format(sessionTimeLayout)
}

func sessionNumberText(session chatlog.Session) string {
	return fmt.Sprintf("Session #%d", session.Number)
}
