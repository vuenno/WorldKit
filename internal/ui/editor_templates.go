package ui

import (
	"fmt"
	"image"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/compose"
	"world-kit/internal/settings"
)

// canvasTemplate is a ready-made canvas size.
type canvasTemplate struct {
	name string
	size image.Point
}

// canvasTemplates are the sizes offered first: the common game resolutions, then social
// formats. Full HD is what most players capture at.
var canvasTemplates = []canvasTemplate{
	{"Full HD", image.Pt(1920, 1080)},
	{"1440p", image.Pt(2560, 1440)},
	{"4K UHD", image.Pt(3840, 2160)},
	{"HD", image.Pt(1280, 720)},
	{"Square", image.Pt(1080, 1080)},
	{"Portrait", image.Pt(1080, 1350)},
	{"Story", image.Pt(1080, 1920)},
}

// templateChoice is what the user picked on the template screen.
type templateChoice struct {
	size        image.Point
	preset      string // the preset the size came from, if any
	isFromImage bool   // "Start from an image…": the size comes from the image picked next
}

// templatePicker is the editor's first screen: pick a canvas size from the templates, the
// saved presets or typed numbers, or start from an image's own size.
type templatePicker struct {
	templateClicks []widget.Clickable
	presetClicks   []widget.Clickable
	templateHovers []hoverArea
	presetHovers   []hoverArea
	customSize     image.Point
	customWidth    *numberField
	customHeight   *numberField
	create         widget.Clickable
	fromImage      widget.Clickable
	list           widget.List
}

func newTemplatePicker() *templatePicker {
	return &templatePicker{
		templateClicks: make([]widget.Clickable, len(canvasTemplates)),
		templateHovers: make([]hoverArea, len(canvasTemplates)),
		customSize:     canvasTemplates[0].size,
		customWidth:    newNumberField(),
		customHeight:   newNumberField(),
		list:           widget.List{List: layout.List{Axis: layout.Vertical}},
	}
}

// Update handles the clicks and returns the user's choice.
func (p *templatePicker) Update(gtx layout.Context, presets []settings.ExportPreset) (templateChoice, bool) {
	for len(p.presetClicks) < len(presets) {
		p.presetClicks = append(p.presetClicks, widget.Clickable{})
		p.presetHovers = append(p.presetHovers, hoverArea{})
	}
	if w, ok := p.customWidth.Update(gtx, float64(p.customSize.X), 0); ok {
		p.customSize = compose.ClampCanvasSize(image.Pt(int(w), p.customSize.Y))
	}
	if h, ok := p.customHeight.Update(gtx, float64(p.customSize.Y), 0); ok {
		p.customSize = compose.ClampCanvasSize(image.Pt(p.customSize.X, int(h)))
	}
	if p.fromImage.Clicked(gtx) {
		return templateChoice{isFromImage: true}, true
	}
	if p.create.Clicked(gtx) {
		return templateChoice{size: p.customSize}, true
	}
	for i, template := range canvasTemplates {
		if p.templateClicks[i].Clicked(gtx) {
			return templateChoice{size: template.size}, true
		}
	}
	for i, preset := range presets {
		if p.presetClicks[i].Clicked(gtx) {
			return templateChoice{size: image.Pt(preset.Width, preset.Height), preset: preset.Name}, true
		}
	}
	return templateChoice{}, false
}

// Layout draws the header, the template tiles, the custom size row and the presets.
func (p *templatePicker) Layout(gtx layout.Context, theme *material.Theme, presets []settings.ExportPreset) layout.Dimensions {
	items := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return pageHeader(gtx, theme, "Screenshot Editor",
				"Lay chat images over your game screenshots. Pick a canvas size to start.",
				secondaryButton(&p.fromImage, "Start from an image…").withIcon(iconImage).Widget(theme))
		},
		sectionHeading(theme, "Canvas size"),
		func(gtx layout.Context) layout.Dimensions {
			tiles := make([]templateTile, len(canvasTemplates))
			for i, template := range canvasTemplates {
				tiles[i] = templateTile{click: &p.templateClicks[i], hover: &p.templateHovers[i], name: template.name, size: template.size}
			}
			return layoutTileGrid(gtx, theme, tiles)
		},
		sectionHeading(theme, "Custom size"),
		func(gtx layout.Context) layout.Dimensions { return p.layoutCustomRow(gtx, theme) },
	}
	if len(presets) > 0 {
		items = append(items, sectionHeading(theme, "Your presets"), func(gtx layout.Context) layout.Dimensions {
			tiles := make([]templateTile, len(presets))
			for i, preset := range presets {
				tiles[i] = templateTile{
					click: &p.presetClicks[i], hover: &p.presetHovers[i], name: preset.Name,
					size: image.Pt(preset.Width, preset.Height), detail: presetFormat(preset).Describe(),
				}
			}
			return layoutTileGrid(gtx, theme, tiles)
		})
	}
	items = append(items, layout.Spacer{Height: spacingXLarge}.Layout)
	inset := layout.Inset{Left: spacingPage, Right: spacingPage, Top: spacingSmall}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return styledList(theme, &p.list).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
			return items[index](gtx)
		})
	})
}

// sectionHeading is a group label with the page's section spacing.
func sectionHeading(theme *material.Theme, text string) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions {
		inset := layout.Inset{Top: spacingXLarge, Bottom: spacingMedium, Left: spacingTiny}
		return inset.Layout(gtx, groupLabel(theme, text).Layout)
	}
}

func (p *templatePicker) layoutCustomRow(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	field := func(f *numberField, prefix string) layout.FlexChild {
		return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Max.X = gtx.Dp(inspectorFieldWidth + spacingXLarge)
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return f.Layout(gtx, theme, prefix, "px")
		})
	}
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		field(p.customWidth, "W"),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(secondaryText(theme, "×").Layout),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		field(p.customHeight, "H"),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Rigid(secondaryButton(&p.create, "Create canvas").withHeight(controlHeightSmall).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			label := caption(theme, fmt.Sprintf("%d to %d px a side.", compose.MinCanvasSide, compose.MaxCanvasSide))
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
	)
}

// templateTile is one canvas size to pick: its shape at its true aspect ratio, its name
// and its size in pixels (and, for a preset, its export format).
type templateTile struct {
	click  *widget.Clickable
	hover  *hoverArea
	name   string
	size   image.Point
	detail string
}

// layoutTileGrid lays tiles out in as many columns as fit, like the screenshot gallery.
func layoutTileGrid(gtx layout.Context, theme *material.Theme, tiles []templateTile) layout.Dimensions {
	gap := gtx.Dp(spacingLarge)
	columns := galleryColumns(gtx.Constraints.Max.X, gtx.Dp(templateTileMinWidth), gap)
	width := (gtx.Constraints.Max.X - gap*(columns-1)) / columns
	var rows []layout.FlexChild
	for start := 0; start < len(tiles); start += columns {
		row := tiles[start:min(start+columns, len(tiles))]
		if start > 0 {
			rows = append(rows, layout.Rigid(layout.Spacer{Height: spacingLarge}.Layout))
		}
		rows = append(rows, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			var children []layout.FlexChild
			for i, tile := range row {
				if i > 0 {
					children = append(children, layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout))
				}
				children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints = layout.Constraints{Min: image.Pt(width, 0), Max: image.Pt(width, gtx.Constraints.Max.Y)}
					return tile.Layout(gtx, theme)
				}))
			}
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
		}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, rows...)
}

func (t templateTile) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return t.hover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return t.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			border := colorDivider
			if t.hover.isHovered {
				border = colorBorderStrong
			}
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(func(gtx layout.Context) layout.Dimensions {
					dims := outlinedSurface(gtx, colorSurface, border, radiusCard)
					defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
					pointer.CursorPointer.Add(gtx.Ops)
					return dims
				}),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = gtx.Constraints.Max.X
					return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return t.layoutContent(gtx, theme)
					})
				}),
			)
		})
	})
}

func (t templateTile) layoutContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	details := fmt.Sprintf("%d × %d", t.size.X, t.size.Y)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.layoutShape(gtx) }),
		layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := styledLabel(theme, textSizeBody, font.SemiBold, colorTextPrimary, t.name)
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: spacingTiny / 2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := monoText(theme, textSizeCaption, details)
			label.Color = colorTextSecondary
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if t.detail == "" {
				return layout.Dimensions{}
			}
			label := caption(theme, t.detail)
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
	)
}

// layoutShape outlines a rectangle of the template's aspect ratio, centered in the tile.
func (t templateTile) layoutShape(gtx layout.Context) layout.Dimensions {
	area := image.Pt(gtx.Constraints.Max.X, gtx.Dp(templateShapeHeight))
	shape := compose.FitInside(t.size, compose.Vec{X: float64(area.X), Y: float64(area.Y)})
	w, h := float32(shape.X), float32(shape.Y)
	left, top := (float32(area.X)-w)/2, (float32(area.Y)-h)/2
	stroke := float32(gtx.Dp(templateShapeStroke))
	bounds := image.Rect(int(left+stroke), int(top+stroke), int(left+w-stroke), int(top+h-stroke))
	rect := clip.UniformRRect(bounds, gtx.Dp(radiusSmall/2))
	strokeColor := colorTextTertiary
	if t.hover.isHovered {
		strokeColor = colorPrimary
		paint.FillShape(gtx.Ops, withOpacity(colorPrimary, liveTintOpacity), rect.Op(gtx.Ops))
	}
	paint.FillShape(gtx.Ops, strokeColor, clip.Stroke{Path: rect.Path(gtx.Ops), Width: stroke}.Op())
	return layout.Dimensions{Size: area}
}

// presetFormat is a saved preset's export format.
func presetFormat(preset settings.ExportPreset) compose.Format {
	return compose.Format{Type: compose.FileType(preset.Format), Quality: preset.Quality, IsLossless: preset.IsLossless}.Normalized()
}
