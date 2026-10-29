package screenshot

import (
	"fmt"
	"math"
	"strings"
)

// ChatFont is the font GTA World's chat is set to; players pick it in the game's settings.
type ChatFont int

// The chat fonts WorldKit can draw.
const (
	FontArial ChatFont = iota
	FontMukta
)

// ChatFonts lists every chat font, in the order the editor shows them.
var ChatFonts = []ChatFont{FontArial, FontMukta}

// String returns the font's name as the game shows it.
func (f ChatFont) String() string {
	if f == FontMukta {
		return "Mukta"
	}
	return "Arial"
}

// ParseChatFont returns the font named name (case-insensitive); unknown names are Arial,
// the game's default.
func ParseChatFont(name string) ChatFont {
	if strings.EqualFold(name, FontMukta.String()) {
		return FontMukta
	}
	return FontArial
}

// The chat scales a screenshot can be drawn at, in percent, as the game's Chat scale
// setting offers them.
const (
	MinChatScale     = 50
	MaxChatScale     = 150
	DefaultChatScale = 100
	ChatScaleStep    = 5
)

// ClampChatScale returns scale within MinChatScale..MaxChatScale, and DefaultChatScale for
// zero.
func ClampChatScale(scale int) int {
	if scale == 0 {
		return DefaultChatScale
	}
	return max(MinChatScale, min(scale, MaxChatScale))
}

// ChatScaleForFontSize converts a font size in pixels, how screenshot looks were chosen
// before chat scales, to the nearest chat scale.
func ChatScaleForFontSize(sizePx int) int {
	scale := float64(sizePx) / chatBaseFontPx * 100
	return ClampChatScale(int(math.Round(scale/ChatScaleStep)) * ChatScaleStep)
}

// Look is how the game's chat is set up: the two settings that change how a line looks.
type Look struct {
	Font      ChatFont
	ChatScale int // percent; zero: DefaultChatScale
}

// String describes the look as the game's settings do: "Arial 60%".
func (l Look) String() string {
	return fmt.Sprintf("%s %d%%", l.Font, ClampChatScale(l.ChatScale))
}

// The in-game chat, from GTA World's stylesheet (FiveM's browser, Chromium) at
// 1920×1080, the common game resolution its vh and vw sizes are taken at:
//
//	.chat           { font-size: 1.55vh; font-weight: 700; letter-spacing: 0.4px }
//	.chat__messages { width: 40vw; padding: 0 0.5vh; text-shadow: 0 0 0.18vh #000 ×3 }
//	li              { margin-left: 10px }
//
// Messages are drawn at the chat scale times the base size, and each line is that size
// plus a fifth of the base size tall.
const (
	chatBaseFontPx      = 16.74 // 1.55vh
	chatLineGapPx       = 3.348 // a fifth of the base size
	chatLetterSpacingPx = 0.4
	chatTextWidthPx     = 758 // 40vw, less the line's 10px margin
	// The in-game chat's first line starts at x 18.359375 (0.5vh padding plus the 10px
	// margin) and y 8.296875 (0.77vh). Only the fractions matter here: they decide how the
	// text sits between pixels, and so how its edges are smoothed.
	chatTextLeftFraction = 0.359375
	chatTextTopFraction  = 0.296875
)

// chatGeometry is the measures of one look, as Chromium lays them out.
type chatGeometry struct {
	look       Look
	fontPx     float64 // the CSS font size
	drawPx     float64 // the size glyphs are drawn and measured at (see drawSize)
	lineHeight float64 // in layout units (see layoutUnit)
}

func geometryFor(look Look) chatGeometry {
	look.ChatScale = ClampChatScale(look.ChatScale)
	fontPx := float64(float32(chatBaseFontPx * float64(look.ChatScale) / 100))
	return chatGeometry{
		look:       look,
		fontPx:     fontPx,
		drawPx:     drawSize(fontPx),
		lineHeight: layoutUnit(fontPx + chatLineGapPx),
	}
}

// drawSize is the font size Chromium draws and measures text at: its font cache keys
// sizes by hundredths of a pixel, truncated in 32-bit float math, so 10.044 px becomes
// 10.04.
func drawSize(fontPx float64) float64 {
	return float64(uint32(float32(fontPx)*100)) / 100
}

// layoutUnit rounds v to Chromium's layout precision, 1/64 pixel.
func layoutUnit(v float64) float64 {
	return math.Round(v*64) / 64
}

// baselineOffset is how far below a row's top the text's baseline is: the font's
// ascent plus half the leading, both in whole pixels, as Chromium places them.
func (g chatGeometry) baselineOffset(ascent, descent float64) float64 {
	roundedAscent, roundedDescent := math.Round(ascent), math.Round(descent)
	halfLeading := math.Floor((g.lineHeight - roundedAscent - roundedDescent) / 2)
	return roundedAscent + halfLeading
}
