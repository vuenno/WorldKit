package screenshot

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/opentype"
)

// faces are the two font faces a screenshot is drawn with.
type faces struct {
	regular font.Face
	italic  font.Face
}

// newFaces returns Arial Bold and Arial Bold Italic at the screenshot size. A face is not
// safe for concurrent use, so each render makes its own.
func newFaces() (*faces, error) {
	regular, regularErr := newFace(loadFont("arialbd.ttf", gobold.TTF))
	italic, italicErr := newFace(loadFont("arialbi.ttf", gobolditalic.TTF))
	if err := errors.Join(regularErr, italicErr); err != nil {
		return nil, fmt.Errorf("create screenshot font faces: %w", err)
	}
	return &faces{regular: regular, italic: italic}, nil
}

func newFace(parsed *opentype.Font) (font.Face, error) {
	return opentype.NewFace(parsed, &opentype.FaceOptions{Size: fontSizePx, DPI: 72, Hinting: font.HintingFull})
}

func (f *faces) of(isItalic bool) font.Face {
	if isItalic {
		return f.italic
	}
	return f.regular
}

// Close releases both faces.
func (f *faces) Close() {
	// opentype faces hold no OS resources; Close never fails.
	_ = f.regular.Close()
	_ = f.italic.Close()
}

// parsedFonts caches fonts by path: the edit dialog renders a live preview on every
// keystroke, and Arial is ~1 MB to read and parse.
var parsedFonts = struct {
	sync.Mutex
	byPath map[string]*opentype.Font
}{byPath: map[string]*opentype.Font{}}

// loadFont returns the Windows font fileName (Arial Bold or Bold Italic, the in-game chat
// look). Its license doesn't allow bundling it, but every Windows install has it, so it is
// read from the Windows fonts folder. If that fails, the bundled fallback (a Go font) keeps
// Extract working.
func loadFont(fileName string, fallback []byte) *opentype.Font {
	path := filepath.Join(windowsDir(), "Fonts", fileName)
	parsedFonts.Lock()
	defer parsedFonts.Unlock()
	if parsed, ok := parsedFonts.byPath[path]; ok {
		return parsed
	}
	parsed, err := parseFontFile(path)
	if err != nil {
		slog.Warn("load screenshot font, using a Go font instead", "path", path, "error", err)
		parsed, err = opentype.Parse(fallback)
		if err != nil {
			panic(fmt.Sprintf("parse bundled Go font: %v", err)) // the bundled fonts are always valid
		}
	}
	parsedFonts.byPath[path] = parsed
	return parsed
}

func parseFontFile(path string) (*opentype.Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read font: %w", err)
	}
	parsed, err := opentype.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("parse font: %w", err)
	}
	return parsed, nil
}

func windowsDir() string {
	if dir := os.Getenv("WINDIR"); dir != "" {
		return dir
	}
	return `C:\Windows`
}
