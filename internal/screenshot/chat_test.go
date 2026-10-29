package screenshot

import (
	"bytes"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
)

func TestClampChatScale(t *testing.T) {
	tests := []struct{ scale, want int }{
		{0, DefaultChatScale},
		{MinChatScale, MinChatScale},
		{75, 75},
		{MaxChatScale, MaxChatScale},
		{10, MinChatScale},
		{-5, MinChatScale},
		{400, MaxChatScale},
	}
	for _, test := range tests {
		if got := ClampChatScale(test.scale); got != test.want {
			t.Errorf("ClampChatScale(%d) = %d, want %d", test.scale, got, test.want)
		}
	}
}

func TestChatScaleForFontSize(t *testing.T) {
	tests := []struct{ sizePx, want int }{
		{10, 60},
		{14, 85},
		{17, 100},
		{24, 145},
	}
	for _, test := range tests {
		if got := ChatScaleForFontSize(test.sizePx); got != test.want {
			t.Errorf("ChatScaleForFontSize(%d) = %d, want %d", test.sizePx, got, test.want)
		}
	}
}

func TestParseChatFont(t *testing.T) {
	for _, chatFont := range ChatFonts {
		if got := ParseChatFont(strings.ToLower(chatFont.String())); got != chatFont {
			t.Errorf("ParseChatFont(%q) = %v", chatFont.String(), got)
		}
	}
	if got := ParseChatFont("Comic Sans"); got != FontArial {
		t.Errorf("unknown font = %v, want Arial", got)
	}
}

// TestGeometryMatchesChromium checks the measures against what Chromium lays the game's
// chat out with (read from its layout at 1920×1080).
func TestGeometryMatchesChromium(t *testing.T) {
	tests := []struct {
		look                    Look
		drawPx, lineHeight      float64
		ascent, descent, offset float64
	}{
		{Look{FontArial, 60}, 10.04, 13.390625, 9.09, 2.13, 10},
		{Look{FontMukta, 75}, 12.55, 15.90625, 14.18, 6.68, 11},
		{Look{FontArial, 100}, 16.74, 20.09375, 15.15, 3.55, 15},
		{Look{FontMukta, 150}, 25.11, 28.453125, 28.37, 13.36, 21},
	}
	for _, test := range tests {
		g := geometryFor(test.look)
		if g.drawPx != test.drawPx || g.lineHeight != test.lineHeight {
			t.Errorf("%v: draw size %v, line height %v; want %v, %v", test.look, g.drawPx, g.lineHeight, test.drawPx, test.lineHeight)
		}
		if got := g.baselineOffset(test.ascent, test.descent); got != test.offset {
			t.Errorf("%v: baseline offset %v, want %v", test.look, got, test.offset)
		}
	}
}

func TestFacesHaveTheFontsMetrics(t *testing.T) {
	ascent, descent := facesFor(FontMukta, 12.55).regular.verticalMetrics()
	if math.Abs(ascent-14.18) > 0.01 || math.Abs(descent-6.68) > 0.01 {
		t.Errorf("Mukta at 12.55 px: ascent %.3f, descent %.3f; want 14.18, 6.68", ascent, descent)
	}
}

// TestLuminanceBucket checks colors land in the gamma buckets Chromium put them in.
func TestLuminanceBucket(t *testing.T) {
	tests := []struct {
		rgb  uint32
		want int
	}{
		{0x000000, 0}, {0xFFFFFF, 7}, {0x3F3F3F, 1}, {0x404040, 2}, {0xE0E0E0, 7},
		{0xFF0000, 1}, {0x00FF00, 5}, {0x0000FF, 0}, {0xFF00FF, 2}, {0x00FFFF, 6},
		{0xC2A3DA, 6}, {0xEDA841, 5}, {0x3896F3, 4}, {0x939799, 4}, {0xFFA0A0, 6}, {0xD0B0B0, 5},
	}
	for _, test := range tests {
		c := color.NRGBA{uint8(test.rgb >> 16), uint8(test.rgb >> 8), uint8(test.rgb), 0xFF}
		if got := luminanceBucket(c); got != test.want {
			t.Errorf("luminanceBucket(%06X) = %d, want %d", test.rgb, got, test.want)
		}
	}
}

// TestCoverageTable checks the curve against coverage Chromium drew white and black text
// with.
func TestCoverageTable(t *testing.T) {
	tests := []struct {
		c        color.NRGBA
		raw      int
		chromium uint8
	}{
		{color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}, 57, 130},
		{color.NRGBA{0xFF, 0xFF, 0xFF, 0xFF}, 128, 187},
		{color.NRGBA{A: 0xFF}, 57, 51},
		{color.NRGBA{A: 0xFF}, 128, 119},
	}
	for _, test := range tests {
		got := coverageTable(test.c)[test.raw]
		if diff := int(got) - int(test.chromium); diff < -2 || diff > 2 {
			t.Errorf("%v at %d: %d, want %d", test.c, test.raw, got, test.chromium)
		}
	}
	for _, c := range []color.NRGBA{{A: 0xFF}, {0xFF, 0xFF, 0xFF, 0xFF}} {
		if table := coverageTable(c); table[0] != 0 || table[255] != 255 {
			t.Errorf("%v: ends %d, %d; want 0 and 255", c, table[0], table[255])
		}
	}
}

func TestFakeBoldThickensGlyphs(t *testing.T) {
	mukta := facesFor(FontMukta, 12.55)
	face, glyph, ok := mukta.regular.lookup('H')
	if !ok {
		t.Fatal("Mukta has no H")
	}
	plain := outlineRasterizer{font: face.outline, sizePx: 12.55}
	bold := plain
	bold.emboldenPx = fakeBoldPx(12.55)
	if got := ink(t, plain, glyph); ink(t, bold, glyph) <= got*11/10 {
		t.Errorf("bold H has %d ink, want clearly more than plain's %d", ink(t, bold, glyph), got)
	}
	if got := fakeBoldPx(12.55); math.Abs(got-0.5057) > 0.001 {
		t.Errorf("fakeBoldPx(12.55) = %.4f, want 0.5057 (Skia)", got)
	}
}

func ink(t *testing.T, raster glyphRasterizer, glyph sfnt.GlyphIndex) int {
	t.Helper()
	mask, err := raster.rasterize(glyph, 0)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, v := range mask.Pix {
		total += int(v)
	}
	return total
}

func TestMuktaFallsBackToArial(t *testing.T) {
	face, _, ok := facesFor(FontMukta, 12.55).regular.lookup('Ж') // Cyrillic: not in Mukta
	if !ok || face == facesFor(FontMukta, 12.55).regular {
		t.Errorf("lookup(Ж) = %v, %v; want Arial's face", face, ok)
	}
}

func TestRenderGrowsWithChatScale(t *testing.T) {
	lines := []Line{sayLine("John Doe says: Hello there."), sayLine("Jane Doe says: Hi.")}
	small := render(t, lines, Style{Look: Look{ChatScale: MinChatScale}})
	normal := render(t, lines, Style{})
	large := render(t, lines, Style{Look: Look{ChatScale: MaxChatScale}})
	smallSize, normalSize, largeSize := small.Image.Bounds().Size(), normal.Image.Bounds().Size(), large.Image.Bounds().Size()
	if smallSize.X >= normalSize.X || normalSize.X >= largeSize.X {
		t.Errorf("widths %d, %d, %d; want growing with the scale", smallSize.X, normalSize.X, largeSize.X)
	}
	if smallSize.Y >= normalSize.Y || normalSize.Y >= largeSize.Y {
		t.Errorf("heights %d, %d, %d; want growing with the scale", smallSize.Y, normalSize.Y, largeSize.Y)
	}
}

func TestRenderWrapsAtTheInGameWidth(t *testing.T) {
	// The chat box keeps its width at any scale, so bigger text wraps into more rows.
	lines := []Line{sayLine(longText(300))}
	small := len(render(t, lines, Style{Look: Look{ChatScale: MinChatScale}}).rows)
	large := len(render(t, lines, Style{Look: Look{ChatScale: MaxChatScale}}).rows)
	if small >= large {
		t.Errorf("rows at 50%% = %d, at 150%% = %d; want more rows for bigger text", small, large)
	}
}

func longText(words int) string {
	return "John Doe says:" + strings.Repeat(" word", words)
}

func TestSplitWordsBreaksAfterHyphens(t *testing.T) {
	chars := lineChars(sayLine("get your L-C-C set-up 1-2"))
	var words []string
	for _, word := range splitWords(chars) {
		var text []rune
		for _, i := range word {
			text = append(text, chars[i].R)
		}
		words = append(words, string(text))
	}
	want := []string{"get", " your", " L-", "C-", "C", " set-", "up", " 1-2"}
	if strings.Join(words, "|") != strings.Join(want, "|") {
		t.Errorf("splitWords = %q, want %q", words, want)
	}
}

func TestRowBoxesAtChatScale(t *testing.T) {
	lines := []Line{sayLine("A long first line of text"), sayLine("short"), sayLine("third")}
	picture := render(t, lines, Style{Look: Look{FontMukta, MaxChatScale}})
	if boxes := RowBoxes(picture.Image, picture.grid); len(boxes) != len(lines) {
		t.Errorf("RowBoxes = %d boxes, want %d", len(boxes), len(lines))
	}
}

func TestStoreKeepsLook(t *testing.T) {
	store := NewStore(t.TempDir())
	look := Look{FontMukta, 75}
	shot, err := store.Save([]Line{sayLine("John Doe says: Hi.")}, Style{Look: look})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := store.Load(shot.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := chatGrid(geometryFor(look)); saved.Grid != want {
		t.Errorf("loaded grid = %+v, want %+v", saved.Grid, want)
	}
}

func TestStoreLoadsOlderScreenshots(t *testing.T) {
	picture := render(t, []Line{sayLine("John Doe says: Hi.")}, Style{})
	var plain bytes.Buffer
	if err := png.Encode(&plain, picture.Image); err != nil {
		t.Fatal(err)
	}
	withSize := spliceText(t, plain.Bytes(), fontSizeKeyword, "20")
	tests := []struct {
		name string
		data []byte
		want RowGrid
	}{
		{"before font sizes", plain.Bytes(), legacyGrid(legacyDefaultFontSize)},
		{"with a font size", withSize, legacyGrid(20)},
	}
	dir := t.TempDir()
	for i, test := range tests {
		path := filepath.Join(dir, "2026-10-18_10153"+string(rune('0'+i))+".png")
		if err := os.WriteFile(path, test.data, 0o644); err != nil {
			t.Fatal(err)
		}
		saved, err := NewStore(dir).Load(path)
		if err != nil {
			t.Fatalf("%s: Load: %v", test.name, err)
		}
		if saved.Grid != test.want {
			t.Errorf("%s: grid = %+v, want %+v", test.name, saved.Grid, test.want)
		}
	}
	if got := legacyGrid(20).Height; got != 26 {
		t.Errorf("legacy rows at 20 px are %v tall, want 26 (as drawn then)", got)
	}
}

// spliceText adds a tEXt chunk after a PNG's header, as older versions saved it.
func spliceText(t *testing.T, data []byte, keyword, text string) []byte {
	t.Helper()
	afterHeader := pngSignatureLength + chunkHeaderLength + 13 + chunkCRCLength // IHDR is 13 bytes
	return bytes.Join([][]byte{data[:afterHeader], textChunk(keyword, text), data[afterHeader:]}, nil)
}
