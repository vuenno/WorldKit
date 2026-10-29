// Package directwrite draws glyphs with Windows' DirectWrite, the text engine Chromium
// uses on Windows, through COM with golang.org/x/sys/windows: no CGO. It draws them the
// way Chromium's Skia does for text in the in-game chat: one glyph at a time, at a
// fractional pen position, with ClearType's red, green and blue coverage averaged into
// one gray coverage mask.
package directwrite

import (
	"fmt"
	"image"
	"math"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	dwrite                  = windows.NewLazySystemDLL("dwrite.dll")
	procDWriteCreateFactory = dwrite.NewProc("DWriteCreateFactory")

	// IDWriteFactory2 takes a grid fit mode when analyzing a glyph run.
	iidFactory2 = windows.GUID{Data1: 0x0439FC60, Data2: 0xCA44, Data3: 0x4994, Data4: [8]byte{0x8D, 0xEE, 0x3A, 0x9A, 0xF7, 0xB7, 0x32, 0xEC}}
)

// Method indexes in the COM vtables used here: IUnknown first, then each interface's own
// methods in declaration order.
const (
	methodRelease                 = 2
	methodCreateFontFileReference = 7  // IDWriteFactory
	methodCreateFontFace          = 9  // IDWriteFactory
	methodCreateGlyphRunAnalysis  = 30 // IDWriteFactory2
	methodGetAlphaTextureBounds   = 3  // IDWriteGlyphRunAnalysis
	methodCreateAlphaTexture      = 4  // IDWriteGlyphRunAnalysis
	maxMethods                    = 32

	factoryTypeShared    = 0
	fontFaceTypeTrueType = 1
	simulationsNone      = 0
	// Chromium's settings for chat text: natural rendering, outlines snapped to the pixel
	// grid vertically, ClearType coverage (three bytes a pixel), natural measuring.
	renderingModeNatural = 4
	gridFitEnabled       = 2
	antialiasClearType   = 0
	textureClearType3x1  = 1
	measuringNatural     = 0

	hresultFailureBit = 0x80000000
)

// comObject is a COM interface pointer: its first field points at the method table.
type comObject struct {
	vtable *[maxMethods]uintptr
}

// call invokes method number index on a COM object and returns its HRESULT. A float
// argument is passed as its bits: Go's Windows syscalls copy the first four arguments to
// the float registers too, and later ones go on the stack either way.
//
//go:uintptrescapes
func call(object *comObject, index int, args ...uintptr) uintptr {
	hr, _, _ := syscall.SyscallN(object.vtable[index], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	return hr
}

func release(object *comObject) {
	call(object, methodRelease)
}

func failed(hr uintptr) bool {
	return hr&hresultFailureBit != 0
}

func floatArg(f float32) uintptr {
	return uintptr(math.Float32bits(f))
}

// sharedFactory is created once: DirectWrite's shared factory is safe to use from any
// goroutine and lives as long as the process.
var sharedFactory = sync.OnceValues(func() (*comObject, error) {
	if err := procDWriteCreateFactory.Find(); err != nil {
		return nil, fmt.Errorf("load DirectWrite: %w", err)
	}
	var factory *comObject
	hr, _, _ := procDWriteCreateFactory.Call(factoryTypeShared, uintptr(unsafe.Pointer(&iidFactory2)), uintptr(unsafe.Pointer(&factory)))
	if failed(hr) || factory == nil {
		return nil, fmt.Errorf("create DirectWrite factory: HRESULT %#x", hr)
	}
	return factory, nil
})

// Font is the first face of a TrueType font file, opened in DirectWrite. It is safe for
// concurrent use.
type Font struct {
	face *comObject
}

// Open opens the font file at path.
func Open(path string) (*Font, error) {
	factory, err := sharedFactory()
	if err != nil {
		return nil, err
	}
	pathUTF16, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, fmt.Errorf("open font %q: %w", path, err)
	}
	var file *comObject
	if hr := call(factory, methodCreateFontFileReference, uintptr(unsafe.Pointer(pathUTF16)), 0, uintptr(unsafe.Pointer(&file))); failed(hr) {
		return nil, fmt.Errorf("open font file %q: HRESULT %#x", path, hr)
	}
	defer release(file)
	files := [1]*comObject{file}
	var face *comObject
	hr := call(factory, methodCreateFontFace, fontFaceTypeTrueType, 1, uintptr(unsafe.Pointer(&files[0])), 0, simulationsNone, uintptr(unsafe.Pointer(&face)))
	if failed(hr) {
		return nil, fmt.Errorf("create font face for %q: HRESULT %#x", path, hr)
	}
	return &Font{face: face}, nil
}

// Close releases the font. It must not be used afterwards.
func (f *Font) Close() {
	if f.face != nil {
		release(f.face)
		f.face = nil
	}
}

// glyphRun mirrors DWRITE_GLYPH_RUN on amd64.
type glyphRun struct {
	fontFace      *comObject
	fontEmSize    float32
	glyphCount    uint32
	glyphIndices  *uint16
	glyphAdvances *float32
	glyphOffsets  uintptr
	isSideways    int32
	bidiLevel     uint32
}

type rect struct {
	left, top, right, bottom int32
}

// GlyphMask draws glyph at sizePx pixels per em with its pen at (originX, 0); a
// fractional originX places it between pixels. The mask's bounds are pixels relative to
// the pen's whole-pixel position, y growing down from the baseline; a glyph with no ink
// (a space) has an empty mask.
func (f *Font) GlyphMask(glyph uint16, sizePx, originX float32) (*image.Alpha, error) {
	factory, err := sharedFactory()
	if err != nil {
		return nil, err
	}
	advance := float32(0)
	run := glyphRun{fontFace: f.face, fontEmSize: sizePx, glyphCount: 1, glyphIndices: &glyph, glyphAdvances: &advance}
	var analysis *comObject
	hr := call(factory, methodCreateGlyphRunAnalysis,
		uintptr(unsafe.Pointer(&run)), 0, renderingModeNatural, measuringNatural, gridFitEnabled, antialiasClearType,
		floatArg(originX), floatArg(0), uintptr(unsafe.Pointer(&analysis)))
	if failed(hr) {
		return nil, fmt.Errorf("analyze glyph %d: HRESULT %#x", glyph, hr)
	}
	defer release(analysis)
	var bounds rect
	if hr := call(analysis, methodGetAlphaTextureBounds, textureClearType3x1, uintptr(unsafe.Pointer(&bounds))); failed(hr) {
		return nil, fmt.Errorf("measure glyph %d: HRESULT %#x", glyph, hr)
	}
	mask := image.NewAlpha(image.Rect(int(bounds.left), int(bounds.top), int(bounds.right), int(bounds.bottom)))
	if len(mask.Pix) == 0 {
		return mask, nil
	}
	rgb := make([]byte, 3*len(mask.Pix))
	hr = call(analysis, methodCreateAlphaTexture, textureClearType3x1, uintptr(unsafe.Pointer(&bounds)), uintptr(unsafe.Pointer(&rgb[0])), uintptr(len(rgb)))
	if failed(hr) {
		return nil, fmt.Errorf("draw glyph %d: HRESULT %#x", glyph, hr)
	}
	for i := range mask.Pix {
		mask.Pix[i] = uint8((int(rgb[3*i]) + int(rgb[3*i+1]) + int(rgb[3*i+2])) / 3)
	}
	return mask, nil
}
