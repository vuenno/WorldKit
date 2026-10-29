package screenshot

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"world-kit/assets"
	"world-kit/internal/directwrite"
)

// chatFace is one face of a chat font, regular or italic, at the size a look draws it.
// Faces are cached for the process's life and safe for concurrent use.
type chatFace struct {
	outline  *sfnt.Font // glyph lookup, advances, kerning and vertical metrics
	sizePx   float64
	raster   glyphRasterizer
	fallback *chatFace // draws characters the font lacks, as the game's sans-serif would; nil for none
}

// faceSet is a look's regular and italic faces.
type faceSet struct {
	regular, italic *chatFace
}

func (s faceSet) of(isItalic bool) *chatFace {
	if isItalic {
		return s.italic
	}
	return s.regular
}

// lookup returns the face that has r, starting with f and then its fallbacks, and r's
// glyph in it. Without one, ok is false.
func (f *chatFace) lookup(r rune) (face *chatFace, glyph sfnt.GlyphIndex, ok bool) {
	var buffer sfnt.Buffer
	for face = f; face != nil; face = face.fallback {
		glyph, err := face.outline.GlyphIndex(&buffer, r)
		if err == nil && glyph != 0 {
			return face, glyph, true
		}
	}
	return nil, 0, false
}

// advance is the glyph's width in pixels, as Chromium measures it: unhinted.
func (f *chatFace) advance(glyph sfnt.GlyphIndex) float64 {
	var buffer sfnt.Buffer
	units, err := f.outline.GlyphAdvance(&buffer, glyph, f.unitsPerEm(), font.HintingNone)
	if err != nil {
		return 0 // a glyph index from lookup is always in range
	}
	return f.toPixels(units)
}

// kern is the adjustment between two glyphs of this face, in pixels.
func (f *chatFace) kern(left, right sfnt.GlyphIndex) float64 {
	var buffer sfnt.Buffer
	units, err := f.outline.Kern(&buffer, left, right, f.unitsPerEm(), font.HintingNone)
	if err != nil {
		return 0 // most pairs have no kerning
	}
	return f.toPixels(units)
}

// verticalMetrics are the font's ascent and descent in pixels, unrounded.
func (f *chatFace) verticalMetrics() (ascent, descent float64) {
	var buffer sfnt.Buffer
	metrics, err := f.outline.Metrics(&buffer, f.unitsPerEm(), font.HintingNone)
	if err != nil {
		return f.sizePx, 0 // a parsed font always has metrics
	}
	return f.toPixels(metrics.Ascent), f.toPixels(metrics.Descent)
}

// unitsPerEm, passed as sfnt's pixels per em, makes sfnt answer in design units.
func (f *chatFace) unitsPerEm() fixed.Int26_6 {
	return fixed.Int26_6(f.outline.UnitsPerEm())
}

func (f *chatFace) toPixels(designUnits fixed.Int26_6) float64 {
	return float64(designUnits) * f.sizePx / float64(f.outline.UnitsPerEm())
}

type faceSetKey struct {
	font   ChatFont
	sizePx float64
}

var faceSets = struct {
	sync.Mutex
	byKey map[faceSetKey]faceSet
}{byKey: map[faceSetKey]faceSet{}}

// facesFor returns the faces a look's text is drawn with, at sizePx.
func facesFor(chatFont ChatFont, sizePx float64) faceSet {
	key := faceSetKey{chatFont, sizePx}
	faceSets.Lock()
	defer faceSets.Unlock()
	if set, ok := faceSets.byKey[key]; ok {
		return set
	}
	set := arialFaces(sizePx)
	if chatFont == FontMukta {
		set = muktaFaces(sizePx, set)
	}
	faceSets.byKey[key] = set
	return set
}

// arialFaces are Arial Bold and Arial Bold Italic, drawn with DirectWrite as the game
// draws them. Their license doesn't allow bundling them, but every Windows install has
// them, so they are read from the Windows fonts folder. If that fails, the bundled Go
// fonts keep screenshots working, drawn from their outlines.
func arialFaces(sizePx float64) faceSet {
	return faceSet{
		regular: windowsFace("arialbd.ttf", gobold.TTF, sizePx),
		italic:  windowsFace("arialbi.ttf", gobolditalic.TTF, sizePx),
	}
}

func windowsFace(fileName string, fallback []byte, sizePx float64) *chatFace {
	path := filepath.Join(windowsDir(), "Fonts", fileName)
	outline, err := parsedFont(path)
	if err != nil {
		slog.Warn("load screenshot font, using a Go font instead", "path", path, "error", err)
		goFont := mustParse(fallback)
		return &chatFace{outline: goFont, sizePx: sizePx, raster: outlineRasterizer{font: goFont, sizePx: sizePx}}
	}
	face := &chatFace{outline: outline, sizePx: sizePx, raster: outlineRasterizer{font: outline, sizePx: sizePx}}
	if drawn, err := directWriteFont(path); err == nil {
		face.raster = directWriteRasterizer{font: drawn, sizePx: float32(sizePx)}
	} else {
		slog.Warn("open screenshot font in DirectWrite, drawing outlines instead", "path", path, "error", err)
	}
	return face
}

// muktaFaces are the bundled Mukta Regular with the synthetic bold (and italic) the game
// draws it with. Characters Mukta lacks fall back to Arial, the game's sans-serif.
func muktaFaces(sizePx float64, arial faceSet) faceSet {
	mukta := mustParse(assets.MuktaRegular)
	bold := outlineRasterizer{font: mukta, sizePx: sizePx, emboldenPx: fakeBoldPx(sizePx)}
	boldItalic := bold
	boldItalic.skew = fakeItalicSkew
	return faceSet{
		regular: &chatFace{outline: mukta, sizePx: sizePx, raster: bold, fallback: arial.regular},
		italic:  &chatFace{outline: mukta, sizePx: sizePx, raster: boldItalic, fallback: arial.italic},
	}
}

// parsedFonts caches fonts by path: the edit dialog renders a live preview on every
// keystroke, and Arial is ~1 MB to read and parse.
var parsedFonts = struct {
	sync.Mutex
	byPath map[string]*sfnt.Font
}{byPath: map[string]*sfnt.Font{}}

func parsedFont(path string) (*sfnt.Font, error) {
	parsedFonts.Lock()
	defer parsedFonts.Unlock()
	if parsed, ok := parsedFonts.byPath[path]; ok {
		return parsed, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	parsed, err := sfnt.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	parsedFonts.byPath[path] = parsed
	return parsed, nil
}

// directWriteFonts caches fonts opened in DirectWrite by path; they stay open for the
// process's life.
var directWriteFonts = struct {
	sync.Mutex
	byPath map[string]*directwrite.Font
}{byPath: map[string]*directwrite.Font{}}

func directWriteFont(path string) (*directwrite.Font, error) {
	directWriteFonts.Lock()
	defer directWriteFonts.Unlock()
	if opened, ok := directWriteFonts.byPath[path]; ok {
		return opened, nil
	}
	opened, err := directwrite.Open(path)
	if err != nil {
		return nil, err
	}
	directWriteFonts.byPath[path] = opened
	return opened, nil
}

// bundledFonts caches the parsed bundled fonts (Mukta, the Go fallbacks) by their data.
var bundledFonts sync.Map // *byte (first byte of the data) → *sfnt.Font

// mustParse parses a font bundled into the binary, once.
func mustParse(data []byte) *sfnt.Font {
	if parsed, ok := bundledFonts.Load(&data[0]); ok {
		return parsed.(*sfnt.Font)
	}
	parsed, err := sfnt.Parse(data)
	if err != nil {
		panic(fmt.Sprintf("parse bundled font: %v", err)) // bundled fonts are always valid
	}
	actual, _ := bundledFonts.LoadOrStore(&data[0], parsed)
	return actual.(*sfnt.Font)
}

func windowsDir() string {
	if dir := os.Getenv("WINDIR"); dir != "" {
		return dir
	}
	return `C:\Windows`
}
