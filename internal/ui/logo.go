package ui

import (
	"image"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"golang.org/x/image/draw"
)

// Logo draws an image at any size, staying sharp by resampling once per pixel size.
// Gio only scales images linearly, which looks jagged when shrinking a large logo.
type Logo struct {
	source       image.Image
	imagesBySize map[int]paint.ImageOp
}

// NewLogo wraps a source image, ideally large and square.
func NewLogo(source image.Image) *Logo {
	return &Logo{source: source, imagesBySize: make(map[int]paint.ImageOp)}
}

// Layout draws the logo as a square of the given size.
func (l *Logo) Layout(gtx layout.Context, size unit.Dp) layout.Dimensions {
	sidePx := gtx.Dp(size)
	square := image.Pt(sidePx, sidePx)

	defer clip.Rect{Max: square}.Push(gtx.Ops).Pop()
	l.imageAt(sidePx).Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: square}
}

func (l *Logo) imageAt(sidePx int) paint.ImageOp {
	if cached, ok := l.imagesBySize[sidePx]; ok {
		return cached
	}
	resized := image.NewNRGBA(image.Rect(0, 0, sidePx, sidePx))
	draw.CatmullRom.Scale(resized, resized.Bounds(), l.source, l.source.Bounds(), draw.Src, nil)
	imageOp := paint.NewImageOp(resized)
	l.imagesBySize[sidePx] = imageOp
	return imageOp
}
