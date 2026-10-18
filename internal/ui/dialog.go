package ui

import (
	"image"
	"image/color"

	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// dialog is a modal card centered over a dimmed window. The scrim swallows clicks,
// so nothing behind the dialog can be used until it closes.
type dialog struct {
	scrim widget.Clickable
}

func (d *dialog) Layout(gtx layout.Context, theme *material.Theme, title, message string, content layout.Widget) layout.Dimensions {
	d.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		paint.FillShape(gtx.Ops, colorScrim, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	})
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(dialogWidth))
		return card(gtx, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pageTitle(theme, title).Layout),
				layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
				layout.Rigid(bodyText(theme, message).Layout),
				layout.Rigid(layout.Spacer{Height: spacingLarge}.Layout),
				layout.Rigid(content),
			)
		})
	})
}

// card draws content on a bordered surface panel with page-sized padding, filling the width.
func card(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return paddedSurface(gtx, spacingLarge, content)
}

// panel is a compact card (spacingMedium padding) for tool panels inside a page.
func panel(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return paddedSurface(gtx, spacingMedium, content)
}

func paddedSurface(gtx layout.Context, padding unit.Dp, content layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return outlinedSurface(gtx, colorSurface)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			// Stack clears Min for stacked children; restore the full width.
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(padding).Layout(gtx, content)
		}),
	)
}

// outlinedSurface fills the minimum size with a rounded panel and a 1 dp border.
// It is the background of cards, text fields and list rows.
func outlinedSurface(gtx layout.Context, fill color.NRGBA) layout.Dimensions {
	size := gtx.Constraints.Min
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(cornerRadius))
	paint.FillShape(gtx.Ops, fill, shape.Op(gtx.Ops))
	border := clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(borderWidth))}
	paint.FillShape(gtx.Ops, colorBorder, border.Op())
	return layout.Dimensions{Size: size}
}
