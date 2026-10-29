package directwrite

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/image/font/sfnt"
)

func arialBold(t *testing.T) (path string, glyphZ uint16) {
	t.Helper()
	path = filepath.Join(os.Getenv("WINDIR"), "Fonts", "arialbd.ttf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("no Arial Bold: %v", err)
	}
	parsed, err := sfnt.Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	glyph, err := parsed.GlyphIndex(&sfnt.Buffer{}, 'Z')
	if err != nil {
		t.Fatal(err)
	}
	return path, uint16(glyph)
}

// TestGlyphMaskMatchesChromium checks the top of an Arial Bold "Z" at 10.04 px, a quarter
// pixel in: Chromium draws it with these coverages (before its gamma).
func TestGlyphMaskMatchesChromium(t *testing.T) {
	path, glyph := arialBold(t)
	font, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer font.Close()
	mask, err := font.GlyphMask(glyph, 10.04, 0.25)
	if err != nil {
		t.Fatalf("GlyphMask: %v", err)
	}
	want := []uint8{57, 241, 255, 255, 255, 241, 57}
	if mask.Rect.Dx() != len(want) || mask.Rect.Dy() != 7 {
		t.Fatalf("mask bounds %v, want 7×7", mask.Rect)
	}
	top := mask.Pix[:mask.Rect.Dx()]
	for i := range want {
		if diff := int(top[i]) - int(want[i]); diff < -3 || diff > 3 {
			t.Errorf("top row = %v, want %v", top, want)
			break
		}
	}
}

func TestGlyphMaskOfASpaceIsEmpty(t *testing.T) {
	path, _ := arialBold(t)
	font, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer font.Close()
	mask, err := font.GlyphMask(3, 12, 0) // glyph 3 is the space in Arial
	if err != nil {
		t.Fatalf("GlyphMask: %v", err)
	}
	if !mask.Rect.Empty() {
		t.Errorf("space mask bounds %v, want empty", mask.Rect)
	}
}

func TestOpenMissingFont(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing.ttf")); err == nil {
		t.Error("Open(missing) succeeded")
	}
}
