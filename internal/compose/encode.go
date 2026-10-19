package compose

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
)

// FileType is the kind of file an export writes.
type FileType string

const (
	FileTypePNG  FileType = "png"
	FileTypeJPEG FileType = "jpeg"
)

// Quality limits, in percent.
const (
	MinQuality         = 10
	MaxQuality         = 100
	DefaultJPEGQuality = 92
	DefaultPNGQuality  = 80 // used when a PNG is saved with reduced colors
)

// Format is how an export is encoded. PNG is lossless unless IsLossless is false, then
// its colors are reduced (Quality picks how many). JPEG is always lossy: the standard
// library has no lossless JPEG, so Quality 100 is the best it gets.
type Format struct {
	Type       FileType
	Quality    int  // MinQuality..MaxQuality
	IsLossless bool // PNG only
}

// Normalized returns f with an unknown type replaced by PNG and the quality in range.
func (f Format) Normalized() Format {
	if f.Type != FileTypeJPEG {
		f.Type = FileTypePNG
	}
	if f.Quality == 0 {
		f.Quality = DefaultJPEGQuality
	}
	f.Quality = clampInt(f.Quality, MinQuality, MaxQuality)
	return f
}

// Extension is the file name extension for f, with the dot.
func (f Format) Extension() string {
	if f.Type == FileTypeJPEG {
		return ".jpg"
	}
	return ".png"
}

// Describe names f for people: "PNG · Lossless", "PNG · 80% colors", "JPEG · 92%".
func (f Format) Describe() string {
	switch {
	case f.Type == FileTypeJPEG:
		return fmt.Sprintf("JPEG · %d%%", f.Quality)
	case f.IsLossless:
		return "PNG · Lossless"
	default:
		return fmt.Sprintf("PNG · %d%% colors", f.Quality)
	}
}

// FileTypeOf returns the file type a path's extension names, if any.
func FileTypeOf(path string) (FileType, bool) {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return FileTypePNG, true
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return FileTypeJPEG, true
	}
	return "", false
}

// Encode writes img in format. JPEG has no transparency, so it is flattened over black
// (the editor tells the user before export).
func Encode(w io.Writer, img *image.RGBA, format Format) error {
	format = format.Normalized()
	switch {
	case format.Type == FileTypeJPEG:
		return jpeg.Encode(w, flattenOnBlack(img), &jpeg.Options{Quality: format.Quality})
	case format.IsLossless:
		return png.Encode(w, img)
	default:
		return png.Encode(w, Quantize(img, paletteSize(format.Quality), format.Quality >= ditherFromQuality))
	}
}

// flattenOnBlack draws img over opaque black, unless it is already opaque.
func flattenOnBlack(img *image.RGBA) image.Image {
	if img.Opaque() {
		return img
	}
	flat := image.NewRGBA(img.Bounds())
	draw.Draw(flat, flat.Bounds(), image.NewUniform(color.Black), image.Point{}, draw.Src)
	draw.Draw(flat, flat.Bounds(), img, img.Bounds().Min, draw.Over)
	return flat
}

// ditherFromQuality is the lowest quality that dithers reduced-color PNGs: below it flat
// areas compress better without the noise.
const ditherFromQuality = 50

// paletteSize maps a quality percent to a palette of 16..256 colors.
func paletteSize(quality int) int {
	return clampInt((256*quality+45)/90, 16, 256)
}
