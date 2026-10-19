package compose

import "image"

// Where new layers go, in canvas pixels.
const (
	placementMargin  = 24  // chat images start this far from the canvas' top-left corner
	placementGap     = 8   // between stacked chat images
	cascadeStep      = 24  // each later photo moves this far right and down
	cascadeSteps     = 8   // then the cascade starts over
	laterPhotoFactor = 0.6 // later photos fit in this share of the canvas
)

// PlacePhoto sizes and positions a new photo: the first one fits the whole canvas
// (never cropped), later ones fit 60 % of it, cascading so they don't hide each other.
func PlacePhoto(c Composition, natural image.Point) (center, size Vec) {
	canvas := Vec{float64(c.Size.X), float64(c.Size.Y)}
	photos := 0
	for _, layer := range c.Layers {
		if layer.Kind == KindPhoto {
			photos++
		}
	}
	if photos == 0 {
		return canvas.Scale(0.5), FitInside(natural, canvas)
	}
	step := float64(cascadeStep * (photos % cascadeSteps))
	return canvas.Scale(0.5).Add(Vec{step, step}), FitInside(natural, canvas.Scale(laterPhotoFactor))
}

// PlaceChat positions a new chat image at its real size, near the top-left corner,
// below the chat image added last, so a conversation reads top to bottom.
func PlaceChat(c Composition, natural image.Point) (center, size Vec) {
	size = Vec{float64(natural.X), float64(natural.Y)}
	top := float64(placementMargin)
	for i := len(c.Layers) - 1; i >= 0; i-- {
		if c.Layers[i].Kind == KindChat {
			_, hi := c.Layers[i].Bounds()
			top = hi.Y + placementGap
			break
		}
	}
	if top+size.Y > float64(c.Size.Y) {
		top = placementMargin
	}
	return Vec{placementMargin + size.X/2, top + size.Y/2}, size
}
