package compose

import "math"

// Affine maps a point (x, y) to (A·x + B·y + C, D·x + E·y + F).
type Affine struct {
	A, B, C, D, E, F float64
}

// Apply maps p.
func (m Affine) Apply(p Vec) Vec {
	return Vec{m.A*p.X + m.B*p.Y + m.C, m.D*p.X + m.E*p.Y + m.F}
}

// Transform maps the layer's source pixels (0..Natural) onto the canvas. The preview and
// the export both draw with it, so they agree on where every pixel goes.
func (l Layer) Transform() Affine {
	sx, sy := 1.0, 1.0
	if l.Natural.X > 0 && l.Natural.Y > 0 {
		sx, sy = l.Size.X/float64(l.Natural.X), l.Size.Y/float64(l.Natural.Y)
	}
	sin, cos := math.Sincos(l.Rotation)
	halfX, halfY := float64(l.Natural.X)/2, float64(l.Natural.Y)/2
	// Translate(Center) · Rotate · Scale · Translate(−Natural/2)
	return Affine{
		A: cos * sx, B: -sin * sy, C: l.Center.X - cos*sx*halfX + sin*sy*halfY,
		D: sin * sx, E: cos * sy, F: l.Center.Y - sin*sx*halfX - cos*sy*halfY,
	}
}

// ToLocal returns p relative to the layer's center, in the layer's unrotated frame.
func (l Layer) ToLocal(p Vec) Vec {
	sin, cos := math.Sincos(-l.Rotation)
	d := p.Sub(l.Center)
	return Vec{cos*d.X - sin*d.Y, sin*d.X + cos*d.Y}
}

// FromLocal is the inverse of ToLocal.
func (l Layer) FromLocal(local Vec) Vec {
	sin, cos := math.Sincos(l.Rotation)
	return l.Center.Add(Vec{cos*local.X - sin*local.Y, sin*local.X + cos*local.Y})
}

// Contains reports whether canvas point p is on the layer.
func (l Layer) Contains(p Vec) bool {
	local := l.ToLocal(p)
	return math.Abs(local.X) <= l.Size.X/2 && math.Abs(local.Y) <= l.Size.Y/2
}

// Corners returns the layer's corners on the canvas: top-left, top-right, bottom-right,
// bottom-left (before rotation).
func (l Layer) Corners() [4]Vec {
	hx, hy := l.Size.X/2, l.Size.Y/2
	return [4]Vec{
		l.FromLocal(Vec{-hx, -hy}), l.FromLocal(Vec{hx, -hy}),
		l.FromLocal(Vec{hx, hy}), l.FromLocal(Vec{-hx, hy}),
	}
}

// Bounds returns the axis-aligned box around the rotated layer: min and max corners.
func (l Layer) Bounds() (Vec, Vec) {
	corners := l.Corners()
	lo, hi := corners[0], corners[0]
	for _, c := range corners[1:] {
		lo = Vec{math.Min(lo.X, c.X), math.Min(lo.Y, c.Y)}
		hi = Vec{math.Max(hi.X, c.X), math.Max(hi.Y, c.Y)}
	}
	return lo, hi
}

// LayerAt returns the index of the top-most visible layer under p, or -1.
func (c Composition) LayerAt(p Vec) int {
	for i := len(c.Layers) - 1; i >= 0; i-- {
		if !c.Layers[i].IsHidden && c.Layers[i].Contains(p) {
			return i
		}
	}
	return -1
}

// Handle is a grip on the selected layer's frame.
type Handle int

const (
	HandleNone Handle = iota
	HandleNorthWest
	HandleNorth
	HandleNorthEast
	HandleEast
	HandleSouthEast
	HandleSouth
	HandleSouthWest
	HandleWest
	HandleRotate
)

// resizeHandles are the eight resize grips in drawing order.
var resizeHandles = []Handle{
	HandleNorthWest, HandleNorth, HandleNorthEast, HandleEast,
	HandleSouthEast, HandleSouth, HandleSouthWest, HandleWest,
}

// ResizeHandles returns the eight resize grips.
func ResizeHandles() []Handle {
	return resizeHandles
}

// Direction is the handle's side of the layer: −1, 0 or 1 on each axis.
func (h Handle) Direction() (int, int) {
	switch h {
	case HandleNorthWest:
		return -1, -1
	case HandleNorth:
		return 0, -1
	case HandleNorthEast:
		return 1, -1
	case HandleEast:
		return 1, 0
	case HandleSouthEast:
		return 1, 1
	case HandleSouth:
		return 0, 1
	case HandleSouthWest:
		return -1, 1
	case HandleWest:
		return -1, 0
	}
	return 0, 0
}

// IsCorner reports whether the handle resizes both axes.
func (h Handle) IsCorner() bool {
	dx, dy := h.Direction()
	return dx != 0 && dy != 0
}

// HandlePosition returns where h sits on the canvas. The rotate handle is rotateOffset
// canvas pixels above the top edge's middle.
func (l Layer) HandlePosition(h Handle, rotateOffset float64) Vec {
	if h == HandleRotate {
		return l.FromLocal(Vec{0, -l.Size.Y/2 - rotateOffset})
	}
	dx, dy := h.Direction()
	return l.FromLocal(Vec{float64(dx) * l.Size.X / 2, float64(dy) * l.Size.Y / 2})
}

// HandleAt returns the handle within radius of p (all in canvas pixels), or HandleNone.
func (l Layer) HandleAt(p Vec, radius, rotateOffset float64) Handle {
	if distance(p, l.HandlePosition(HandleRotate, rotateOffset)) <= radius {
		return HandleRotate
	}
	for _, h := range resizeHandles {
		if distance(p, l.HandlePosition(h, rotateOffset)) <= radius {
			return h
		}
	}
	return HandleNone
}

func distance(a, b Vec) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

// Moved returns start shifted by delta.
func Moved(start Layer, delta Vec) Layer {
	start.Center = start.Center.Add(delta)
	return start
}

// Resized returns start resized by dragging handle h to pointer, the opposite side staying
// put. Corners keep the aspect ratio when keepAspect is set; edges stretch one axis.
// Neither side gets smaller than minSide canvas pixels.
func Resized(start Layer, h Handle, pointer Vec, keepAspect bool, minSide float64) Layer {
	dx, dy := h.Direction()
	if dx == 0 && dy == 0 {
		return start
	}
	local := start.ToLocal(pointer)
	anchor := Vec{-float64(dx) * start.Size.X / 2, -float64(dy) * start.Size.Y / 2}
	size := start.Size
	if dx != 0 {
		size.X = math.Max(minSide, float64(dx)*(local.X-anchor.X))
	}
	if dy != 0 {
		size.Y = math.Max(minSide, float64(dy)*(local.Y-anchor.Y))
	}
	if keepAspect && h.IsCorner() && start.Size.X > 0 && start.Size.Y > 0 {
		scale := math.Max(size.X/start.Size.X, size.Y/start.Size.Y)
		scale = math.Max(scale, minSide/math.Min(start.Size.X, start.Size.Y))
		size = start.Size.Scale(scale)
	}
	center := Vec{}
	if dx != 0 {
		center.X = anchor.X + float64(dx)*size.X/2
	}
	if dy != 0 {
		center.Y = anchor.Y + float64(dy)*size.Y/2
	}
	start.Center = start.FromLocal(center)
	start.Size = size
	return start
}

// RotationSnap is the step rotation snaps to while Shift is held.
const RotationSnap = math.Pi / 12 // 15°

// Rotated returns start turned by the angle the pointer swept around its center since
// grab. With snap, the result lands on a multiple of RotationSnap.
func Rotated(start Layer, grab, pointer Vec, snap bool) Layer {
	angle := func(p Vec) float64 { return math.Atan2(p.Y-start.Center.Y, p.X-start.Center.X) }
	rotation := start.Rotation + angle(pointer) - angle(grab)
	if snap {
		rotation = math.Round(rotation/RotationSnap) * RotationSnap
	}
	start.Rotation = NormalizeAngle(rotation)
	return start
}

// Guides are the canvas lines a snapped layer lines up with, in canvas pixels; NaN when
// that axis didn't snap.
type Guides struct {
	X, Y float64
}

// Snapped moves l so its bounds' nearest edge or center lines up with the canvas' edges
// or center when within threshold canvas pixels, and returns the lines it snapped to.
func Snapped(l Layer, canvas Vec, threshold float64) (Layer, Guides) {
	lo, hi := l.Bounds()
	shiftX, guideX := snapAxis([3]float64{lo.X, (lo.X + hi.X) / 2, hi.X}, canvas.X, threshold)
	shiftY, guideY := snapAxis([3]float64{lo.Y, (lo.Y + hi.Y) / 2, hi.Y}, canvas.Y, threshold)
	l.Center = l.Center.Add(Vec{shiftX, shiftY})
	return l, Guides{X: guideX, Y: guideY}
}

// snapAxis finds the smallest shift that puts one of edges on 0, length/2 or length.
func snapAxis(edges [3]float64, length, threshold float64) (shift, guide float64) {
	best, guide := threshold, math.NaN()
	for _, target := range [3]float64{0, length / 2, length} {
		for _, edge := range edges {
			if d := target - edge; math.Abs(d) <= math.Abs(best) {
				best, shift, guide = math.Abs(d), d, target
			}
		}
	}
	return shift, guide
}
