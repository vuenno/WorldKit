package ui

import (
	"fmt"
	"io/fs"

	"gioui.org/font"
	"gioui.org/font/opentype"
	"gioui.org/text"

	"world-kit/assets"
)

// Typefaces, as named inside the embedded font files.
const (
	fontUI   font.Typeface = "Geist"      // every label, title and chat line
	fontMono font.Typeface = "Geist Mono" // times, counts, sizes and paths
)

// newShaper returns a text shaper that knows only the embedded fonts, so text looks the same
// on every PC regardless of what is installed.
func newShaper() *text.Shaper {
	faces, err := loadFontFaces()
	if err != nil {
		panic(err) // the fonts are embedded at build time, so this is a programming error
	}
	return text.NewShaper(text.NoSystemFonts(), text.WithCollection(faces))
}

func loadFontFaces() ([]font.FontFace, error) {
	paths, err := fs.Glob(assets.Fonts, "fonts/*.ttf")
	if err != nil {
		return nil, fmt.Errorf("list embedded fonts: %w", err)
	}
	var faces []font.FontFace
	for _, path := range paths {
		data, err := fs.ReadFile(assets.Fonts, path)
		if err != nil {
			return nil, fmt.Errorf("read embedded font %q: %w", path, err)
		}
		parsed, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, fmt.Errorf("parse embedded font %q: %w", path, err)
		}
		faces = append(faces, parsed...)
	}
	return faces, nil
}
