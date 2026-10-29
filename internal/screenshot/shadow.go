package screenshot

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// The chat's text-shadow, "0 0 0.18vh #000" three times over, as Chromium draws it at
// 1080p: each shadow is the run's glyphs in black, blurred by this kernel (measured from
// Chromium's output; its blur at this small radius is not a plain Gaussian), and the three
// are stacked.
var shadowKernel = [...]float64{0.044, 0.210, 0.492, 0.210, 0.044}

const (
	shadowLayers = 3
	// shadowReachPx is how far the shadow reaches past the glyphs' own coverage.
	shadowReachPx = len(shadowKernel) / 2
)

var shadowColor = image.NewUniform(color.Black)

// placedGlyph is a glyph's coverage mask where it lands on the canvas.
type placedGlyph struct {
	mask   *image.Alpha
	origin image.Point // where the mask's (0, 0), the pen's pixel, lands
}

func (g placedGlyph) bounds() image.Rectangle {
	return g.mask.Rect.Add(g.origin)
}

// drawShadow draws the shadow of a run of glyphs onto canvas.
func drawShadow(canvas *image.RGBA, glyphs []placedGlyph) {
	shadow := shadowMask(glyphs)
	if shadow == nil {
		return
	}
	for range shadowLayers {
		draw.DrawMask(canvas, shadow.Rect, shadowColor, image.Point{}, shadow, shadow.Rect.Min, draw.Over)
	}
}

// shadowMask is one layer of the run's shadow: the glyphs' coverage as black text would
// have it, blurred. nil when the run has no ink.
func shadowMask(glyphs []placedGlyph) *image.Alpha {
	var bounds image.Rectangle
	for _, g := range glyphs {
		bounds = bounds.Union(g.bounds())
	}
	if bounds.Empty() {
		return nil
	}
	bounds = bounds.Inset(-shadowReachPx)
	width, height := bounds.Dx(), bounds.Dy()
	coverage := make([]float64, width*height)
	table := coverageTable(color.NRGBA{A: 0xFF})
	for _, g := range glyphs {
		addCoverage(coverage, bounds, g, table)
	}
	blurred := blur(blur(coverage, width, height, 1, width), height, width, width, 1)
	mask := image.NewAlpha(bounds)
	for i, v := range blurred {
		mask.Pix[i] = uint8(math.Round(255 * max(0, min(1, v))))
	}
	return mask
}

// addCoverage composites a glyph's coverage, through table, over coverage (bounds' pixels).
func addCoverage(coverage []float64, bounds image.Rectangle, g placedGlyph, table *[256]uint8) {
	area := g.bounds()
	for y := area.Min.Y; y < area.Max.Y; y++ {
		for x := area.Min.X; x < area.Max.X; x++ {
			a := float64(table[g.mask.AlphaAt(x-g.origin.X, y-g.origin.Y).A]) / 255
			i := (y-bounds.Min.Y)*bounds.Dx() + x - bounds.Min.X
			coverage[i] += a * (1 - coverage[i])
		}
	}
}

// blur convolves count lines of length samples with shadowKernel: a line's samples are
// step apart and consecutive lines lineStep apart, so one function blurs rows (step 1)
// and columns (step width).
func blur(values []float64, count, length, lineStep, step int) []float64 {
	out := make([]float64, len(values))
	reach := len(shadowKernel) / 2
	for line := range count {
		base := line * lineStep
		for i := range length {
			sum := 0.0
			for k, weight := range shadowKernel {
				if j := i + k - reach; j >= 0 && j < length {
					sum += weight * values[base+j*step]
				}
			}
			out[base+i*step] = sum
		}
	}
	return out
}
