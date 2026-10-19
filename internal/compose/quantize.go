package compose

import (
	"image"
	"image/color"
	"slices"
)

// Reduced-color PNGs: a median-cut palette of up to 256 colors (alpha included) and
// optional Floyd–Steinberg dithering. Like pngquant, this trades exact pixels for files
// about half to a quarter the size. The standard library has no quantizer, and its
// FloydSteinberg searches the whole palette for every pixel (seconds at 4K), so nearest
// colors here are cached per histogram bin.

// Histogram bins: 5 bits for each color channel and 4 for alpha.
const (
	binColorBits = 5
	binAlphaBits = 4
	binCount     = 1 << (3*binColorBits + binAlphaBits)
)

func binOf(r, g, b, a int32) int {
	return int(r>>(8-binColorBits))<<(2*binColorBits+binAlphaBits) |
		int(g>>(8-binColorBits))<<(binColorBits+binAlphaBits) |
		int(b>>(8-binColorBits))<<binAlphaBits |
		int(a>>(8-binAlphaBits))
}

// colorStats is a histogram bin: how many pixels fell in it and their channel sums.
type colorStats struct {
	count uint64
	sum   [4]uint64 // straight R, G, B, A
}

func (s colorStats) mean(channel int) uint64 { return s.sum[channel] / s.count }

// Quantize returns img with at most colors colors, dithered when dither is set.
// Fully transparent pixels get a palette entry of their own (index 0, when there are any),
// so they stay exactly transparent and never pick up dithering noise.
func Quantize(img *image.RGBA, colors int, dither bool) *image.Paletted {
	bins, hasTransparent := histogram(img)
	if hasTransparent {
		colors--
	}
	palette := buildPalette(bins, max(1, colors))
	if hasTransparent {
		palette = append(color.Palette{color.NRGBA{}}, palette...)
	}
	out := image.NewPaletted(img.Bounds(), palette)
	mapPixels(out, img, newNearestCache(palette), dither)
	return out
}

// straightAt returns the unpremultiplied color at offset i of img's pixels.
func straightAt(pix []uint8, i int) [4]int32 {
	a := int32(pix[i+3])
	if a == 0 {
		return [4]int32{}
	}
	unpremultiply := func(c uint8) int32 { return min(255, (int32(c)*255+a/2)/a) }
	return [4]int32{unpremultiply(pix[i]), unpremultiply(pix[i+1]), unpremultiply(pix[i+2]), a}
}

// histogram counts the colors of every pixel that isn't fully transparent, and reports
// whether any pixel is.
func histogram(img *image.RGBA) ([]colorStats, bool) {
	hasTransparent := false
	bins := make([]colorStats, binCount)
	size := img.Bounds().Size()
	for y := range size.Y {
		row := img.PixOffset(img.Bounds().Min.X, img.Bounds().Min.Y+y)
		for x := range size.X {
			c := straightAt(img.Pix, row+4*x)
			if c[3] == 0 {
				hasTransparent = true
				continue
			}
			bin := &bins[binOf(c[0], c[1], c[2], c[3])]
			bin.count++
			for ch := range 4 {
				bin.sum[ch] += uint64(c[ch])
			}
		}
	}
	used := bins[:0]
	for _, bin := range bins {
		if bin.count > 0 {
			used = append(used, bin)
		}
	}
	return used, hasTransparent
}

// buildPalette splits the used bins into up to colors boxes, each time halving the box
// with the widest channel range at its pixel-weighted median, and averages each box.
func buildPalette(bins []colorStats, colors int) color.Palette {
	if len(bins) == 0 {
		return nil // every pixel is transparent; Quantize adds that entry
	}
	boxes := [][]colorStats{bins}
	for len(boxes) < colors {
		index, channel := widestBox(boxes)
		if index < 0 {
			break
		}
		low, high := splitAtMedian(boxes[index], channel)
		boxes[index] = low
		boxes = append(boxes, high)
	}
	palette := make(color.Palette, 0, len(boxes))
	for _, box := range boxes {
		palette = append(palette, averageColor(box))
	}
	return palette
}

// widestBox returns the splittable box with the widest channel range, or -1.
func widestBox(boxes [][]colorStats) (index, channel int) {
	index, widest := -1, uint64(0)
	for i, box := range boxes {
		if len(box) < 2 {
			continue
		}
		for ch := range 4 {
			lo, hi := box[0].mean(ch), box[0].mean(ch)
			for _, bin := range box[1:] {
				lo, hi = min(lo, bin.mean(ch)), max(hi, bin.mean(ch))
			}
			if hi-lo > widest || index < 0 {
				index, channel, widest = i, ch, hi-lo
			}
		}
	}
	return index, channel
}

func splitAtMedian(box []colorStats, channel int) ([]colorStats, []colorStats) {
	slices.SortFunc(box, func(a, b colorStats) int { return int(a.mean(channel)) - int(b.mean(channel)) })
	var total, seen uint64
	for _, bin := range box {
		total += bin.count
	}
	split := 1
	for i, bin := range box[:len(box)-1] {
		seen += bin.count
		split = i + 1
		if seen*2 >= total {
			break
		}
	}
	return box[:split:split], box[split:]
}

func averageColor(box []colorStats) color.NRGBA {
	var count uint64
	var sum [4]uint64
	for _, bin := range box {
		count += bin.count
		for ch := range 4 {
			sum[ch] += bin.sum[ch]
		}
	}
	mean := func(ch int) uint8 { return uint8((sum[ch] + count/2) / count) }
	return color.NRGBA{R: mean(0), G: mean(1), B: mean(2), A: mean(3)}
}

// nearestCache remembers the nearest palette entry per histogram bin.
type nearestCache struct {
	palette [][4]int32
	indexes []int16 // −1: not looked up yet
}

func newNearestCache(palette color.Palette) *nearestCache {
	cache := &nearestCache{indexes: make([]int16, binCount)}
	for i := range cache.indexes {
		cache.indexes[i] = -1
	}
	for _, c := range palette {
		n := c.(color.NRGBA)
		cache.palette = append(cache.palette, [4]int32{int32(n.R), int32(n.G), int32(n.B), int32(n.A)})
	}
	return cache
}

func (c *nearestCache) nearest(px [4]int32) int {
	bin := binOf(px[0], px[1], px[2], px[3])
	if c.indexes[bin] >= 0 {
		return int(c.indexes[bin])
	}
	best, bestDistance := 0, int32(-1)
	for i, entry := range c.palette {
		var distance int32
		for ch := range 4 {
			d := px[ch] - entry[ch]
			distance += d * d
		}
		if bestDistance < 0 || distance < bestDistance {
			best, bestDistance = i, distance
		}
	}
	c.indexes[bin] = int16(best)
	return best
}

// mapPixels picks a palette entry for every pixel, spreading each pixel's error to its
// unvisited neighbors (Floyd–Steinberg) when dither is set.
func mapPixels(out *image.Paletted, img *image.RGBA, cache *nearestCache, dither bool) {
	size := img.Bounds().Size()
	current, next := make([][4]int32, size.X+2), make([][4]int32, size.X+2)
	for y := range size.Y {
		row := img.PixOffset(img.Bounds().Min.X, img.Bounds().Min.Y+y)
		for x := range size.X {
			px := straightAt(img.Pix, row+4*x)
			if px[3] == 0 {
				out.Pix[y*out.Stride+x] = 0 // the transparent entry: Quantize adds it whenever a pixel is empty
				continue                    // no error flows into or out of empty pixels
			}
			if dither {
				for ch := range 3 {
					px[ch] = max(0, min(255, px[ch]+current[x+1][ch]/16))
				}
			}
			index := cache.nearest(px)
			out.Pix[y*out.Stride+x] = uint8(index)
			if dither {
				spreadError(current, next, x+1, px, cache.palette[index])
			}
		}
		current, next = next, current
		clear(next)
	}
}

// spreadError hands the color difference between wanted and got to the right (7/16) and
// the row below (3/16, 5/16, 1/16), in sixteenths. Alpha isn't diffused: spread alpha
// error shows as faint blotches around soft edges.
func spreadError(current, next [][4]int32, x int, wanted, got [4]int32) {
	for ch := range 3 {
		e := wanted[ch] - got[ch]
		current[x+1][ch] += 7 * e
		next[x-1][ch] += 3 * e
		next[x][ch] += 5 * e
		next[x+1][ch] += e
	}
}
