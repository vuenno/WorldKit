package compose

import (
	"context"
	"fmt"
	"image"
	"image/draw"
	"math"
	"runtime"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/math/f64"
)

// edgeMargin is the transparent border around a resampled layer, so the resampling kernel
// blends rotated edges into transparency instead of cutting them hard.
const edgeMargin = 2

// maxLayerScale caps a layer's resampled size at this many canvas sides; anything larger
// is mostly off the canvas and only costs memory.
const maxLayerScale = 2

// LayerError is returned when a layer's image can't be read for export (moved, deleted or
// unreadable).
type LayerError struct {
	Name string
	Err  error
}

func (e *LayerError) Error() string { return fmt.Sprintf("layer %q: %v", e.Name, e.Err) }

// Unwrap returns the read error.
func (e *LayerError) Unwrap() error { return e.Err }

// Opener reads a layer's source image; Decode is the usual one.
type Opener func(path string) (image.Image, error)

// Render draws the composition at full size on the CPU, bottom layer first. It blends in
// linear light, as Gio's GPU preview does, so the file looks like the editor showed it.
// Peak memory is the canvas plus one layer at a time.
func Render(ctx context.Context, comp Composition, open Opener) (*image.RGBA, error) {
	canvas := newLinearCanvas(comp.Size, comp.Background)
	for _, layer := range comp.Layers {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if layer.IsHidden || layer.Opacity <= 0 || layer.Size.X < 1 || layer.Size.Y < 1 {
			continue
		}
		source, err := open(layer.Source)
		if err != nil {
			return nil, &LayerError{Name: layer.Name, Err: err}
		}
		placed := placeLayer(layer, source, comp.Size)
		canvas.composite(placed, layer.Opacity)
	}
	return canvas.toRGBA(), nil
}

// placeLayer resamples source to its size and position on the canvas and returns it as
// premultiplied sRGB, covering only the part of the canvas the layer touches.
func placeLayer(layer Layer, source image.Image, canvasSize image.Point) *image.RGBA {
	scaled, scaledSize := scaleWithMargin(source, layer.Size, canvasSize)
	lo, hi := layer.Bounds()
	area := image.Rect(int(math.Floor(lo.X))-1, int(math.Floor(lo.Y))-1, int(math.Ceil(hi.X))+1, int(math.Ceil(hi.Y))+1)
	area = area.Intersect(image.Rectangle{Max: canvasSize})
	placed := image.NewRGBA(area)
	if area.Empty() {
		return placed
	}
	// scaled pixel (u, v) → natural (u − margin, v − margin) · Natural/scaledSize → canvas.
	toNatural := Vec{float64(layer.Natural.X) / scaledSize.X, float64(layer.Natural.Y) / scaledSize.Y}
	m := layer.Transform()
	s2d := f64.Aff3{
		m.A * toNatural.X, m.B * toNatural.Y, m.C - (m.A*toNatural.X+m.B*toNatural.Y)*edgeMargin,
		m.D * toNatural.X, m.E * toNatural.Y, m.F - (m.D*toNatural.X+m.E*toNatural.Y)*edgeMargin,
	}
	if isPixelAligned(s2d) {
		offset := image.Pt(int(math.Round(s2d[2])), int(math.Round(s2d[5])))
		draw.Draw(placed, area, scaled, area.Min.Sub(offset), draw.Src)
		return placed
	}
	inParallelBands(area, func(band image.Rectangle) {
		xdraw.CatmullRom.Transform(placed.SubImage(band).(*image.RGBA), s2d, scaled, scaled.Bounds(), xdraw.Src, nil)
	})
	return placed
}

// scaleWithMargin resamples source to size (capped by canvasSize) inside a transparent
// edgeMargin border. It returns the image and the size the source was scaled to.
func scaleWithMargin(source image.Image, size Vec, canvasSize image.Point) (*image.RGBA, Vec) {
	limit := float64(maxLayerScale * max(canvasSize.X, canvasSize.Y))
	if size.X > limit || size.Y > limit {
		size = FitInside(image.Pt(int(size.X), int(size.Y)), Vec{limit, limit})
	}
	target := image.Pt(max(1, int(math.Round(size.X))), max(1, int(math.Round(size.Y))))
	out := image.NewRGBA(image.Rect(0, 0, target.X+2*edgeMargin, target.Y+2*edgeMargin))
	inner := image.Rectangle{Min: image.Pt(edgeMargin, edgeMargin), Max: target.Add(image.Pt(edgeMargin, edgeMargin))}
	if source.Bounds().Size() == target {
		draw.Draw(out, inner, source, source.Bounds().Min, draw.Src)
	} else {
		xdraw.CatmullRom.Scale(out, inner, source, source.Bounds(), xdraw.Src, nil)
	}
	return out, Vec{float64(target.X), float64(target.Y)}
}

// isPixelAligned reports whether m only moves by whole pixels, so a plain copy draws the
// layer exactly: chat images at their real size keep every pixel.
func isPixelAligned(m f64.Aff3) bool {
	const tolerance = 1e-6
	isWhole := func(v float64) bool { return math.Abs(v-math.Round(v)) < 1e-3 }
	return math.Abs(m[0]-1) < tolerance && math.Abs(m[1]) < tolerance &&
		math.Abs(m[3]) < tolerance && math.Abs(m[4]-1) < tolerance &&
		isWhole(m[2]) && isWhole(m[5])
}

// inParallelBands splits area into horizontal bands and runs work on each at once. Every
// band writes only its own rows, so the result is the same as one pass.
func inParallelBands(area image.Rectangle, work func(band image.Rectangle)) {
	bands := min(runtime.NumCPU(), max(1, area.Dy()/64))
	var wg sync.WaitGroup
	for i := range bands {
		band := area
		band.Min.Y = area.Min.Y + area.Dy()*i/bands
		band.Max.Y = area.Min.Y + area.Dy()*(i+1)/bands
		wg.Add(1)
		go func() {
			defer wg.Done()
			work(band)
		}()
	}
	wg.Wait()
}
