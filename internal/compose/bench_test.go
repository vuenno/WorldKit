package compose

import (
	"context"
	"image"
	"image/color"
	"io"
	"math"
	"testing"
)

// benchmarkComposition is a 4K canvas with a full-frame photo and a rotated, faded one.
func benchmarkComposition() (Composition, Opener) {
	size := image.Pt(3840, 2160)
	photo := image.NewRGBA(image.Rectangle{Max: size})
	for i := 0; i < len(photo.Pix); i += 4 {
		photo.Pix[i], photo.Pix[i+1], photo.Pix[i+2], photo.Pix[i+3] = uint8(i), uint8(i>>8), uint8(i>>16), 255
	}
	open := func(string) (image.Image, error) { return photo, nil }
	full := Layer{Natural: size, Center: Vec{1920, 1080}, Size: Vec{3840, 2160}, Opacity: 1}
	turned := Layer{Natural: size, Center: Vec{1500, 900}, Size: Vec{1920, 1080}, Rotation: math.Pi / 9, Opacity: 0.6}
	return Composition{Size: size, Layers: []Layer{full, turned}}, open
}

func BenchmarkRender4K(b *testing.B) {
	comp, open := benchmarkComposition()
	for b.Loop() {
		if _, err := Render(context.Background(), comp, open); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncode4K(b *testing.B) {
	img := image.NewRGBA(image.Rect(0, 0, 3840, 2160))
	for y := range 2160 {
		for x := range 3840 {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}
	for _, format := range []Format{{Type: FileTypePNG, IsLossless: true}, {Type: FileTypePNG, Quality: 80}, {Type: FileTypeJPEG, Quality: 92}} {
		b.Run(format.Describe(), func(b *testing.B) {
			for b.Loop() {
				if err := Encode(io.Discard, img, format); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
