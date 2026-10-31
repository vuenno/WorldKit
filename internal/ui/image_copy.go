package ui

import (
	"image"
	"log/slog"

	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget/material"

	"world-kit/internal/clipboard"
)

// imageCopy draws an image and puts it on the clipboard off the UI goroutine, then says
// how it went in a toast. Gio's own clipboard carries text only.
type imageCopy struct {
	copier *backgroundLoad[struct{}]
}

func newImageCopy(invalidate func()) *imageCopy {
	return &imageCopy{copier: newBackgroundLoad[struct{}](invalidate)}
}

// Start draws the image with draw and copies it, in the background.
func (c *imageCopy) Start(draw func() (image.Image, error)) {
	c.copier.Start(func() (struct{}, error) {
		img, err := draw()
		if err != nil {
			return struct{}{}, err
		}
		return struct{}{}, clipboard.CopyImage(img)
	})
}

// IsCopying reports whether a copy is still running.
func (c *imageCopy) IsCopying() bool {
	return c.copier.IsLoading()
}

// ShowResult shows "Image copied" (or why it wasn't) in t once a copy finishes.
func (c *imageCopy) ShowResult(gtx layout.Context, t *toast) {
	result, ok := c.copier.Result()
	if !ok {
		return
	}
	if result.err != nil {
		slog.Error("copy image to clipboard", "error", result.err)
		t.ShowError(gtx.Now, "Couldn't copy the image.")
		return
	}
	t.Show(gtx.Now, "Image copied to clipboard", "")
}

// layoutCornerToast draws t pinned to the bottom-right corner of the space it is given.
func layoutCornerToast(gtx layout.Context, theme *material.Theme, t *toast) layout.Dimensions {
	recording := op.Record(gtx.Ops)
	dims := t.Layout(gtx, theme)
	drawn := recording.Stop()
	margin := gtx.Dp(spacingLarge)
	origin := image.Pt(gtx.Constraints.Max.X-dims.Size.X-margin, gtx.Constraints.Max.Y-dims.Size.Y-margin)
	defer op.Offset(origin).Push(gtx.Ops).Pop()
	drawn.Add(gtx.Ops)
	return layout.Dimensions{Size: gtx.Constraints.Max}
}
