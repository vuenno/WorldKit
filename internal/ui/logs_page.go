package ui

import (
	"image"
	"image/color"
	"log/slog"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/capture"
	"world-kit/internal/chatlog"
)

// sessionEntry is a saved session with what the list shows about it.
type sessionEntry struct {
	session chatlog.Session
	summary chatlog.Summary
	click   *widget.Clickable
}

// loadSessionEntries lists the sessions and summarizes each one. A session that can't be
// read is still listed, with an empty summary.
func loadSessionEntries(library LogLibrary) ([]sessionEntry, error) {
	sessions, err := library.Sessions()
	if err != nil {
		return nil, err
	}
	entries := make([]sessionEntry, len(sessions))
	for i, session := range sessions {
		entries[i] = sessionEntry{session: session, click: new(widget.Clickable)}
		records, err := library.Records(session.Path)
		if err != nil {
			slog.Warn("summarize chat session", "path", session.Path, "error", err)
			continue
		}
		entries[i].summary = chatlog.Summarize(records)
	}
	return entries, nil
}

// LogsPage lists the saved chat sessions, newest first and grouped by day, and opens one in
// the log viewer. The session being written right now is shown on its own, as "Live now".
type LogsPage struct {
	entries    []sessionEntry
	hasLoaded  bool
	errorText  string
	loader     *backgroundLoad[[]sessionEntry]
	search     widget.Editor
	openLive   widget.Clickable
	list       widget.List
	viewer     *logViewer
	lastSaved  time.Time
	invalidate func()
}

// NewLogsPage creates the page. invalidate redraws the window once a background read finishes.
func NewLogsPage(invalidate func()) *LogsPage {
	return &LogsPage{
		loader:     newBackgroundLoad[[]sessionEntry](invalidate),
		search:     widget.Editor{SingleLine: true},
		list:       widget.List{List: layout.List{Axis: layout.Vertical}},
		invalidate: invalidate,
	}
}

// Reload re-reads the session list in the background.
func (p *LogsPage) Reload(library LogLibrary) {
	p.loader.Start(func() ([]sessionEntry, error) { return loadSessionEntries(library) })
}

// Viewer returns the open log viewer, or nil on the session list.
func (p *LogsPage) Viewer() *logViewer {
	return p.viewer
}

// Layout handles input, then draws the open session or the session list.
func (p *LogsPage) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	p.receiveEntries()
	status := services.Capture.Status()
	if !status.LastSaved.Equal(p.lastSaved) {
		p.lastSaved = status.LastSaved
		p.Reload(services.Logs)
	}
	if p.viewer != nil && p.viewer.BackClicked(gtx) {
		p.viewer = nil
	}
	if p.viewer != nil {
		return p.viewer.Layout(gtx, theme)
	}
	p.openClickedSession(gtx, services, status)
	return p.layoutList(gtx, theme, status)
}

func (p *LogsPage) receiveEntries() {
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
	p.entries = result.value
}

func (p *LogsPage) openClickedSession(gtx layout.Context, services Services, status capture.Status) {
	live, hasLive := p.liveEntry(status)
	if hasLive && p.openLive.Clicked(gtx) {
		p.open(live, services, true)
		return
	}
	for _, entry := range p.entries {
		if entry.click.Clicked(gtx) {
			p.open(entry, services, hasLive && entry.session.Path == live.session.Path)
			return
		}
	}
}

func (p *LogsPage) open(entry sessionEntry, services Services, isLive bool) {
	p.viewer = newLogViewer(viewerOptions{
		session:     entry.session,
		isLive:      isLive,
		library:     services.Logs,
		screenshots: services.Screenshots,
		settings:    services.Settings,
		invalidate:  p.invalidate,
	})
}

// liveEntry returns the session capture is writing to: the newest one, once this run has
// saved lines.
func (p *LogsPage) liveEntry(status capture.Status) (sessionEntry, bool) {
	if len(p.entries) == 0 || status.State != capture.StateWatching || status.LastSaved.IsZero() {
		return sessionEntry{}, false
	}
	return p.entries[0], true
}

// visibleEntries returns the sessions matching the search box, live one excluded.
func (p *LogsPage) visibleEntries(status capture.Status) []sessionEntry {
	query := strings.ToLower(strings.TrimSpace(p.search.Text()))
	live, hasLive := p.liveEntry(status)
	var visible []sessionEntry
	for _, entry := range p.entries {
		if hasLive && entry.session.Path == live.session.Path {
			continue
		}
		if query == "" || entryMatches(entry, query) {
			visible = append(visible, entry)
		}
	}
	return visible
}

// entryMatches reports whether a session has a character or a date containing query.
func entryMatches(entry sessionEntry, query string) bool {
	if strings.Contains(strings.ToLower(entry.session.Started.Format(fullLayout)), query) {
		return true
	}
	for _, character := range entry.summary.Characters {
		if strings.Contains(strings.ToLower(character.Name), query) {
			return true
		}
	}
	return false
}

func (p *LogsPage) layoutList(gtx layout.Context, theme *material.Theme, status capture.Status) layout.Dimensions {
	items := p.listItems(theme, status, gtx.Now)
	inset := layout.Inset{Left: spacingPage, Right: spacingPage, Top: spacingSmall}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		list := styledList(theme, &p.list)
		return list.Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(contentMaxWidth))
			return items[index](gtx)
		})
	})
}

// listItems builds the scrolling page: header, live card, then day groups of sessions.
func (p *LogsPage) listItems(theme *material.Theme, status capture.Status, now time.Time) []layout.Widget {
	items := []layout.Widget{func(gtx layout.Context) layout.Dimensions { return p.layoutHeader(gtx, theme) }}
	if p.errorText != "" {
		items = append(items, func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, p.errorText) })
	}
	if live, ok := p.liveEntry(status); ok {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: spacingXLarge}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return liveCard{entry: live, open: &p.openLive, lastSaved: status.LastSaved, now: now}.Layout(gtx, theme)
			})
		})
	}
	visible := p.visibleEntries(status)
	if len(visible) == 0 {
		return append(items, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: spacingXLarge}.Layout(gtx, secondaryText(theme, p.emptyText(status)).Layout)
		})
	}
	lastGroup := ""
	for _, entry := range visible {
		if group := dayGroupLabel(entry.session.Started, now); group != lastGroup {
			lastGroup = group
			items = append(items, func(gtx layout.Context) layout.Dimensions {
				inset := layout.Inset{Top: spacingXLarge, Bottom: spacingMedium, Left: spacingTiny}
				return inset.Layout(gtx, groupLabel(theme, group).Layout)
			})
		}
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Bottom: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return sessionRow{entry: entry, now: now}.Layout(gtx, theme)
			})
		})
	}
	return append(items, layout.Spacer{Height: spacingXLarge}.Layout)
}

func (p *LogsPage) emptyText(status capture.Status) string {
	switch {
	case !p.hasLoaded:
		return "Loading saved logs…"
	case p.search.Len() > 0:
		return "No sessions match your search."
	case len(p.entries) > 0 && p.errorText == "":
		return "No other sessions yet." // only the live one exists
	case status.State == capture.StateWatching:
		return "No chat saved yet. Join GTA World and your sessions show up here."
	default:
		return "No chat saved yet. " + captureStatusText(status)
	}
}

func (p *LogsPage) layoutHeader(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return pageHeader(gtx, theme, "Logs", "Every GTA World session, saved as it happens.",
		func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = min(gtx.Dp(searchFieldWidth), gtx.Constraints.Max.X/2)
			gtx.Constraints.Max.X = gtx.Constraints.Min.X
			return textField{editor: &p.search, hint: "Search by character or date", icon: iconSearch}.Layout(gtx, theme)
		})
}

// liveCard features the session being captured right now.
type liveCard struct {
	entry     sessionEntry
	open      *widget.Clickable
	lastSaved time.Time
	now       time.Time
}

func (c liveCard) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return card(gtx, spacingXLarge-spacingTiny, func(gtx layout.Context) layout.Dimensions {
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return c.layoutText(gtx, theme) }),
			layout.Rigid(layout.Spacer{Width: spacingXLarge}.Layout),
			layout.Rigid(primaryButton(c.open, "Open live log").withIcon(iconArrowRight).Widget(theme)),
		)
	})
}

func (c liveCard) layoutText(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	summary := c.entry.summary
	meta := plural(summary.Lines, "line", "lines") + " · last line " + c.lastSaved.UTC().Format("15:04:05") +
		" · " + charactersText(summary.Characters, 2)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return statusPill{label: "Live now", color: colorLive}.Layout(gtx, theme)
				}),
				layout.Rigid(layout.Spacer{Width: spacingMedium - spacingTiny/2}.Layout),
				layout.Rigid(secondaryText(theme, sessionNumberText(c.entry.session)).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
		layout.Rigid(screenTitle(theme, dayText(c.entry.session.Started, c.now)+", started "+
			c.entry.session.Started.UTC().Format(clockLayout)+" UTC").Layout),
		layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
		layout.Rigid(secondaryText(theme, meta).Layout),
	)
}

func sessionNumberText(session chatlog.Session) string {
	return "Session #" + countText(session.Number)
}

// sessionRow is one clickable session: start time and length, day and characters, line
// count with a bar of what kind of chat it was, and a chevron.
type sessionRow struct {
	entry sessionEntry
	now   time.Time
}

func (r sessionRow) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	click := r.entry.click
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				fill := colorSurface
				if click.Hovered() {
					fill = colorElevatedSurface
				}
				dims := outlinedSurface(gtx, fill, colorDivider, radiusControl+spacingTiny/2)
				defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
				pointer.CursorPointer.Add(gtx.Ops)
				return dims
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				inset := layout.Inset{Top: spacingMedium + spacingTiny/2, Bottom: spacingMedium + spacingTiny/2, Left: spacingLarge, Right: spacingLarge}
				return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return r.layoutColumns(gtx, theme) })
			}),
		)
	})
}

func (r sessionRow) layoutColumns(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	summary := r.entry.summary
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(sessionTimeColumnWidth)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := monoText(theme, textSizeSessionTime, r.entry.session.Started.UTC().Format(clockLayout))
					label.Font.Weight = font.Medium
					return label.Layout(gtx)
				}),
				layout.Rigid(styledLabel(theme, textSizeTiny, font.Normal, colorTextTertiary, durationText(summary.Duration())).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(styledLabel(theme, textSizeBody, font.SemiBold, colorTextPrimary, dateText(r.entry.session.Started, r.now)).Layout),
				layout.Rigid(layout.Spacer{Height: spacingTiny + spacingTiny/2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := secondaryText(theme, charactersText(summary.Characters, 3))
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(sessionStatsColumnWidth)
			return layout.Flex{Axis: layout.Vertical, Alignment: layout.End}.Layout(gtx,
				layout.Rigid(caption(theme, plural(summary.Lines, "line", "lines")).Layout),
				layout.Rigid(layout.Spacer{Height: spacingTiny + spacingTiny/2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return kindBar(gtx, summary) }),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return iconChevronRight.Layout(gtx, iconSize, colorTimestamp)
		}),
	)
}

// kindBar shows what a session was made of: says, emotes, OOC and PMs, in chat colors.
func kindBar(gtx layout.Context, summary chatlog.Summary) layout.Dimensions {
	size := image.Pt(gtx.Dp(kindBarWidth), gtx.Dp(kindBarHeight))
	defer clip.UniformRRect(image.Rectangle{Max: size}, size.Y/2).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colorDivider)
	if summary.Lines == 0 {
		return layout.Dimensions{Size: size}
	}
	x := 0
	for _, part := range []struct {
		count int
		color color.NRGBA
	}{{summary.Says, colorChatSay}, {summary.Emotes, colorChatEmote}, {summary.OOC, colorTimestamp}, {summary.PMs, colorChatPM}} {
		width := size.X * part.count / summary.Lines
		paint.FillShape(gtx.Ops, part.color, clip.Rect(image.Rect(x, 0, x+width, size.Y)).Op())
		x += width
	}
	return layout.Dimensions{Size: size}
}

// styledList is a material list with the design's quiet scrollbar.
func styledList(theme *material.Theme, list *widget.List) material.ListStyle {
	style := material.List(theme, list)
	style.Indicator.Color = colorBorder
	style.Indicator.HoverColor = colorTextTertiary
	return style
}
