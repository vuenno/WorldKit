package ui

import (
	"image"

	"world-kit/internal/compose"
)

// Versions the editor decodes each layer's image at, in imageLoader request order.
const (
	versionPreview = iota // drawn on the canvas (≤ previewMaxSide)
	versionThumb          // the layer's row in the layers list
)

// editorDoc is the Screenshot Editor's working state, shared by the canvas, the inspector
// and the layers list: the composition, its undo history, the selected layer and each
// layer's decoded images. It lives as long as the app, so leaving the page keeps the work.
type editorDoc struct {
	comp       compose.Composition
	hasCanvas  bool
	history    compose.History
	selectedID int                 // 0: nothing selected
	lastID     int                 // IDs are never reused, so results for a discarded canvas are recognized
	firstID    int                 // the first ID on the current canvas
	images     map[int]loadedImage // by layer ID, as decoding finishes
	loader     *imageLoader[int]
}

func newEditorDoc(invalidate func()) *editorDoc {
	return &editorDoc{images: map[int]loadedImage{}, loader: newImageLoader[int](invalidate)}
}

// startCanvas replaces everything with an empty canvas of size.
func (d *editorDoc) startCanvas(size image.Point) {
	d.loader.Reset()
	d.comp = compose.Composition{Size: compose.ClampCanvasSize(size)}
	d.hasCanvas = true
	d.history.Clear()
	d.selectedID = 0
	d.firstID = d.lastID + 1
	clear(d.images)
}

// close goes back to the template picker, dropping the canvas.
func (d *editorDoc) close() {
	d.startCanvas(image.Point{})
	d.hasCanvas = false
}

// receiveImages stores decoded images for this canvas's layers. Images of deleted layers
// stay until the canvas is replaced, so undo brings them back at once.
func (d *editorDoc) receiveImages() {
	for _, result := range d.loader.Receive() {
		if result.key >= d.firstID {
			d.images[result.key] = result.image
		}
	}
}

// addLayer puts layer on top of the others, selects it and starts decoding its image.
func (d *editorDoc) addLayer(layer compose.Layer) {
	d.lastID++
	layer.ID = d.lastID
	d.comp.Layers = append(d.comp.Layers, layer)
	d.selectedID = layer.ID
	d.loader.Load(layer.ID, layer.Source, previewMaxSide, thumbnailMaxSide)
}

// remember keeps the current state for undo, before a change.
func (d *editorDoc) remember() {
	d.history.Remember(d.comp)
}

// change remembers the current state, then applies update.
func (d *editorDoc) change(update func(comp *compose.Composition)) {
	d.remember()
	update(&d.comp)
}

func (d *editorDoc) undo() {
	if comp, ok := d.history.Undo(d.comp); ok {
		d.comp = comp
		d.keepSelectionValid()
	}
}

func (d *editorDoc) redo() {
	if comp, ok := d.history.Redo(d.comp); ok {
		d.comp = comp
		d.keepSelectionValid()
	}
}

func (d *editorDoc) keepSelectionValid() {
	if d.comp.IndexOf(d.selectedID) < 0 {
		d.selectedID = 0
	}
}

// selectedIndex returns the selected layer's position, or -1.
func (d *editorDoc) selectedIndex() int {
	if d.selectedID == 0 {
		return -1
	}
	return d.comp.IndexOf(d.selectedID)
}

// selected returns the selected layer.
func (d *editorDoc) selected() (compose.Layer, bool) {
	index := d.selectedIndex()
	if index < 0 {
		return compose.Layer{}, false
	}
	return d.comp.Layers[index], true
}

// setSelected replaces the selected layer without remembering (during a drag, which
// remembered once when it started).
func (d *editorDoc) setSelected(layer compose.Layer) {
	if index := d.selectedIndex(); index >= 0 {
		d.comp.Layers[index] = layer
	}
}

// changeSelected remembers the current state, then applies update to the selected layer.
func (d *editorDoc) changeSelected(update func(layer *compose.Layer)) {
	index := d.selectedIndex()
	if index < 0 {
		return
	}
	d.remember()
	update(&d.comp.Layers[index])
}

// preview returns the layer's canvas image, if decoded.
func (d *editorDoc) preview(id int) (imageVersion, bool) {
	loaded, ok := d.images[id]
	if !ok || loaded.err != nil || len(loaded.versions) <= versionPreview {
		return imageVersion{}, false
	}
	return loaded.versions[versionPreview], true
}

// imageBytes estimates the memory every decoded layer image holds.
func (d *editorDoc) imageBytes() int {
	total := 0
	for _, loaded := range d.images {
		for _, version := range loaded.versions {
			total += version.bytes()
		}
	}
	return total
}

// missingLayer returns the name of a layer whose image couldn't be read, if any.
func (d *editorDoc) missingLayer() (string, bool) {
	for _, layer := range d.comp.Layers {
		if loaded, ok := d.images[layer.ID]; ok && loaded.err != nil && !layer.IsHidden {
			return layer.Name, true
		}
	}
	return "", false
}
