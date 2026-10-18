package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// buttonKind is one of the button styles in DESIGN.md.
type buttonKind int

const (
	buttonPrimary   buttonKind = iota // the one main action on a screen: orange
	buttonSecondary                   // everything else: surface with an outline
	buttonGhost                       // actions inside a floating menu: no fill until hovered
	buttonDanger                      // confirming a delete: filled danger color
)

// button draws a text and/or icon button. Build one with primaryButton, secondaryButton,
// ghostButton, dangerButton or iconButton, then adjust it with the with… methods.
type button struct {
	click    *widget.Clickable
	label    string
	icon     strokeIcon // optional, drawn before the label
	kind     buttonKind
	height   unit.Dp
	isDanger bool // secondary/ghost buttons that delete: label and icon in colorDanger
}

func primaryButton(click *widget.Clickable, label string) button {
	return button{click: click, label: label, kind: buttonPrimary, height: controlHeight}
}

func secondaryButton(click *widget.Clickable, label string) button {
	return button{click: click, label: label, kind: buttonSecondary, height: controlHeight}
}

func ghostButton(click *widget.Clickable, label string) button {
	return button{click: click, label: label, kind: buttonGhost, height: controlHeightSmall}
}

func dangerButton(click *widget.Clickable, label string) button {
	return button{click: click, label: label, kind: buttonDanger, height: controlHeight}
}

// iconButton is a square secondary button showing only icon.
func iconButton(click *widget.Clickable, icon strokeIcon) button {
	return button{click: click, icon: icon, kind: buttonSecondary, height: controlHeight}
}

func (b button) withIcon(icon strokeIcon) button {
	b.icon = icon
	return b
}

func (b button) withHeight(height unit.Dp) button {
	b.height = height
	return b
}

func (b button) withKind(kind buttonKind) button {
	b.kind = kind
	return b
}

// asDanger colors the label and icon for a destructive action.
func (b button) asDanger() button {
	b.isDanger = true
	return b
}

func (b button) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return b.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min = image.Point{}
		height := gtx.Dp(b.height)
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(b.drawBackground),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Pt(height, height)
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return b.layoutContent(gtx, theme)
				})
			}),
		)
	})
}

func (b button) layoutContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	paddingX := buttonPaddingX
	if b.label == "" {
		paddingX = 0
	}
	return layout.Inset{Left: paddingX, Right: paddingX}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if b.icon == "" {
					return layout.Dimensions{}
				}
				return b.icon.Layout(gtx, b.iconSize(), b.contentColor())
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if b.icon == "" || b.label == "" {
					return layout.Dimensions{}
				}
				return layout.Spacer{Width: spacingSmall}.Layout(gtx)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if b.label == "" {
					return layout.Dimensions{}
				}
				return b.layoutLabel(gtx, theme)
			}),
		)
	})
}

func (b button) layoutLabel(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	size := textSizeBody
	if b.height < controlHeight {
		size = textSizeSmall
	}
	label := material.Label(theme, size, b.label)
	label.Font.Weight = font.Medium
	if b.kind == buttonPrimary || b.kind == buttonDanger {
		label.Font.Weight = font.SemiBold
	}
	label.MaxLines = 1
	label.Color = b.contentColor()
	return label.Layout(gtx)
}

func (b button) iconSize() unit.Dp {
	if b.height < controlHeight {
		return iconSizeSmall
	}
	return iconSizeButton
}

func (b button) drawBackground(gtx layout.Context) layout.Dimensions {
	size := gtx.Constraints.Min
	radius := radiusControl
	if b.height <= controlHeightSmall {
		radius = radiusMenu
	}
	shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radius))
	defer shape.Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	if fill, ok := b.fillColor(); ok {
		paint.Fill(gtx.Ops, fill)
	}
	if b.kind == buttonSecondary {
		border := clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(borderWidth))}
		paint.FillShape(gtx.Ops, colorBorder, border.Op())
	}
	return layout.Dimensions{Size: size}
}

// fillColor returns the background, or false for a ghost button at rest.
func (b button) fillColor() (color.NRGBA, bool) {
	isHovered := b.click.Hovered()
	switch b.kind {
	case buttonPrimary:
		if isHovered {
			return colorPrimaryHover, true
		}
		return colorPrimary, true
	case buttonDanger:
		if isHovered {
			return colorDangerHover, true
		}
		return colorDanger, true
	case buttonGhost:
		return colorPressed, isHovered
	default:
		if isHovered {
			return colorElevatedSurface, true
		}
		return colorSurface, true
	}
}

func (b button) contentColor() color.NRGBA {
	switch {
	case b.kind == buttonPrimary || b.kind == buttonDanger:
		return colorOnPrimary
	case b.isDanger:
		return colorDanger
	default:
		return colorTextPrimary
	}
}

// Widget returns the button as a layout.Widget, for flex children.
func (b button) Widget(theme *material.Theme) layout.Widget {
	return func(gtx layout.Context) layout.Dimensions { return b.Layout(gtx, theme) }
}
