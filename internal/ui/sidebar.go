package ui

import (
	"image"
	"image/color"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/text"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"
)

// NavItem is one destination in the sidebar.
type NavItem struct {
	Label string
	Icon  *widget.Icon
	click widget.Clickable
}

// NewNavItem creates a sidebar destination from Material icon data.
func NewNavItem(label string, iconData []byte) *NavItem {
	return &NavItem{Label: label, Icon: mustIcon(iconData)}
}

// Sidebar is the collapsible navigation bar on the left edge of the window.
type Sidebar struct {
	Items         []*NavItem
	SelectedIndex int

	toggleButton widget.Clickable
	toggleIcon   *widget.Icon
	expansion    expandAnimation
}

// NewSidebar creates an expanded sidebar with the first item selected.
func NewSidebar(items ...*NavItem) *Sidebar {
	return &Sidebar{
		Items:      items,
		toggleIcon: mustIcon(icons.NavigationMenu),
		expansion:  newExpandAnimation(sidebarToggleDuration, true),
	}
}

// SelectedItem returns the destination the user is currently on.
func (s *Sidebar) SelectedItem() *NavItem {
	return s.Items[s.SelectedIndex]
}

// Layout handles clicks and draws the sidebar at its current animated width.
func (s *Sidebar) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	s.handleClicks(gtx)

	expandedness := s.expansion.Progress(gtx)
	width := lerp(gtx.Dp(sidebarCollapsedWidth), gtx.Dp(sidebarExpandedWidth), expandedness)
	size := image.Pt(width, gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(size)

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colorSurface)
	drawRightBorder(gtx, size)

	return layout.UniformInset(sidebarPadding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return s.layoutContent(gtx, theme, expandedness)
	})
}

func (s *Sidebar) handleClicks(gtx layout.Context) {
	if s.toggleButton.Clicked(gtx) {
		s.expansion.Toggle(gtx.Now)
	}
	for index, item := range s.Items {
		if item.click.Clicked(gtx) {
			s.SelectedIndex = index
		}
	}
}

func (s *Sidebar) layoutContent(gtx layout.Context, theme *material.Theme, labelOpacity float32) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			toggle := navRow{click: &s.toggleButton, icon: s.toggleIcon}
			return toggle.Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
		layout.Rigid(horizontalDivider),
	}
	for index, item := range s.Items {
		row := navRow{
			click:        &item.click,
			icon:         item.Icon,
			label:        item.Label,
			labelOpacity: labelOpacity,
			isActive:     index == s.SelectedIndex,
		}
		children = append(children,
			layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.Layout(gtx, theme) }),
		)
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

// navRow draws a clickable icon + label row used by both the toggle button and nav items.
type navRow struct {
	click        *widget.Clickable
	icon         *widget.Icon
	label        string
	labelOpacity float32
	isActive     bool
}

func (r navRow) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return r.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(navItemHeight))
		gtx.Constraints = layout.Exact(size)
		defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)

		r.drawBackground(gtx, size)
		return layout.Inset{Left: navItemPaddingX, Right: navItemPaddingX}.Layout(gtx,
			func(gtx layout.Context) layout.Dimensions { return r.layoutIconAndLabel(gtx, theme) })
	})
}

func (r navRow) drawBackground(gtx layout.Context, size image.Point) {
	if r.isActive || r.click.Hovered() {
		rounded := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(cornerRadius))
		paint.FillShape(gtx.Ops, colorElevatedSurface, rounded.Op(gtx.Ops))
	}
	if r.isActive {
		indicator := image.Rect(0, size.Y/4, gtx.Dp(activeIndicatorWidth), size.Y*3/4)
		paint.FillShape(gtx.Ops, colorPrimary, clip.UniformRRect(indicator, gtx.Dp(activeIndicatorWidth)/2).Op(gtx.Ops))
	}
}

// layoutIconAndLabel keeps the icon at a fixed left position and vertically centers the row.
func (r navRow) layoutIconAndLabel(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// layout.W keeps Min.Y; without clearing it the label stretches to the
		// full row height and draws its text at the top.
		gtx.Constraints.Min.Y = 0
		return r.layoutRowContent(gtx, theme)
	})
}

func (r navRow) layoutRowContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(iconSize)
			gtx.Constraints.Max.X = gtx.Dp(iconSize)
			return r.icon.Layout(gtx, r.iconColor())
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if r.label == "" || r.labelOpacity == 0 {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return r.layoutLabel(gtx, theme)
			})
		}),
	)
}

// layoutLabel always measures the label at full sidebar width so it never re-wraps
// mid-animation; the sidebar clip hides whatever does not fit.
func (r navRow) layoutLabel(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = 0
	gtx.Constraints.Max.X = gtx.Dp(sidebarExpandedWidth)
	label := material.Label(theme, textSizeNavLabel, r.label)
	label.Font.Weight = font.Medium
	label.MaxLines = 1
	label.Alignment = text.Start
	label.Color = withOpacity(r.labelColor(), r.labelOpacity)
	return label.Layout(gtx)
}

func (r navRow) iconColor() color.NRGBA {
	switch {
	case r.isActive:
		return colorPrimary
	case r.click.Hovered():
		return colorTextPrimary
	default:
		return colorTextSecondary
	}
}

func (r navRow) labelColor() color.NRGBA {
	if r.isActive || r.click.Hovered() {
		return colorTextPrimary
	}
	return colorTextSecondary
}

func horizontalDivider(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(borderWidth))
	paint.FillShape(gtx.Ops, colorBorder, clip.Rect{Max: size}.Op())
	return layout.Dimensions{Size: size}
}

func drawRightBorder(gtx layout.Context, size image.Point) {
	border := image.Rect(size.X-gtx.Dp(borderWidth), 0, size.X, size.Y)
	paint.FillShape(gtx.Ops, colorBorder, clip.Rect(border).Op())
}

func lerp(from, to int, progress float32) int {
	return from + int(float32(to-from)*progress)
}

// mustIcon panics on invalid icon data, which can only happen from a programming error.
func mustIcon(data []byte) *widget.Icon {
	icon, err := widget.NewIcon(data)
	if err != nil {
		panic(err)
	}
	return icon
}
