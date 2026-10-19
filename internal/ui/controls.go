package ui

import (
	"image"
	"image/color"
	"strconv"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/unit"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// segmentedControl picks exactly one of a few options, shown side by side.
type segmentedControl struct {
	labels   []string
	clicks   []widget.Clickable
	selected int
}

func newSegmentedControl(labels ...string) *segmentedControl {
	return &segmentedControl{labels: labels, clicks: make([]widget.Clickable, len(labels))}
}

// Update handles clicks and reports whether the selection changed.
func (s *segmentedControl) Update(gtx layout.Context) bool {
	isChanged := false
	for i := range s.clicks {
		if s.clicks[i].Clicked(gtx) && s.selected != i {
			s.selected = i
			isChanged = true
		}
	}
	return isChanged
}

func (s *segmentedControl) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	children := make([]layout.FlexChild, len(s.labels))
	for i := range s.labels {
		children[i] = layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.layoutSegment(gtx, theme, i)
		})
	}
	return surfaceBox{fill: colorSurface, border: colorDivider, radius: radiusControl, padding: segmentedPadding}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
		})
}

func (s *segmentedControl) layoutSegment(gtx layout.Context, theme *material.Theme, index int) layout.Dimensions {
	click := &s.clicks[index]
	isSelected := index == s.selected
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		height := gtx.Dp(segmentHeight)
		return layout.Stack{Alignment: layout.Center}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				size := gtx.Constraints.Min
				shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusSmall))
				defer shape.Push(gtx.Ops).Pop()
				pointer.CursorPointer.Add(gtx.Ops)
				if isSelected {
					paint.Fill(gtx.Ops, colorPressed)
				}
				return layout.Dimensions{Size: size}
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min = image.Pt(0, height)
				return layout.Inset{Left: spacingMedium, Right: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						label := material.Label(theme, textSizeSmall, s.labels[index])
						label.Font.Weight = font.Medium
						label.MaxLines = 1
						label.Color = colorTextSecondary
						if isSelected || click.Hovered() {
							label.Color = colorTextPrimary
						}
						return label.Layout(gtx)
					})
				})
			}),
		)
	})
}

// numberStepper picks a whole number in min..max with − and + buttons around its value.
type numberStepper struct {
	value, min, max int
	unit            string // shown after the value, e.g. "px"
	decrease        widget.Clickable
	increase        widget.Clickable
}

func newNumberStepper(value, min, max int, unit string) *numberStepper {
	return &numberStepper{value: value, min: min, max: max, unit: unit}
}

// Update handles clicks and reports whether the value changed.
func (s *numberStepper) Update(gtx layout.Context) bool {
	old := s.value
	if s.decrease.Clicked(gtx) {
		s.step(-1)
	}
	if s.increase.Clicked(gtx) {
		s.step(1)
	}
	return s.value != old
}

// step moves the value by delta, staying within min..max.
func (s *numberStepper) step(delta int) {
	s.value = max(s.min, min(s.value+delta, s.max))
}

func (s *numberStepper) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	stepButton := func(click *widget.Clickable, icon strokeIcon) layout.FlexChild {
		return layout.Rigid(iconButton(click, icon).withKind(buttonGhost).withHeight(segmentHeight).Widget(theme))
	}
	return surfaceBox{fill: colorSurface, border: colorDivider, radius: radiusControl, padding: segmentedPadding}.Layout(gtx,
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}.Layout(gtx,
				stepButton(&s.decrease, iconMinus),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min = image.Pt(gtx.Dp(stepperValueWidth), 0)
					return layout.Center.Layout(gtx, monoText(theme, textSizeSmall, strconv.Itoa(s.value)+" "+s.unit).Layout)
				}),
				stepButton(&s.increase, iconPlus),
			)
		})
}

// toggleSwitch is an on/off switch: an orange track while on.
type toggleSwitch struct {
	value widget.Bool
}

// Update reports whether the user flipped the switch.
func (s *toggleSwitch) Update(gtx layout.Context) bool {
	return s.value.Update(gtx)
}

func (s *toggleSwitch) Layout(gtx layout.Context) layout.Dimensions {
	return s.value.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Dp(switchWidth), gtx.Dp(switchHeight))
		track := clip.UniformRRect(image.Rectangle{Max: size}, size.Y/2)
		defer track.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		trackColor, knobColor := colorPressed, colorTextSecondary
		if s.value.Value {
			trackColor, knobColor = colorPrimary, colorOnPrimary
		}
		paint.Fill(gtx.Ops, trackColor)
		inset := gtx.Dp(segmentedPadding)
		knobSide := size.Y - 2*inset
		knobX := inset
		if s.value.Value {
			knobX = size.X - inset - knobSide
		}
		knob := image.Rect(knobX, inset, knobX+knobSide, inset+knobSide)
		paint.FillShape(gtx.Ops, knobColor, clip.Ellipse(knob).Op(gtx.Ops))
		return layout.Dimensions{Size: size}
	})
}

// removableChip is a selected value with a × that removes it, e.g. an active character filter.
type removableChip struct {
	remove *widget.Clickable
	label  string
}

func (c removableChip) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	box := surfaceBox{fill: colorElevatedSurface, radius: pillHeight / 2, height: pillHeight}
	return box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			layout.Rigid(layout.Spacer{Width: chipPaddingX}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Label(theme, textSizeCaption, c.label)
				label.Color = colorTextPrimary
				label.MaxLines = 1
				return label.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return c.remove.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					side := gtx.Dp(pillHeight - spacingTiny)
					defer clip.Rect{Max: image.Pt(side, side)}.Push(gtx.Ops).Pop()
					pointer.CursorPointer.Add(gtx.Ops)
					iconColor := colorTextSecondary
					if c.remove.Hovered() {
						iconColor = colorTextPrimary
					}
					return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return iconClose.Layout(gtx, chipIconSize, iconColor)
					})
				})
			}),
			layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
		)
	})
}

// statusPill is a small rounded label with a dot, e.g. "Live" or "Capturing".
type statusPill struct {
	label string
	color color.NRGBA
}

func (p statusPill) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	box := surfaceBox{fill: withOpacity(p.color, liveTintOpacity), radius: pillHeight / 2, height: pillHeight, paddingX: chipPaddingX}
	return box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return dot(gtx, pillDotSize, p.color) }),
			layout.Rigid(layout.Spacer{Width: pillDotSize}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				label := material.Label(theme, textSizeCaption, p.label)
				label.Font.Weight = font.SemiBold
				label.Color = p.color
				return label.Layout(gtx)
			}),
		)
	})
}

// countBadge is an orange count next to a control's label, e.g. how many characters are on.
func countBadge(gtx layout.Context, theme *material.Theme, text string) layout.Dimensions {
	box := surfaceBox{fill: colorPrimary, radius: badgeHeight / 2, height: badgeHeight, paddingX: pillDotSize, minWidth: badgeHeight}
	return box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := material.Label(theme, textSizeCaption, text)
		label.Font.Weight = font.SemiBold
		label.Color = colorOnPrimary
		return label.Layout(gtx)
	})
}

// dot draws a filled circle of the given diameter.
func dot(gtx layout.Context, diameter unit.Dp, c color.NRGBA) layout.Dimensions {
	side := gtx.Dp(diameter)
	paint.FillShape(gtx.Ops, c, clip.Ellipse(image.Rect(0, 0, side, side)).Op(gtx.Ops))
	return layout.Dimensions{Size: image.Pt(side, side)}
}

// surfaceBox draws content on a rounded fill with an optional 1 dp border. Content is
// centered vertically when height is set.
type surfaceBox struct {
	fill     color.NRGBA
	border   color.NRGBA // zero: no border
	radius   unit.Dp
	padding  unit.Dp // all sides
	paddingX unit.Dp // added left and right
	height   unit.Dp // exact height; zero sizes to content
	minWidth unit.Dp
}

func (b surfaceBox) Layout(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(b.radius))
			paint.FillShape(gtx.Ops, b.fill, shape.Op(gtx.Ops))
			if b.border != (color.NRGBA{}) {
				border := clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(borderWidth))}
				paint.FillShape(gtx.Ops, b.border, border.Op())
			}
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			inset := layout.Inset{Top: b.padding, Bottom: b.padding, Left: b.padding + b.paddingX, Right: b.padding + b.paddingX}
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if b.height > 0 {
					gtx.Constraints.Min.Y = gtx.Dp(b.height) - 2*gtx.Dp(b.padding)
				}
				gtx.Constraints.Min.X = max(gtx.Constraints.Min.X, gtx.Dp(b.minWidth)-2*gtx.Dp(b.padding+b.paddingX))
				if b.height == 0 {
					return content(gtx)
				}
				return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.Y = 0 // layout.W keeps Min.Y; text would stretch
					return content(gtx)
				})
			})
		}),
	)
}

// textField draws a single-line editor on an outlined input, with an optional leading icon.
type textField struct {
	editor *widget.Editor
	hint   string
	icon   strokeIcon
	height unit.Dp
	isMono bool // paths and other machine text
}

func (f textField) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	height := f.height
	if height == 0 {
		height = controlHeight
	}
	box := surfaceBox{fill: colorAppBackground, border: colorBorder, radius: radiusControl, height: height, paddingX: spacingMedium}
	return box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if f.icon == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Right: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return f.icon.Layout(gtx, iconSizeSmall, colorTextTertiary)
				})
			}),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				field := material.Editor(theme, f.editor, f.hint)
				field.TextSize = textSizeSmall
				field.HintColor = colorTextTertiary
				field.SelectionColor = withOpacity(colorPrimary, selectionOpacity)
				if f.isMono {
					field.Font.Typeface = fontMono
				}
				return field.Layout(gtx)
			}),
		)
	})
}

// hoverArea tracks whether the pointer is over an area without making it clickable, so
// buttons inside it keep their own clicks (e.g. a screenshot card's actions).
type hoverArea struct {
	isHovered bool
}

// Layout lays out content inside the area. The area is registered as content's parent, not
// on top of it: an area drawn over the buttons would take their clicks.
func (h *hoverArea) Layout(gtx layout.Context, content layout.Widget) layout.Dimensions {
	for {
		ev, ok := gtx.Event(pointer.Filter{Target: h, Kinds: pointer.Enter | pointer.Leave | pointer.Cancel})
		if !ok {
			break
		}
		if e, ok := ev.(pointer.Event); ok {
			h.isHovered = e.Kind == pointer.Enter
		}
	}
	recording := op.Record(gtx.Ops)
	dims := content(gtx)
	call := recording.Stop()
	defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
	event.Op(gtx.Ops, h)
	call.Add(gtx.Ops)
	return dims
}

// linkButton is a text-only action in the link color, e.g. "Clear filters" or a toast's "View".
type linkButton struct {
	click *widget.Clickable
	label string
}

func (b linkButton) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return b.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := styledLabel(theme, textSizeCaption, font.SemiBold, colorLink, b.label)
		if b.click.Hovered() {
			label.Color = colorPrimaryHover
		}
		dims := label.Layout(gtx)
		defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return dims
	})
}
