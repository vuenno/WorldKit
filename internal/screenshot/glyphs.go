package screenshot

import (
	"fmt"
	"image"
	"math"
	"sync"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"golang.org/x/image/vector"

	"world-kit/internal/directwrite"
)

// Chromium places glyphs at quarter-pixel steps: each glyph is drawn at its x rounded to
// the nearest quarter, its y rounded to a whole pixel.
const subpixelSteps = 4

// glyphRasterizer draws a glyph's raw coverage (before coverageTable) with its pen at
// quarter/subpixelSteps of a pixel right of the mask's origin. Bounds are relative to the
// pen's whole-pixel position, y growing down from the baseline.
type glyphRasterizer interface {
	rasterize(glyph sfnt.GlyphIndex, quarter int) (*image.Alpha, error)
}

// directWriteRasterizer draws glyphs as Chromium does for a font that has the style
// itself (Arial Bold, Arial Bold Italic): with DirectWrite.
type directWriteRasterizer struct {
	font   *directwrite.Font
	sizePx float32
}

func (r directWriteRasterizer) rasterize(glyph sfnt.GlyphIndex, quarter int) (*image.Alpha, error) {
	return r.font.GlyphMask(uint16(glyph), r.sizePx, float32(quarter)/subpixelSteps)
}

// outlineRasterizer draws glyphs from their outlines, as Chromium's Skia does for a style
// it has to fake (Mukta comes only in Regular, so the chat's bold is synthetic): the
// outline is thickened by emboldenPx, slanted by skew for a fake italic, then drawn with
// Skia's ClearType filter and averaged to gray.
type outlineRasterizer struct {
	font       *sfnt.Font
	sizePx     float64
	emboldenPx float64
	skew       float64 // x shift per pixel above the baseline
}

// fakeBoldPx is how much Skia thickens a glyph to fake bold at sizePx: a stroke of
// 1/24 of the size at 9 px, easing to 1/32 at 36 px and above.
func fakeBoldPx(sizePx float64) float64 {
	t := max(0, min(1, (sizePx-9)/(36-9)))
	return sizePx * (1.0/24 + t*(1.0/32-1.0/24))
}

// fakeItalicSkew is the slant Chromium gives a fake italic.
const fakeItalicSkew = 0.25

func (r outlineRasterizer) rasterize(glyph sfnt.GlyphIndex, quarter int) (*image.Alpha, error) {
	contours, err := r.contours(glyph)
	if err != nil {
		return nil, err
	}
	if len(contours) == 0 {
		return image.NewAlpha(image.Rectangle{}), nil
	}
	if r.emboldenPx > 0 {
		embolden(contours, r.emboldenPx)
	}
	if r.skew != 0 {
		for _, c := range contours {
			for i := range c.points {
				c.points[i].x -= r.skew * c.points[i].y
			}
		}
	}
	return clearTypeCoverage(contours, float64(quarter)/subpixelSteps), nil
}

// contour is one closed loop of a glyph outline in pixels, y growing down. ops say how
// points are joined: a move takes one point, a line one, a quadratic curve two and a
// cubic three.
type contour struct {
	points []outlinePoint
	ops    []sfnt.SegmentOp
}

type outlinePoint struct{ x, y float64 }

func (r outlineRasterizer) contours(glyph sfnt.GlyphIndex) ([]*contour, error) {
	var buffer sfnt.Buffer
	unitsPerEm := r.font.UnitsPerEm()
	// Loading at one pixel per design unit keeps full precision; scale here instead.
	segments, err := r.font.LoadGlyph(&buffer, glyph, fixed.Int26_6(unitsPerEm)<<6, nil)
	if err != nil {
		return nil, fmt.Errorf("load glyph %d: %w", glyph, err)
	}
	scale := r.sizePx / float64(unitsPerEm) / 64
	var contours []*contour
	for _, segment := range segments {
		if segment.Op == sfnt.SegmentOpMoveTo {
			contours = append(contours, &contour{})
		}
		current := contours[len(contours)-1]
		current.ops = append(current.ops, segment.Op)
		for _, arg := range segment.Args[:pointsPerOp(segment.Op)] {
			current.points = append(current.points, outlinePoint{float64(arg.X) * scale, float64(arg.Y) * scale})
		}
	}
	return contours, nil
}

func pointsPerOp(op sfnt.SegmentOp) int {
	switch op {
	case sfnt.SegmentOpQuadTo:
		return 2
	case sfnt.SegmentOpCubeTo:
		return 3
	default:
		return 1
	}
}

// embolden moves every outline point outward so strokes grow by strength in total, with
// mitered corners: FreeType's FT_Outline_EmboldenXY, which closely matches the
// stroke-and-fill Skia fakes bold with.
func embolden(contours []*contour, strength float64) {
	isClockwise := outlineArea(contours) > 0 // on screen; outer contours set the direction
	half := strength / 2
	for _, c := range contours {
		shifts := make([]outlinePoint, len(c.points))
		for i := range c.points {
			shifts[i] = emboldenShift(c.points, i, half, isClockwise)
		}
		for i := range c.points {
			c.points[i].x += shifts[i].x
			c.points[i].y += shifts[i].y
		}
	}
}

// emboldenShift is how far point i of a contour moves, along the bisector of its two
// edges, so both edges move out by half.
func emboldenShift(points []outlinePoint, i int, half float64, isClockwise bool) outlinePoint {
	prev, next := distinctNeighbor(points, i, -1), distinctNeighbor(points, i, 1)
	p := points[i]
	inX, inY, inLength := unit(p.x-prev.x, p.y-prev.y)
	outX, outY, outLength := unit(next.x-p.x, next.y-p.y)
	if inLength == 0 || outLength == 0 {
		return outlinePoint{}
	}
	cosine := inX*outX + inY*outY
	if cosine <= -0.9375 { // a spike: leave it rather than shoot it far out
		return outlinePoint{}
	}
	// The outward side of an edge (dx, dy) is (dy, -dx) for an outline running clockwise
	// on screen, (-dy, dx) otherwise; turn is positive at the outline's convex corners.
	shiftX, shiftY := inY+outY, -(inX + outX)
	turn := inX*outY - inY*outX
	if !isClockwise {
		shiftX, shiftY, turn = -shiftX, -shiftY, -turn
	}
	scale := half / (1 + cosine)
	// A corner between short edges can't move further than those edges are long.
	if shortest := min(inLength, outLength); half*turn > shortest*(1+cosine) {
		scale = shortest / turn
	}
	return outlinePoint{shiftX * scale, shiftY * scale}
}

// distinctNeighbor walks from point i in direction step (±1, wrapping) to the first point
// that isn't at the same place, so a contour that repeats its start point at its end
// still has two edges at every point.
func distinctNeighbor(points []outlinePoint, i, step int) outlinePoint {
	n := len(points)
	for k := 1; k < n; k++ {
		q := points[((i+step*k)%n+n)%n]
		if math.Abs(q.x-points[i].x) > 1e-9 || math.Abs(q.y-points[i].y) > 1e-9 {
			return q
		}
	}
	return points[i]
}

func unit(x, y float64) (ux, uy, length float64) {
	length = math.Hypot(x, y)
	if length == 0 {
		return 0, 0, 0
	}
	return x / length, y / length, length
}

// outlineArea is twice the outline's signed area over all contours, positive when it
// runs clockwise on screen (y down).
func outlineArea(contours []*contour) float64 {
	area := 0.0
	for _, c := range contours {
		for i, p := range c.points {
			q := c.points[(i+1)%len(c.points)]
			area += p.x*q.y - q.x*p.y
		}
	}
	return area
}

// skiaClearTypeFilter is Skia's filter for drawing an outline as ClearType: for each of a
// pixel's red, green and blue parts, weights (out of 256) over 12 samples at 4× width,
// spanning the pixel before, the pixel and the pixel after.
var skiaClearTypeFilter = [3][3 * clearTypeSamples]int{
	{0x03, 0x0b, 0x1c, 0x33, 0x40, 0x39, 0x24, 0x10, 0x05, 0x01, 0x00, 0x00},
	{0x00, 0x02, 0x08, 0x16, 0x2b, 0x3d, 0x3d, 0x2b, 0x16, 0x08, 0x02, 0x00},
	{0x00, 0x00, 0x01, 0x05, 0x10, 0x24, 0x39, 0x40, 0x33, 0x1c, 0x0b, 0x03},
}

const clearTypeSamples = 4 // per pixel, across

// clearTypeCoverage draws the outline with its pen at (penX, 0) at four samples per pixel
// across, filters the samples into red, green and blue coverage as Skia does, and averages
// them into gray coverage.
func clearTypeCoverage(contours []*contour, penX float64) *image.Alpha {
	bounds := outlineBounds(contours, penX).Inset(-1) // the filter reaches a pixel each side
	wide := rasterizeWide(contours, penX, bounds)
	mask := image.NewAlpha(bounds)
	wideStride := bounds.Dx() * clearTypeSamples
	for y := 0; y < bounds.Dy(); y++ {
		row := wide.Pix[y*wideStride : (y+1)*wideStride]
		for x := 0; x < bounds.Dx(); x++ {
			mask.Pix[y*mask.Stride+x] = filteredCoverage(row, x)
		}
	}
	return mask
}

// filteredCoverage is pixel x's gray coverage from a row of samples.
func filteredCoverage(samples []uint8, x int) uint8 {
	first := (x - 1) * clearTypeSamples
	total := 0
	for channel := range skiaClearTypeFilter {
		sum := 0
		for k, weight := range skiaClearTypeFilter[channel] {
			if s := first + k; s >= 0 && s < len(samples) {
				sum += weight * int(samples[s])
			}
		}
		total += min(sum>>8, 255)
	}
	return uint8(total / len(skiaClearTypeFilter))
}

// outlineBounds is the pixels the outline touches with its pen at (penX, 0).
func outlineBounds(contours []*contour, penX float64) image.Rectangle {
	minX, minY := math.Inf(1), math.Inf(1)
	maxX, maxY := math.Inf(-1), math.Inf(-1)
	for _, c := range contours {
		for _, p := range c.points {
			minX, maxX = min(minX, p.x+penX), max(maxX, p.x+penX)
			minY, maxY = min(minY, p.y), max(maxY, p.y)
		}
	}
	return image.Rect(int(math.Floor(minX)), int(math.Floor(minY)), int(math.Ceil(maxX)), int(math.Ceil(maxY)))
}

// rasterizeWide draws the outline's exact area coverage over bounds at clearTypeSamples
// columns per pixel.
func rasterizeWide(contours []*contour, penX float64, bounds image.Rectangle) *image.Alpha {
	width, height := bounds.Dx()*clearTypeSamples, bounds.Dy()
	rasterizer := vector.NewRasterizer(width, height)
	at := func(p outlinePoint) (float32, float32) {
		return float32((p.x + penX - float64(bounds.Min.X)) * clearTypeSamples), float32(p.y - float64(bounds.Min.Y))
	}
	for _, c := range contours {
		points := c.points
		for _, op := range c.ops {
			switch op {
			case sfnt.SegmentOpMoveTo:
				rasterizer.MoveTo(at(points[0]))
			case sfnt.SegmentOpLineTo:
				rasterizer.LineTo(at(points[0]))
			case sfnt.SegmentOpQuadTo:
				x1, y1 := at(points[0])
				x2, y2 := at(points[1])
				rasterizer.QuadTo(x1, y1, x2, y2)
			case sfnt.SegmentOpCubeTo:
				x1, y1 := at(points[0])
				x2, y2 := at(points[1])
				x3, y3 := at(points[2])
				rasterizer.CubeTo(x1, y1, x2, y2, x3, y3)
			}
			points = points[pointsPerOp(op):]
		}
		rasterizer.ClosePath()
	}
	wide := image.NewAlpha(image.Rect(0, 0, width, height))
	rasterizer.Draw(wide, wide.Bounds(), image.Opaque, image.Point{})
	return wide
}

// glyphKey names one drawn glyph in glyphCache.
type glyphKey struct {
	face    *chatFace
	glyph   sfnt.GlyphIndex
	quarter int
}

// glyphCache keeps drawn glyphs: the editor redraws the whole picture on every keystroke,
// and the same few dozen glyphs make up nearly every line. Faces are cached for the
// process's life too, so keys stay valid.
var glyphCache = struct {
	sync.Mutex
	masks map[glyphKey]*image.Alpha
}{masks: map[glyphKey]*image.Alpha{}}

// maxCachedGlyphs bounds glyphCache; past it the cache starts over.
const maxCachedGlyphs = 16384

// glyphMask returns face's glyph drawn at quarter, from the cache when it was drawn
// before. The mask is shared: don't modify it.
func glyphMask(face *chatFace, glyph sfnt.GlyphIndex, quarter int) (*image.Alpha, error) {
	key := glyphKey{face, glyph, quarter}
	glyphCache.Lock()
	mask, ok := glyphCache.masks[key]
	glyphCache.Unlock()
	if ok {
		return mask, nil
	}
	mask, err := face.raster.rasterize(glyph, quarter)
	if err != nil {
		return nil, err
	}
	glyphCache.Lock()
	defer glyphCache.Unlock()
	if len(glyphCache.masks) >= maxCachedGlyphs {
		clear(glyphCache.masks)
	}
	glyphCache.masks[key] = mask
	return mask, nil
}
