package ui

import (
	"image/color"
	"strings"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
	"world-kit/internal/screenshot"
)

// chatLine is one saved chat line shown in its in-game color.
type chatLine struct {
	text  string
	color color.NRGBA
}

// newChatLines prepares records for display, each line exactly as stored, minus its
// timestamp unless showsTimestamps.
func newChatLines(records []chatlog.Record, showsTimestamps bool) []chatLine {
	lines := make([]chatLine, len(records))
	for i, record := range records {
		text := record.Raw
		if !showsTimestamps {
			text = record.RawWithoutTimestamp()
		}
		lines[i] = chatLine{text: text, color: chatLineColor(record)}
	}
	return lines
}

// Layout draws the line across the full width, tinted with the accent when isSelected.
func (l chatLine) Layout(gtx layout.Context, theme *material.Theme, isSelected bool) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			if isSelected {
				fill := withOpacity(colorPrimary, lineSelectionOpacity)
				paint.FillShape(gtx.Ops, fill, clip.Rect{Max: gtx.Constraints.Min}.Op())
			}
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			label := material.Label(theme, textSizeChatLine, l.text)
			label.Color = l.color
			return label.Layout(gtx)
		}),
	)
}

// joinChatLines returns the lines' text, one per line.
func joinChatLines(lines []chatLine) string {
	texts := make([]string, len(lines))
	for i, line := range lines {
		texts[i] = line.text
	}
	return strings.Join(texts, "\n")
}

// chatLineColor returns the color GTA World's chat draws a line of this kind in.
func chatLineColor(record chatlog.Record) color.NRGBA {
	switch record.Kind {
	case chatlog.KindSay:
		if strings.Contains(record.Raw, " says [low]: ") {
			return colorChatLow
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

// screenshotLines turns lines into what the screenshot renderer draws, keeping each color.
func screenshotLines(lines []chatLine) []screenshot.Line {
	shotLines := make([]screenshot.Line, len(lines))
	for i, line := range lines {
		shotLines[i] = screenshot.Line{Text: line.text, Color: line.color}
	}
	return shotLines
}
