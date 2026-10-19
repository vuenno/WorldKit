package compose

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func nearVec(a, b Vec) bool { return near(a.X, b.X) && near(a.Y, b.Y) }

func testLayer() Layer {
	return Layer{Natural: image.Pt(100, 50), Center: Vec{200, 100}, Size: Vec{100, 50}, Opacity: 1}
}

func TestTransformMapsCorners(t *testing.T) {
	tests := []struct {
		name     string
		rotation float64
		size     Vec
		source   Vec
		want     Vec
	}{
		{"top-left unrotated", 0, Vec{100, 50}, Vec{0, 0}, Vec{150, 75}},
		{"bottom-right scaled", 0, Vec{200, 100}, Vec{100, 50}, Vec{300, 150}},
		{"center stays put when rotated", math.Pi / 3, Vec{100, 50}, Vec{50, 25}, Vec{200, 100}},
		{"quarter turn clockwise", math.Pi / 2, Vec{100, 50}, Vec{100, 25}, Vec{200, 150}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layer := testLayer()
			layer.Rotation, layer.Size = tt.rotation, tt.size
			if got := layer.Transform().Apply(tt.source); !nearVec(got, tt.want) {
				t.Errorf("Apply(%v) = %v, want %v", tt.source, got, tt.want)
			}
		})
	}
}

func TestLocalRoundTripAndContains(t *testing.T) {
	layer := testLayer()
	layer.Rotation = 0.7
	p := Vec{230, 90}
	if got := layer.FromLocal(layer.ToLocal(p)); !nearVec(got, p) {
		t.Fatalf("round trip = %v, want %v", got, p)
	}
	if !layer.Contains(layer.Center) {
		t.Error("layer should contain its center")
	}
	if layer.Contains(Vec{400, 400}) {
		t.Error("layer should not contain a far point")
	}
}

func TestLayerAtPicksTopVisible(t *testing.T) {
	bottom, top, hidden := testLayer(), testLayer(), testLayer()
	bottom.ID, top.ID, hidden.ID = 1, 2, 3
	hidden.IsHidden = true
	comp := Composition{Layers: []Layer{bottom, top, hidden}}
	if got := comp.LayerAt(Vec{200, 100}); got != 1 {
		t.Errorf("LayerAt = %d, want 1 (the top visible layer)", got)
	}
	if got := comp.LayerAt(Vec{0, 0}); got != -1 {
		t.Errorf("LayerAt outside = %d, want -1", got)
	}
}

func TestResizedKeepsOppositeSide(t *testing.T) {
	tests := []struct {
		name       string
		rotation   float64
		handle     Handle
		pointer    func(Layer) Vec
		keepAspect bool
		wantSize   Vec
	}{
		{"east edge stretches width", 0, HandleEast, func(l Layer) Vec { return Vec{300, 100} }, false, Vec{150, 50}},
		{"corner keeps aspect", 0, HandleSouthEast, func(l Layer) Vec { return Vec{350, 110} }, true, Vec{200, 100}},
		{"corner free aspect", 0, HandleSouthEast, func(l Layer) Vec { return Vec{350, 110} }, false, Vec{200, 35}},
		{"rotated corner", math.Pi / 2, HandleSouthEast, func(l Layer) Vec { return l.FromLocal(Vec{100, 75}) }, false, Vec{150, 100}},
		{"minimum size", 0, HandleEast, func(l Layer) Vec { return Vec{0, 100} }, false, Vec{8, 50}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start := testLayer()
			start.Rotation = tt.rotation
			dx, dy := tt.handle.Direction()
			opposite := start.FromLocal(Vec{-float64(dx) * start.Size.X / 2, -float64(dy) * start.Size.Y / 2})
			got := Resized(start, tt.handle, tt.pointer(start), tt.keepAspect, 8)
			if !nearVec(got.Size, tt.wantSize) {
				t.Errorf("size = %v, want %v", got.Size, tt.wantSize)
			}
			after := got.FromLocal(Vec{-float64(dx) * got.Size.X / 2, -float64(dy) * got.Size.Y / 2})
			if !nearVec(after, opposite) {
				t.Errorf("opposite side moved from %v to %v", opposite, after)
			}
		})
	}
}

func TestRotated(t *testing.T) {
	start := testLayer()
	grab := Vec{200, 50}    // straight above the center
	pointer := Vec{250, 99} // almost straight right: about +90°
	if got := Rotated(start, grab, pointer, true); !near(got.Rotation, math.Pi/2) {
		t.Errorf("snapped rotation = %v, want π/2", got.Rotation)
	}
	free := Rotated(start, grab, pointer, false)
	if near(free.Rotation, math.Pi/2) || math.Abs(free.Rotation-math.Pi/2) > 0.05 {
		t.Errorf("free rotation = %v, want just under π/2", free.Rotation)
	}
}

func TestHandleAt(t *testing.T) {
	layer := testLayer()
	if got := layer.HandleAt(Vec{251, 126}, 5, 20); got != HandleSouthEast {
		t.Errorf("HandleAt corner = %v, want south-east", got)
	}
	if got := layer.HandleAt(Vec{200, 55}, 5, 20); got != HandleRotate {
		t.Errorf("HandleAt above = %v, want rotate", got)
	}
	if got := layer.HandleAt(Vec{200, 100}, 5, 20); got != HandleNone {
		t.Errorf("HandleAt center = %v, want none", got)
	}
}

func TestSnapped(t *testing.T) {
	layer := testLayer()
	layer.Center = Vec{503, 100} // center 3 px right of the canvas middle
	got, guides := Snapped(layer, Vec{1000, 600}, 6)
	if !near(got.Center.X, 500) || !near(guides.X, 500) {
		t.Errorf("x: center %v guide %v, want 500", got.Center.X, guides.X)
	}
	if !math.IsNaN(guides.Y) || !near(got.Center.Y, 100) {
		t.Errorf("y should not snap: center %v guide %v", got.Center.Y, guides.Y)
	}
}

func TestMoveLayerAndResize(t *testing.T) {
	comp := Composition{Size: image.Pt(100, 100), Layers: []Layer{{ID: 1}, {ID: 2}, {ID: 3}}}
	comp.MoveLayer(0, 2)
	if comp.Layers[2].ID != 1 || comp.Layers[0].ID != 2 {
		t.Errorf("order after move = %v", comp.Layers)
	}
	comp.Layers[0].Center = Vec{50, 50}
	comp.Resize(image.Pt(200, 120))
	if !nearVec(comp.Layers[0].Center, Vec{100, 60}) {
		t.Errorf("center after canvas resize = %v, want {100 60}", comp.Layers[0].Center)
	}
}

func TestHistory(t *testing.T) {
	var history History
	first := Composition{Layers: []Layer{{ID: 1}}}
	second := Composition{Layers: []Layer{{ID: 1}, {ID: 2}}}
	history.Remember(first)
	undone, ok := history.Undo(second)
	if !ok || len(undone.Layers) != 1 {
		t.Fatalf("Undo = %v, %v", undone, ok)
	}
	redone, ok := history.Redo(undone)
	if !ok || len(redone.Layers) != 2 {
		t.Fatalf("Redo = %v, %v", redone, ok)
	}
	if history.CanRedo() {
		t.Error("nothing left to redo")
	}
}

// solidOpener returns a uniform image of each layer's natural size.
func solidOpener(c color.RGBA, size image.Point) Opener {
	return func(string) (image.Image, error) {
		img := image.NewRGBA(image.Rectangle{Max: size})
		for i := 0; i < len(img.Pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = c.R, c.G, c.B, c.A
		}
		return img, nil
	}
}

func TestRenderBlendsInLinearLight(t *testing.T) {
	tests := []struct {
		name    string
		opacity float64
		want    uint8
	}{
		{"half white over black is 188, as Gio shows it", 0.5, 188},
		{"opaque white stays white", 1, 255},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			layer := Layer{Natural: image.Pt(8, 8), Center: Vec{4, 4}, Size: Vec{8, 8}, Opacity: tt.opacity}
			comp := Composition{Size: image.Pt(8, 8), Background: BackgroundBlack, Layers: []Layer{layer}}
			img, err := Render(context.Background(), comp, solidOpener(color.RGBA{255, 255, 255, 255}, image.Pt(8, 8)))
			if err != nil {
				t.Fatal(err)
			}
			if got := img.RGBAAt(4, 4); got.R != tt.want || got.A != 255 {
				t.Errorf("pixel = %v, want R %d", got, tt.want)
			}
		})
	}
}

func TestRenderKeepsTransparencyAndExactPixels(t *testing.T) {
	layer := Layer{Natural: image.Pt(4, 4), Center: Vec{4, 4}, Size: Vec{4, 4}, Opacity: 1}
	comp := Composition{Size: image.Pt(10, 10), Layers: []Layer{layer}}
	img, err := Render(context.Background(), comp, solidOpener(color.RGBA{200, 100, 50, 255}, image.Pt(4, 4)))
	if err != nil {
		t.Fatal(err)
	}
	if got := img.RGBAAt(0, 0); got.A != 0 {
		t.Errorf("outside the layer = %v, want transparent", got)
	}
	if got := img.RGBAAt(3, 3); got != (color.RGBA{200, 100, 50, 255}) {
		t.Errorf("a whole-pixel layer must keep its colors exactly, got %v", got)
	}
}

func TestRenderReportsMissingSource(t *testing.T) {
	layer := testLayer()
	layer.Name = "photo.png"
	comp := Composition{Size: image.Pt(400, 200), Layers: []Layer{layer}}
	failing := func(string) (image.Image, error) { return nil, errors.New("gone") }
	if _, err := Render(context.Background(), comp, failing); err == nil {
		t.Fatal("want an error for a missing source")
	}
}

func TestEncodeFormats(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 64, 32))
	for y := range 32 {
		for x := range 64 {
			img.SetRGBA(x, y, color.RGBA{uint8(x * 4), uint8(y * 8), 128, 255})
		}
	}
	tests := []struct {
		name   string
		format Format
	}{
		{"png lossless", Format{Type: FileTypePNG, IsLossless: true}},
		{"png reduced, dithered", Format{Type: FileTypePNG, Quality: 80}},
		{"png reduced, flat", Format{Type: FileTypePNG, Quality: 20}},
		{"jpeg", Format{Type: FileTypeJPEG, Quality: 92}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := Encode(&buf, img, tt.format); err != nil {
				t.Fatal(err)
			}
			if _, _, err := image.Decode(&buf); err != nil {
				t.Fatalf("decode result: %v", err)
			}
		})
	}
}

func TestQuantizeKeepsAlphaAndLimitsColors(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 32, 32))
	for y := range 32 {
		for x := range 32 {
			if x < 16 {
				img.SetRGBA(x, y, color.RGBA{uint8(x * 16), uint8(y * 8), 0, 255})
			}
		}
	}
	for _, dither := range []bool{false, true} {
		got := Quantize(img, 16, dither)
		if len(got.Palette) > 16 {
			t.Errorf("palette has %d colors, want ≤ 16", len(got.Palette))
		}
		for y := range 32 {
			for x := 16; x < 32; x++ {
				if _, _, _, a := got.At(x, y).RGBA(); a != 0 {
					t.Fatalf("dither %v: transparent pixel (%d, %d) came out with alpha %d", dither, x, y, a)
				}
			}
		}
	}
	got := Quantize(img, 16, true)
	var buf bytes.Buffer
	if err := png.Encode(&buf, got); err != nil {
		t.Fatal(err)
	}
}

func TestPaletteSize(t *testing.T) {
	for quality, want := range map[int]int{10: 28, 50: 142, 90: 256, 100: 256} {
		if got := paletteSize(quality); got != want {
			t.Errorf("paletteSize(%d) = %d, want %d", quality, got, want)
		}
	}
}

func TestFileTypeOf(t *testing.T) {
	for path, want := range map[string]FileType{`C:\a\b.PNG`: FileTypePNG, "x.jpeg": FileTypeJPEG, "y.JPG": FileTypeJPEG} {
		if got, ok := FileTypeOf(path); !ok || got != want {
			t.Errorf("FileTypeOf(%q) = %q, %v", path, got, ok)
		}
	}
	if _, ok := FileTypeOf("z.gif"); ok {
		t.Error("gif is not an export type")
	}
}

func TestPlacement(t *testing.T) {
	comp := Composition{Size: image.Pt(1920, 1080)}
	center, size := PlacePhoto(comp, image.Pt(3840, 1600))
	if !nearVec(center, Vec{960, 540}) || !nearVec(size, Vec{1920, 800}) {
		t.Errorf("first photo = %v %v, want centered and fitted", center, size)
	}
	comp.Layers = append(comp.Layers, Layer{Kind: KindPhoto, Center: center, Size: size})
	center, size = PlacePhoto(comp, image.Pt(1920, 1080))
	if !nearVec(size, Vec{1152, 648}) || !nearVec(center, Vec{984, 564}) {
		t.Errorf("second photo = %v %v, want 60%% and cascaded", center, size)
	}
	center, size = PlaceChat(comp, image.Pt(400, 100))
	if !nearVec(center, Vec{224, 74}) || !nearVec(size, Vec{400, 100}) {
		t.Errorf("first chat = %v %v", center, size)
	}
	comp.Layers = append(comp.Layers, Layer{Kind: KindChat, Center: center, Size: size})
	if center, _ = PlaceChat(comp, image.Pt(400, 100)); !nearVec(center, Vec{224, 182}) {
		t.Errorf("second chat center = %v, want below the first", center)
	}
}

func TestQuantizeFullyTransparent(t *testing.T) {
	got := Quantize(image.NewRGBA(image.Rect(0, 0, 4, 4)), 200, true)
	if _, _, _, a := got.At(1, 1).RGBA(); a != 0 || len(got.Palette) != 1 {
		t.Errorf("palette %v, alpha %d: want one transparent entry", got.Palette, a)
	}
}

func TestQuantizeKeepsEmptyPixelsBesideFaintOnes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 2))
	for x := range 8 {
		img.SetRGBA(x, 0, color.RGBA{1, 1, 1, 10}) // a faint dark edge, scanned first
	}
	got := Quantize(img, 16, true)
	for x := range 8 {
		if _, _, _, a := got.At(x, 1).RGBA(); a != 0 {
			t.Fatalf("empty pixel (%d, 1) came out with alpha %d", x, a)
		}
	}
}
