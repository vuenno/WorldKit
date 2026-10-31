package ui

import (
	"image"
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
)

// chatLine is one saved chat line shown in its in-game color, with its timestamp in a
// column of its own when timestamps are on.
type chatLine struct {
	time     string // "09:57:03", or "" when hidden or the line has none
	text     string // the line without its timestamp
	copyText string // what Copy and Extract use: the stored line, minus the timestamp when hidden
	color    color.NRGBA
}

// newChatLines prepares records for display, each line exactly as stored, its timestamp
// split off into the time column (or dropped when !showsTimestamps).
func newChatLines(records []chatlog.Record, showsTimestamps bool) []chatLine {
	lines := make([]chatLine, len(records))
	for i, record := range records {
		text := record.RawWithoutTimestamp()
		line := chatLine{text: text, copyText: text, color: chatLineColor(record)}
		if showsTimestamps {
			line.copyText = record.Raw
			if text != record.Raw {
				line.time = record.Time.Format("15:04:05")
			}
		}
		lines[i] = line
	}
	return lines
}

// Layout draws the line across the full width. Selected lines get an accent tint and a bar
// at their left edge; showsTimeColumn reserves the timestamp column even when this line has none.
func (l chatLine) Layout(gtx layout.Context, theme *material.Theme, isSelected, showsTimeColumn bool) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			if isSelected {
				paint.FillShape(gtx.Ops, withOpacity(colorPrimary, lineSelectionOpacity), clip.Rect{Max: size}.Op())
				bar := image.Rect(0, 0, gtx.Dp(activeIndicatorWidth), size.Y)
				paint.FillShape(gtx.Ops, colorPrimary, clip.Rect(bar).Op())
			}
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			inset := layout.Inset{Left: spacingLarge + spacingTiny, Right: spacingLarge + spacingTiny, Top: chatLinePaddingY, Bottom: chatLinePaddingY}
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return l.layoutColumns(gtx, theme, showsTimeColumn)
			})
		}),
	)
}

func (l chatLine) layoutColumns(gtx layout.Context, theme *material.Theme, showsTimeColumn bool) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if !showsTimeColumn {
				return layout.Dimensions{}
			}
			gtx.Constraints.Min.X = gtx.Dp(chatTimestampWidth)
			label := monoText(theme, textSizeCaption, l.time)
			label.Color = colorTimestamp
			return label.Layout(gtx)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := material.Label(theme, textSizeBody, l.text)
			label.Color = l.color
			label.LineHeight = chatLineHeight
			return label.Layout(gtx)
		}),
	)
}

// joinChatLines returns the lines' copy text, one per line.
func joinChatLines(lines []chatLine) string {
	texts := make([]string, len(lines))
	for i, line := range lines {
		texts[i] = line.copyText
	}
	return strings.Join(texts, "\n")
}

// chatLineColor returns the color GTA World's chat draws a line of this kind in.
func chatLineColor(record chatlog.Record) color.NRGBA {
	switch record.Kind {
	case chatlog.KindSay:
		switch {
		case strings.Contains(record.Raw, " says [low]: "):
			return colorChatLow
		case strings.Contains(record.Raw, " says [lower]: "):
			return colorChatLower
		}
		return colorChatSay
	case chatlog.KindWhisper:
		return colorChatWhisper
	case chatlog.KindMe, chatlog.KindDo:
		return colorChatEmote
	case chatlog.KindOOC:
		return colorChatOOC
	case chatlog.KindPM:
		return colorChatPM
	case chatlog.KindHeader:
		return colorChatHeader
	default:
		return colorChatSay
	}
}
