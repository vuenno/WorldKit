package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// button draws a text button in one of the two styles from DESIGN.md.
type button struct {
	click     *widget.Clickable
	label     string
	isPrimary bool
}

func primaryButton(click *widget.Clickable, label string) button {
	return button{click: click, label: label, isPrimary: true}
}

func secondaryButton(click *widget.Clickable, label string) button {
	return button{click: click, label: label}
}

func (b button) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return b.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(b.drawBackground),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.Y = gtx.Dp(buttonHeight)
				return layout.Inset{Left: buttonPaddingX, Right: buttonPaddingX}.Layout(gtx,
					func(gtx layout.Context) layout.Dimensions { return b.layoutLabel(gtx, theme) })
			}),
		)
	})
}

func (b button) drawBackground(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Min
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(cornerRadius))
	defer shape.Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	paint.Fill(gtx.Ops, b.fillColor())
	if !b.isPrimary {
		border := clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(borderWidth))}
		paint.FillShape(gtx.Ops, colorBorder, border.Op())
	}
	return layout.Dimensions{Size: size}
}

func (b button) layoutLabel(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(theme, textSizeBody, b.label)
		label.Font.Weight = font.Medium
		label.MaxLines = 1
		label.Color = colorTextPrimary
		return label.Layout(gtx)
	})
}

func (b button) fillColor() color.NRGBA {
	switch {
	case b.isPrimary && b.click.Hovered():
		return colorPrimaryHover
	case b.isPrimary:
		return colorPrimary
	default:
		return colorElevatedSurface
	}
}
