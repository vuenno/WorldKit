package ui

import (
	"context"
	"image"

	"gioui.org/op/paint"

	"world-kit/internal/compose"
)

// imageLoaderWorkers is how many images decode at once: decoding a 4K image briefly takes
// ~33 MB, so a few at a time keeps memory flat when many files are added together.
const imageLoaderWorkers = 2

// imageVersion is a decoded image at one size, ready to draw. Its pixels stay alive with
// it: Gio frees textures that aren't drawn in a frame and uploads them again from these.
type imageVersion struct {
	op   paint.ImageOp
	size image.Point
}

// bytes estimates the memory the version holds: its CPU copy plus the GPU texture and
// its mipmaps (a third more).
func (v imageVersion) bytes() int {
	pixels := v.size.X * v.size.Y * 4
	return pixels + pixels*4/3
}

// loadedImage is an image file decoded at the sizes a loader was asked for.
type loadedImage struct {
	natural  image.Point
	versions []imageVersion // one per requested maximum side, in request order
	err      error
}

// imageResult carries a decoded image back to the UI goroutine, with the key it was
// requested under.
type imageResult[K comparable] struct {
	key   K
	image loadedImage
}

// imageLoader decodes image files off the UI goroutine, a few at a time, into
// downscaled versions. Results come back through Receive; Reset drops all pending work,
// so results for a discarded canvas never arrive.
type imageLoader[K comparable] struct {
	ctx        context.Context
	cancel     context.CancelFunc
	slots      chan struct{}
	results    chan imageResult[K]
	invalidate func()
}

func newImageLoader[K comparable](invalidate func()) *imageLoader[K] {
	loader := &imageLoader[K]{
		slots:      make(chan struct{}, imageLoaderWorkers),
		results:    make(chan imageResult[K], 64),
		invalidate: invalidate,
	}
	loader.Reset()
	return loader
}

// Load decodes path in the background into one version per entry of maxSides (each no
// larger than that on its longer side).
func (l *imageLoader[K]) Load(key K, path string, maxSides ...int) {
	ctx := l.ctx
	go func() {
		select {
		case l.slots <- struct{}{}:
		case <-ctx.Done():
			return
		}
		defer func() { <-l.slots }()
		if ctx.Err() != nil {
			return
		}
		result := imageResult[K]{key: key, image: decodeVersions(path, maxSides)}
		select {
		case l.results <- result:
			l.invalidate()
		case <-ctx.Done():
		}
	}()
}

// decodeVersions decodes path and shrinks it to each size in turn, each from the last.
func decodeVersions(path string, maxSides []int) loadedImage {
	img, err := compose.Decode(path)
	if err != nil {
		return loadedImage{err: err}
	}
	loaded := loadedImage{natural: img.Bounds().Size()}
	var source image.Image = img
	for _, side := range maxSides {
		scaled := compose.Downscale(source, side)
		loaded.versions = append(loaded.versions, imageVersion{op: paint.NewImageOp(scaled), size: scaled.Bounds().Size()})
		source = scaled
	}
	return loaded
}

// Receive returns every result that arrived since the last call.
func (l *imageLoader[K]) Receive() []imageResult[K] {
	var received []imageResult[K]
	for {
		select {
		case result := <-l.results:
			received = append(received, result)
		default:
			return received
		}
	}
}

// Reset cancels every pending load and drops results not yet received.
func (l *imageLoader[K]) Reset() {
	if l.cancel != nil {
		l.cancel()
	}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	l.Receive()
}
