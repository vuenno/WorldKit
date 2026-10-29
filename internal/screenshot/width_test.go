package screenshot

import (
	"strings"
	"testing"
)

func TestRenderFitsTheWidthLimit(t *testing.T) {
	long := sayLine("John Doe says: " + strings.Repeat("this sentence keeps going and going ", 8))
	tests := []struct {
		name  string
		lines []Line
		style Style
	}{
		{"wraps long lines", []Line{long}, Style{WidthLimit: 500}},
		{"big font, narrow image", []Line{long}, Style{Look: Look{ChatScale: MaxChatScale}, WidthLimit: 300}},
		{"one word longer than the limit", []Line{sayLine(strings.Repeat("W", 80))}, Style{WidthLimit: MinWidthLimit}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			picture := render(t, test.lines, test.style)
			if width := picture.Image.Bounds().Dx(); width > test.style.WidthLimit {
				t.Errorf("image is %d px wide, want at most %d", width, test.style.WidthLimit)
			}
			if rows := len(picture.rows); rows < 2 {
				t.Errorf("got %d rows, want the text wrapped", rows)
			}
		})
	}
}

func TestRenderKeepsManualLineBreaks(t *testing.T) {
	// Enter in the picture editor starts a new line; wrapping must keep it its own row.
	lines := []Line{sayLine("Short one."), sayLine("Another.")}
	picture := render(t, lines, Style{WidthLimit: 1200})
	if len(picture.rows) != 2 || picture.rows[1].line != 1 {
		t.Errorf("rows = %d, want each line on its own row", len(picture.rows))
	}
}

func TestDefaultWidthLimitIsTheInGameWidth(t *testing.T) {
	long := []Line{sayLine("John Doe says: " + strings.Repeat("lorem ipsum dolor ", 30))}
	unset := render(t, long, Style{})
	explicit := render(t, long, Style{WidthLimit: DefaultWidthLimit()})
	if unset.Image.Bounds() != explicit.Image.Bounds() {
		t.Errorf("no limit draws %v, the default limit %v; want the same", unset.Image.Bounds(), explicit.Image.Bounds())
	}
	if got := ClampWidthLimit(50); got != MinWidthLimit {
		t.Errorf("ClampWidthLimit(50) = %d, want %d", got, MinWidthLimit)
	}
}
