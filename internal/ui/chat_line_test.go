package ui

import (
	"image/color"
	"strings"
	"testing"

	"world-kit/internal/chatlog"
)

func TestChatLineColor(t *testing.T) {
	tests := []struct {
		line string
		want color.NRGBA
	}{
		{"[DATE: 18/OCT/2026 | TIME: 09:57:01]", colorChatHeader},
		{"[09:57:02] John Doe says: hi", colorChatSay},
		{"[09:57:03] John Doe says [low]: quiet", colorChatLow},
		{"[09:57:04] John Doe shouts: hey!", colorChatSay},
		{"[09:57:05] John Doe whispers: psst", colorChatWhisper},
		{"[09:57:06] * John Doe waves.", colorChatEmote},
		{"[09:57:07] * The door is locked. (( John Doe ))", colorChatEmote},
		{"[09:57:08] (( John Doe: brb ))", colorChatOOC},
		{"[09:57:09] (( PM from (169) Jane Roe: hi ))", colorChatPM},
		{"[09:57:10] Welcome to GTA World.", colorChatSay},
	}
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			records, err := chatlog.Parse(strings.NewReader(test.line))
			if err != nil {
				t.Fatal(err)
			}
			if got := chatLineColor(records[0]); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
