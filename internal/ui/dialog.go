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
	width unit.Dp // the most the card may take; zero: dialogWidth
}

func (d *dialog) Layout(gtx layout.Context, theme *material.Theme, title, message string, content layout.Widget) layout.Dimensions {
	d.scrim.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := gtx.Constraints.Max
		paint.FillShape(gtx.Ops, colorScrim, clip.Rect{Max: size}.Op())
		return layout.Dimensions{Size: size}
	})
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		width := d.width
		if width == 0 {
			width = dialogWidth
		}
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(width))
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return card(gtx, spacingXLarge, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(screenTitle(theme, title).Layout),
				layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := bodyText(theme, message)
					label.Color = colorTextSecondary
					return label.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: spacingXLarge}.Layout),
				layout.Rigid(content),
			)
		})
	})
}

// card draws content on a bordered surface panel with the given padding, filling the width.
func card(gtx layout.Context, padding unit.Dp, content layout.Widget) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return outlinedSurface(gtx, colorSurface, colorDivider, radiusCard)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			// Stack clears Min for stacked children; restore the full width.
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(padding).Layout(gtx, content)
		}),
	)
}

// outlinedSurface fills the minimum size with a rounded panel and a 1 dp border.
// It is the background of cards, rows and the chat panel.
func outlinedSurface(gtx layout.Context, fill, border color.NRGBA, radius unit.Dp) layout.Dimensions {
	size := gtx.Constraints.Min
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radius))
	paint.FillShape(gtx.Ops, fill, shape.Op(gtx.Ops))
	stroke := clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(borderWidth))}
	paint.FillShape(gtx.Ops, border, stroke.Op())
	return layout.Dimensions{Size: size}
}

// floatingSurface is outlinedSurface raised above the page with a soft shadow: menus, toasts.
func floatingSurface(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Min
	offset := gtx.Dp(spacingTiny)
	shadow := clip.UniformRRect(image.Rect(0, offset, size.X, size.Y+offset), gtx.Dp(radiusControl+spacingTiny))
	paint.FillShape(gtx.Ops, colorShadow, shadow.Op(gtx.Ops))
	return outlinedSurface(gtx, colorElevatedSurface, colorBorderStrong, radiusFloating)
}
