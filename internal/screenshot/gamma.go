package screenshot

import (
	"image/color"
	"math"
	"sync"
)

// Chromium does not draw glyph coverage as it comes from the rasterizer: Skia's mask
// gamma bends it by the text color's luminance, so light text on (assumed) dark looks as
// heavy as dark text on light. On Windows it uses the system's text gamma (2.2) and
// enhanced contrast (1.0). These match Chromium's output to within a level or two.
const (
	textGamma    = 2.2
	textContrast = 1.0
	// luminanceBits is how finely Skia sorts colors into luminance buckets: each bucket
	// shares one table.
	luminanceBits = 3
)

// coverageTables holds one table per luminance bucket, mapping a glyph's coverage to the
// alpha Chromium draws it with.
var coverageTables = sync.OnceValue(func() [1 << luminanceBits][256]uint8 {
	var tables [1 << luminanceBits][256]uint8
	for bucket := range tables {
		tables[bucket] = correctingTable(expandBucket(bucket))
	}
	return tables
})

// coverageTable returns the table for text drawn in c.
func coverageTable(c color.NRGBA) *[256]uint8 {
	tables := coverageTables()
	return &tables[luminanceBucket(c)]
}

// luminanceBucket is Skia's luminance for c, as it computes it for grayscale text: each
// channel is first reduced to luminanceBits bits, then weighted (Skia's fixed-point
// SkComputeLuminance) and reduced again.
func luminanceBucket(c color.NRGBA) int {
	channel := func(v uint8) int { return expandBucket(int(v) >> (8 - luminanceBits)) }
	lum := (channel(c.R)*54 + channel(c.G)*183 + channel(c.B)*19) >> 8
	return lum >> (8 - luminanceBits)
}

// expandBucket spreads a luminanceBits-bit value over 0..255 by repeating its bits.
func expandBucket(bucket int) int {
	return bucket<<5 | bucket<<2 | bucket>>1
}

// correctingTable is Skia's SkTMaskGamma_build_correcting_lut for text of luminance srcI
// (0..255), guessing the background is its opposite.
func correctingTable(srcI int) [256]uint8 {
	toLinear := func(v float64) float64 { return math.Pow(v, textGamma) }
	fromLinear := func(v float64) float64 { return math.Pow(v, 1/textGamma) }
	src := float64(srcI) / 255
	dst := 1 - src
	linearSrc, linearDst := toLinear(src), toLinear(dst)
	contrast := textContrast * linearDst // contrast fades out as the text gets lighter
	var table [256]uint8
	for i := range table {
		raw := float64(i) / 255
		coverage := raw + (1-raw)*contrast*raw
		if math.Abs(src-dst) < 1.0/256 {
			table[i] = uint8(math.Round(255 * coverage))
			continue
		}
		out := fromLinear(linearSrc*coverage + (1-coverage)*linearDst)
		result := (out - dst) / (src - dst) // undo the blend the blit will do
		table[i] = uint8(math.Round(255 * max(0, min(1, result))))
	}
	return table
}
