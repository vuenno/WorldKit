package compose

import (
	"image"
	"math"
)

// Gio's preview samples images from sRGB textures and blends in linear light; the export
// does the same, so a 50 % white layer over black is as bright in the file as on screen.
// Like the GPU, it converts each premultiplied channel straight to linear (no
// unpremultiply), which also matches the preview on soft edges.

// linearMax is full intensity in the 16-bit linear canvas.
const linearMax = 0xFFFF

// toLinearBits indexes linearToSRGB: linear values are looked up by their top 12 bits.
const toLinearBits = 12

var (
	srgbToLinear = buildSRGBToLinear()
	linearToSRGB = buildLinearToSRGB()
)

func buildSRGBToLinear() [256]uint16 {
	var table [256]uint16
	for i := range table {
		table[i] = uint16(math.Round(decodeSRGB(float64(i)/255) * linearMax))
	}
	return table
}

func buildLinearToSRGB() [1 << toLinearBits]uint8 {
	var table [1 << toLinearBits]uint8
	last := float64(len(table) - 1)
	for i := range table {
		table[i] = uint8(math.Round(encodeSRGB(float64(i)/last) * 255))
	}
	return table
}

func decodeSRGB(v float64) float64 {
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func encodeSRGB(v float64) float64 {
	if v <= 0.0031308 {
		return v * 12.92
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// linearCanvas is premultiplied RGBA in linear light, 16 bits per channel.
type linearCanvas struct {
	size image.Point
	pix  []uint16 // 4 per pixel, rows top to bottom
}

func newLinearCanvas(size image.Point, background Background) *linearCanvas {
	canvas := &linearCanvas{size: size, pix: make([]uint16, 4*size.X*size.Y)}
	var fill [4]uint16
	switch background {
	case BackgroundBlack:
		fill = [4]uint16{0, 0, 0, linearMax}
	case BackgroundWhite:
		fill = [4]uint16{linearMax, linearMax, linearMax, linearMax}
	default:
		return canvas // transparent: already zero
	}
	for i := 0; i < len(canvas.pix); i += 4 {
		copy(canvas.pix[i:i+4], fill[:])
	}
	return canvas
}

// composite blends layer (premultiplied sRGB, placed in canvas coordinates) over the
// canvas at opacity.
func (c *linearCanvas) composite(layer *image.RGBA, opacity float64) {
	area := layer.Bounds().Intersect(image.Rectangle{Max: c.size})
	strength := uint32(math.Round(math.Min(opacity, 1) * linearMax))
	inParallelBands(area, func(band image.Rectangle) {
		for y := band.Min.Y; y < band.Max.Y; y++ {
			src := layer.Pix[layer.PixOffset(band.Min.X, y):]
			dst := c.pix[4*(y*c.size.X+band.Min.X):]
			for x := 0; x < band.Dx(); x++ {
				blendOver(dst[4*x:4*x+4], src[4*x:4*x+4], strength)
			}
		}
	})
}

// blendOver puts one premultiplied sRGB pixel, scaled by strength (0..linearMax), over a
// linear one.
func blendOver(dst []uint16, src []uint8, strength uint32) {
	alpha := uint32(src[3]) * 0x101 * strength / linearMax
	if alpha == 0 {
		return
	}
	keep := linearMax - alpha
	for i := range 3 {
		color := uint32(srgbToLinear[src[i]]) * strength / linearMax
		dst[i] = uint16(color + uint32(dst[i])*keep/linearMax)
	}
	dst[3] = uint16(alpha + uint32(dst[3])*keep/linearMax)
}

// toRGBA converts the canvas back to premultiplied 8-bit sRGB for encoding.
func (c *linearCanvas) toRGBA() *image.RGBA {
	out := image.NewRGBA(image.Rectangle{Max: c.size})
	shift := 16 - toLinearBits
	inParallelBands(out.Bounds(), func(band image.Rectangle) {
		start, end := 4*band.Min.Y*c.size.X, 4*band.Max.Y*c.size.X
		for i := start; i < end; i += 4 {
			alpha := uint8(c.pix[i+3] >> 8)
			for ch := range 3 {
				// Premultiplied: a channel never exceeds alpha.
				out.Pix[i+ch] = min(linearToSRGB[c.pix[i+ch]>>shift], alpha)
			}
			out.Pix[i+3] = alpha
		}
	})
	return out
}
