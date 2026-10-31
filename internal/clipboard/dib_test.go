package clipboard

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"
)

func TestEncodeDIBV5(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})   // top left
	img.SetNRGBA(1, 1, color.NRGBA{R: 200, G: 100, B: 50, A: 128}) // bottom right, half see-through
	data := encodeDIBV5(img)

	le := binary.LittleEndian
	if got, want := len(data), dibHeaderSize+2*2*4; got != want {
		t.Fatalf("len = %d, want %d", got, want)
	}
	header := []struct {
		name   string
		offset int
		want   uint32
	}{
		{"size", 0, dibHeaderSize},
		{"width", 4, 2},
		{"height (bottom-up)", 8, 2},
		{"compression", 16, biBitfields},
		{"alpha mask", 52, maskAlpha},
		{"color space", 56, lcsSRGB},
	}
	for _, field := range header {
		if got := le.Uint32(data[field.offset:]); got != field.want {
			t.Errorf("%s = %#x, want %#x", field.name, got, field.want)
		}
	}
	pixels := data[dibHeaderSize:]
	tests := []struct {
		name   string
		offset int
		want   [4]byte
	}{
		{"top left, stored in the last row", 8, [4]byte{30, 20, 10, 255}},
		{"bottom right, stored in the first row, alpha straight", 4, [4]byte{50, 100, 200, 128}},
		{"transparent stays transparent", 0, [4]byte{0, 0, 0, 0}},
	}
	for _, test := range tests {
		if got := [4]byte(pixels[test.offset : test.offset+4]); got != test.want {
			t.Errorf("%s: BGRA = %v, want %v", test.name, got, test.want)
		}
	}
}
