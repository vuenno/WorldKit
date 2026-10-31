package ui

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"

	"gioui.org/f32"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
)

// strokeIcon is a line icon drawn on a 24×24 grid, in the style of the design's icons
// (rounded 1.8 px strokes, no fills). It is written in a small subset of SVG path syntax,
// absolute coordinates only:
//
//	M x y | L x y | H x | V y | Q cx cy x y | Z      as in SVG
//	O cx cy r                                        a circle
//	R x y w h r                                      a rectangle with corner radius r
type strokeIcon string

// Icons used across WorldKit.
const (
	iconArrowLeft    strokeIcon = "M19 12 H5 M11 6 L5 12 L11 18"
	iconSearch       strokeIcon = "O 11 11 7 M20 20 L16.5 16.5"
	iconCopy         strokeIcon = "R 9 9 12 12 2 M5 15 V5 Q5 3 7 3 H17"
	iconImage        strokeIcon = "R 3 3 18 18 2 O 9 9 2 M21 15 L16.5 10.5 L6 21"
	iconFolder       strokeIcon = "M3 7 Q3 5 5 5 H9 L11 7 H19 Q21 7 21 9 V17 Q21 19 19 19 H5 Q3 19 3 17 Z"
	iconTrash        strokeIcon = "M3 6 H21 M8 6 V4 H16 V6 M6 6 L7 20 H17 L18 6"
	iconClose        strokeIcon = "M6 6 L18 18 M18 6 L6 18"
	iconChevronRight strokeIcon = "M9 6 L15 12 L9 18"
	iconChevronDown  strokeIcon = "M6 9 L12 15 L18 9"
	iconPanelLeft    strokeIcon = "R 3 3 18 18 2 M9 3 V21"
	iconFileText     strokeIcon = "M14 2 H6 Q4 2 4 4 V20 Q4 22 6 22 H18 Q20 22 20 20 V8 Z M14 2 V8 H20 M16 13 H8 M16 17 H8"
	iconSliders      strokeIcon = "M4 21 V14 M4 10 V3 M12 21 V12 M12 8 V3 M20 21 V16 M20 12 V3 M1 14 H7 M9 8 H15 M17 16 H23"
	iconUsers        strokeIcon = "O 9 8 4 M2 21 Q2 14 9 14 Q16 14 16 21 M16 4.5 Q19 5 19 8 Q19 11 16 11.5 M18 14.5 Q22 15.5 22 21"
	iconCheck        strokeIcon = "M20 6 L9 17 L4 12"
	iconDownload     strokeIcon = "M12 4 V15 M7 10 L12 15 L17 10 M5 20 H19"
	iconArrowRight   strokeIcon = "M5 12 H19 M13 6 L19 12 L13 18"
	iconPencil       strokeIcon = "M4 20 V16 L15 5 L19 9 L8 20 Z M13 7 L17 11"
	iconItalic       strokeIcon = "M10 4 H19 M5 20 H14 M15 4 L9 20"
	iconMinus        strokeIcon = "M5 12 H19"
	iconPlus         strokeIcon = "M12 5 V19 M5 12 H19"
	iconLayers       strokeIcon = "M12 3 L21 8 L12 13 L3 8 Z M3 12 L12 17 L21 12 M3 16 L12 21 L21 16"
	iconImages       strokeIcon = "R 7 3 14 14 2 O 12 8 1.5 M21 13 L17 9 L10 17 M3 7 V19 Q3 21 5 21 H17"
	iconChat         strokeIcon = "M4 5 Q4 3 6 3 H18 Q20 3 20 5 V15 Q20 17 18 17 H9 L4 21 Z M8 8 H16 M8 12 H13"
	iconUndo         strokeIcon = "M9 14 L4 9 L9 4 M4 9 H15 Q20 9 20 14 Q20 19 15 19 H11"
	iconRedo         strokeIcon = "M15 14 L20 9 L15 4 M20 9 H9 Q4 9 4 14 Q4 19 9 19 H13"
	iconExport       strokeIcon = "M12 3 V15 M7 10 L12 15 L17 10 M4 17 V19 Q4 21 6 21 H18 Q20 21 20 19 V17"
	iconFit          strokeIcon = "M4 9 V4 H9 M15 4 H20 V9 M20 15 V20 H15 M9 20 H4 V15"
	iconEye          strokeIcon = "M2 12 Q12 2 22 12 Q12 22 2 12 Z O 12 12 3"
	iconEyeOff       strokeIcon = "M2 12 Q12 2 22 12 Q12 22 2 12 Z O 12 12 3 M4 4 L20 20"
	iconLock         strokeIcon = "R 5 11 14 10 2 M8 11 V8 Q8 4 12 4 Q16 4 16 8 V11"
	iconUnlock       strokeIcon = "R 5 11 14 10 2 M8 11 V8 Q8 4 12 4 Q16 4 16 6"
	iconRotate       strokeIcon = "M21 4 V10 H15 M21 10 Q18 4 12 4 Q4 4 4 12 Q4 20 12 20 Q18 20 20 15"
	iconChevronUp    strokeIcon = "M6 15 L12 9 L18 15"
	iconAlert        strokeIcon = "M12 3 L22 20 H2 Z M12 9 V14 M12 17 V17.5"
	iconGrip         strokeIcon = "O 9 6 0.6 O 15 6 0.6 O 9 12 0.6 O 15 12 0.6 O 9 18 0.6 O 15 18 0.6"
	iconMerge        strokeIcon = "M6 3 V8 L12 14 V21 M18 3 V8 L12 14"
)

// iconStrokeWidth is the stroke width on the 24-unit grid.
const iconStrokeWidth = 1.8

// Layout draws the icon as a size×size square in c.
func (i strokeIcon) Layout(gtx layout.Context, size unit.Dp, c color.NRGBA) layout.Dimensions {
	side := gtx.Dp(size)
	scale := float32(side) / 24
	drawStrokePath(gtx, string(i), scale, max(1, iconStrokeWidth*scale), c)
	return layout.Dimensions{Size: image.Pt(side, side)}
}

// drawStrokePath strokes path (in the syntax above), scaled from grid units to pixels.
func drawStrokePath(gtx layout.Context, path string, scale, width float32, c color.NRGBA) {
	var p clip.Path
	p.Begin(gtx.Ops)
	if err := appendIconPath(&p, path, scale); err != nil {
		panic(err) // icon paths are constants, so a bad one is a programming error
	}
	paint.FillShape(gtx.Ops, c, clip.Stroke{Path: p.End(), Width: width}.Op())
}

// appendIconPath adds the shapes in path to p.
func appendIconPath(p *clip.Path, path string, scale float32) error {
	tokens := strings.Fields(strings.NewReplacer("M", " M ", "L", " L ", "H", " H ", "V", " V ",
		"Q", " Q ", "Z", " Z ", "O", " O ", "R", " R ").Replace(path))
	parser := iconPathParser{p: p, scale: scale, tokens: tokens}
	for parser.hasMore() {
		if err := parser.command(); err != nil {
			return fmt.Errorf("icon path %q: %w", path, err)
		}
	}
	return nil
}

type iconPathParser struct {
	p      *clip.Path
	scale  float32
	tokens []string
	pos    int
	pen    f32.Point // in grid units
	start  f32.Point // where the current subpath began, for Z
}

func (r *iconPathParser) hasMore() bool {
	return r.pos < len(r.tokens)
}

func (r *iconPathParser) command() error {
	name := r.tokens[r.pos]
	r.pos++
	args, err := r.numbers(iconCommandArgs[name])
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	switch name {
	case "M":
		r.pen, r.start = f32.Pt(args[0], args[1]), f32.Pt(args[0], args[1])
		r.p.MoveTo(r.px(r.pen))
	case "L":
		r.lineTo(f32.Pt(args[0], args[1]))
	case "H":
		r.lineTo(f32.Pt(args[0], r.pen.Y))
	case "V":
		r.lineTo(f32.Pt(r.pen.X, args[0]))
	case "Q":
		r.pen = f32.Pt(args[2], args[3])
		r.p.QuadTo(r.px(f32.Pt(args[0], args[1])), r.px(r.pen))
	case "Z":
		r.lineTo(r.start)
	case "O":
		r.circle(f32.Pt(args[0], args[1]), args[2])
	case "R":
		r.roundedRect(args[0], args[1], args[2], args[3], args[4])
	default:
		return fmt.Errorf("unknown command %q", name)
	}
	return nil
}

var iconCommandArgs = map[string]int{"M": 2, "L": 2, "H": 1, "V": 1, "Q": 4, "Z": 0, "O": 3, "R": 5}

func (r *iconPathParser) numbers(count int) ([]float32, error) {
	if r.pos+count > len(r.tokens) {
		return nil, fmt.Errorf("want %d numbers", count)
	}
	values := make([]float32, count)
	for i := range values {
		value, err := strconv.ParseFloat(r.tokens[r.pos], 32)
		if err != nil {
			return nil, err
		}
		values[i] = float32(value)
		r.pos++
	}
	return values, nil
}

func (r *iconPathParser) px(point f32.Point) f32.Point {
	return point.Mul(r.scale)
}

func (r *iconPathParser) lineTo(point f32.Point) {
	r.pen = point
	r.p.LineTo(r.px(point))
}

func (r *iconPathParser) circle(center f32.Point, radius float32) {
	r.p.MoveTo(r.px(center.Add(f32.Pt(radius, 0))))
	r.p.ArcTo(r.px(center), r.px(center), 2*math.Pi)
}

func (r *iconPathParser) roundedRect(x, y, w, h, radius float32) {
	r.p.MoveTo(r.px(f32.Pt(x+radius, y)))
	r.p.LineTo(r.px(f32.Pt(x+w-radius, y)))
	r.p.QuadTo(r.px(f32.Pt(x+w, y)), r.px(f32.Pt(x+w, y+radius)))
	r.p.LineTo(r.px(f32.Pt(x+w, y+h-radius)))
	r.p.QuadTo(r.px(f32.Pt(x+w, y+h)), r.px(f32.Pt(x+w-radius, y+h)))
	r.p.LineTo(r.px(f32.Pt(x+radius, y+h)))
	r.p.QuadTo(r.px(f32.Pt(x, y+h)), r.px(f32.Pt(x, y+h-radius)))
	r.p.LineTo(r.px(f32.Pt(x, y+radius)))
	r.p.QuadTo(r.px(f32.Pt(x, y)), r.px(f32.Pt(x+radius, y)))
}
