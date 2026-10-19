package compose

import (
	"errors"
	"fmt"
	"image"
	"image/draw"
	"os"

	// Formats the editor opens. The x/image ones come from a module WorldKit already uses.
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/webp"

	xdraw "golang.org/x/image/draw"
)

// ErrTooLarge is returned for images wider or taller than MaxSourceSide.
var ErrTooLarge = errors.New("image is too large")

// ReadSize returns an image file's pixel size without decoding its pixels, refusing files
// over MaxSourceSide so a huge image is never loaded into memory.
func ReadSize(path string) (image.Point, error) {
	file, err := os.Open(path)
	if err != nil {
		return image.Point{}, fmt.Errorf("open image %q: %w", path, err)
	}
	defer file.Close()
	config, _, err := image.DecodeConfig(file)
	if err != nil {
		return image.Point{}, fmt.Errorf("read image %q: %w", path, err)
	}
	size := image.Pt(config.Width, config.Height)
	if size.X > MaxSourceSide || size.Y > MaxSourceSide {
		return size, fmt.Errorf("image %q is %d × %d px: %w", path, size.X, size.Y, ErrTooLarge)
	}
	return size, nil
}

// Decode reads a PNG, JPEG, BMP or WebP file, after checking its size with ReadSize.
func Decode(path string) (image.Image, error) {
	if _, err := ReadSize(path); err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open image %q: %w", path, err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("decode image %q: %w", path, err)
	}
	return img, nil
}

// Downscale returns img as premultiplied RGBA (what Gio uploads without a copy), shrunk
// with Catmull-Rom so its longer side is at most maxSide; smaller images keep their size.
func Downscale(img image.Image, maxSide int) *image.RGBA {
	size := img.Bounds().Size()
	if size.X <= maxSide && size.Y <= maxSide {
		out := image.NewRGBA(image.Rectangle{Max: size})
		draw.Draw(out, out.Bounds(), img, img.Bounds().Min, draw.Src)
		return out
	}
	shown := FitInside(size, Vec{float64(maxSide), float64(maxSide)})
	out := image.NewRGBA(image.Rect(0, 0, max(1, int(shown.X+0.5)), max(1, int(shown.Y+0.5))))
	xdraw.CatmullRom.Scale(out, out.Bounds(), img, img.Bounds(), xdraw.Src, nil)
	return out
}
