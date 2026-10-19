// Package compose lays images out on a canvas (the Screenshot Editor's model), renders
// the result on the CPU and encodes it as PNG or JPEG. It knows nothing about the UI.
package compose

import (
	"image"
	"math"
)

// Canvas size limits, in pixels. 8192 is also the most a source image may measure: the
// preview's GPU textures and the export's memory both grow with it.
const (
	MinCanvasSide = 16
	MaxCanvasSide = 8192
	MaxSourceSide = 8192
)

// Background is what fills the canvas under the layers.
type Background int

const (
	BackgroundTransparent Background = iota // PNG keeps it; JPEG has no alpha and gets black
	BackgroundBlack
	BackgroundWhite
)

// Kind tells photos (game screenshots) from chat images (extracted chat PNGs).
type Kind int

const (
	KindPhoto Kind = iota
	KindChat
)

// Vec is a point or a size in canvas pixels.
type Vec struct {
	X, Y float64
}

// Add returns v + w.
func (v Vec) Add(w Vec) Vec { return Vec{v.X + w.X, v.Y + w.Y} }

// Sub returns v − w.
func (v Vec) Sub(w Vec) Vec { return Vec{v.X - w.X, v.Y - w.Y} }

// Scale returns v × s.
func (v Vec) Scale(s float64) Vec { return Vec{v.X * s, v.Y * s} }

// Layer is one image placed on the canvas. Position, size and rotation are around the
// layer's center, so resizing and rotating never move it unexpectedly.
type Layer struct {
	ID       int    // stable while the layer exists, for the UI's image cache
	Source   string // the image file; the only reference to its pixels
	Name     string // shown in the layers list
	Kind     Kind
	Natural  image.Point // the source's size in pixels
	Center   Vec         // canvas pixels
	Size     Vec         // shown width and height, canvas pixels
	Rotation float64     // radians, clockwise on screen
	Opacity  float64     // 0..1
	IsHidden bool        // not drawn and not exported
}

// Composition is the canvas and the layers on it; Layers[0] is at the bottom.
type Composition struct {
	Size       image.Point
	Background Background
	Layers     []Layer
}

// Clone returns a copy whose layers can change without touching c's.
func (c Composition) Clone() Composition {
	c.Layers = append([]Layer(nil), c.Layers...)
	return c
}

// IndexOf returns the position of the layer with id, or -1.
func (c Composition) IndexOf(id int) int {
	for i, layer := range c.Layers {
		if layer.ID == id {
			return i
		}
	}
	return -1
}

// MoveLayer moves the layer at from to position to (clamped), shifting the others.
func (c *Composition) MoveLayer(from, to int) {
	if from < 0 || from >= len(c.Layers) {
		return
	}
	to = max(0, min(to, len(c.Layers)-1))
	layer := c.Layers[from]
	c.Layers = append(c.Layers[:from], c.Layers[from+1:]...)
	c.Layers = append(c.Layers[:to], append([]Layer{layer}, c.Layers[to:]...)...)
}

// Resize changes the canvas size and keeps every layer where it was relative to the
// canvas center, so a centered layout stays centered.
func (c *Composition) Resize(size image.Point) {
	shift := Vec{float64(size.X-c.Size.X) / 2, float64(size.Y-c.Size.Y) / 2}
	for i := range c.Layers {
		c.Layers[i].Center = c.Layers[i].Center.Add(shift)
	}
	c.Size = size
}

// ClampCanvasSize keeps a typed canvas size within the limits.
func ClampCanvasSize(size image.Point) image.Point {
	return image.Pt(clampInt(size.X, MinCanvasSide, MaxCanvasSide), clampInt(size.Y, MinCanvasSide, MaxCanvasSide))
}

func clampInt(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

// FitInside returns natural scaled to fit inside box with its aspect ratio kept.
func FitInside(natural image.Point, box Vec) Vec {
	if natural.X <= 0 || natural.Y <= 0 {
		return box
	}
	scale := math.Min(box.X/float64(natural.X), box.Y/float64(natural.Y))
	return Vec{float64(natural.X) * scale, float64(natural.Y) * scale}
}

// NormalizeAngle maps radians into (−π, π].
func NormalizeAngle(angle float64) float64 {
	angle = math.Mod(angle, 2*math.Pi)
	if angle <= -math.Pi {
		angle += 2 * math.Pi
	}
	if angle > math.Pi {
		angle -= 2 * math.Pi
	}
	return angle
}
