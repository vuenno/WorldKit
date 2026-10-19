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

	"world-kit/internal/capture"
)

// NavItem is one destination in the sidebar.
type NavItem struct {
	Label string
	Icon  strokeIcon
	// StartsGroup draws a divider above the item, separating it from the ones before.
	StartsGroup bool
	// HasBadge shows a small orange dot on the icon: work in progress on that page.
	HasBadge bool
	click    widget.Clickable
}

// NewNavItem creates a sidebar destination.
func NewNavItem(label string, icon strokeIcon) *NavItem {
	return &NavItem{Label: label, Icon: icon}
}

// Sidebar is the collapsible navigation bar on the left edge of the window: the brand,
// the main destinations, the capture status and Settings at the bottom.
type Sidebar struct {
	items        []*NavItem // main destinations, then the bottom one
	bottomItem   *NavItem
	selected     *NavItem
	logo         *Logo
	toggleButton widget.Clickable
	expansion    expandAnimation
}

// NewSidebar creates an expanded sidebar with the first main item selected.
func NewSidebar(logo *Logo, mainItems []*NavItem, bottomItem *NavItem) *Sidebar {
	return &Sidebar{
		items:      append(append([]*NavItem{}, mainItems...), bottomItem),
		bottomItem: bottomItem,
		selected:   mainItems[0],
		logo:       logo,
		expansion:  newExpandAnimation(sidebarToggleDuration, true),
	}
}

// SelectedItem returns the destination the user is currently on.
func (s *Sidebar) SelectedItem() *NavItem {
	return s.selected
}

// Select moves to item, e.g. when another page links to it.
func (s *Sidebar) Select(item *NavItem) {
	s.selected = item
}

// Layout handles clicks and draws the sidebar at its current animated width.
func (s *Sidebar) Layout(gtx layout.Context, theme *material.Theme, status capture.Status) layout.Dimensions {
	s.handleClicks(gtx)

	expandedness := s.expansion.Progress(gtx)
	width := lerp(gtx.Dp(sidebarCollapsedWidth), gtx.Dp(sidebarExpandedWidth), expandedness)
	size := image.Pt(width, gtx.Constraints.Max.Y)
	gtx.Constraints = layout.Exact(size)

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colorSidebar)
	drawRightBorder(gtx, size)

	return layout.UniformInset(sidebarPadding).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return s.layoutContent(gtx, theme, status, expandedness)
	})
}

func (s *Sidebar) handleClicks(gtx layout.Context) {
	if s.toggleButton.Clicked(gtx) {
		s.expansion.Toggle(gtx.Now)
	}
	for _, item := range s.items {
		if item.click.Clicked(gtx) {
			s.selected = item
		}
	}
}

func (s *Sidebar) layoutContent(gtx layout.Context, theme *material.Theme, status capture.Status, labelOpacity float32) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.layoutHeader(gtx, theme, labelOpacity)
		}),
		layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
	}
	for _, item := range s.items {
		if item == s.bottomItem {
			continue
		}
		if item.StartsGroup {
			children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Top: spacingSmall - spacingTiny, Bottom: spacingSmall, Left: navItemPaddingX, Right: navItemPaddingX}.Layout(gtx, horizontalDivider)
			}))
		}
		children = append(children, s.navRowChild(theme, item, labelOpacity), layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout))
	}
	children = append(children,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Dimensions{Size: gtx.Constraints.Min}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return captureCard{status: status, labelOpacity: labelOpacity}.Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
		s.navRowChild(theme, s.bottomItem, labelOpacity),
	)
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (s *Sidebar) navRowChild(theme *material.Theme, item *NavItem, labelOpacity float32) layout.FlexChild {
	row := navRow{click: &item.click, icon: item.Icon, label: item.Label, labelOpacity: labelOpacity, isActive: item == s.selected, hasBadge: item.HasBadge}
	return layout.Rigid(func(gtx layout.Context) layout.Dimensions { return row.Layout(gtx, theme) })
}

// layoutHeader draws the brand (logo, name, subtitle), which fades with the labels and is
// clipped away when collapsed, and the collapse button at the right. The brand doubles as a
// handle to drag the window.
func (s *Sidebar) layoutHeader(gtx layout.Context, theme *material.Theme, brandOpacity float32) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.Y = gtx.Dp(navItemHeight)
			return layout.Stack{}.Layout(gtx,
				layout.Expanded(moveArea),
				layout.Stacked(func(gtx layout.Context) layout.Dimensions {
					if brandOpacity == 0 {
						return layout.Dimensions{Size: gtx.Constraints.Min}
					}
					defer clip.Rect{Max: gtx.Constraints.Max}.Push(gtx.Ops).Pop()
					defer paint.PushOpacity(gtx.Ops, brandOpacity).Pop()
					return s.layoutBrand(gtx, theme)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return iconButton(&s.toggleButton, iconPanelLeft).withKind(buttonGhost).withHeight(navItemHeight).Layout(gtx, theme)
		}),
	)
}

func (s *Sidebar) layoutBrand(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = gtx.Dp(sidebarExpandedWidth)
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.logo.Layout(gtx, logoSizeSidebar)
		}),
		layout.Rigid(layout.Spacer{Width: spacingSmall + spacingTiny/2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					name := styledLabel(theme, textSizeSection, font.Bold, colorTextPrimary, appName)
					name.MaxLines = 1
					return name.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					subtitle := styledLabel(theme, textSizeTiny, font.Normal, colorTextTertiary, "for GTA World")
					subtitle.MaxLines = 1
					return subtitle.Layout(gtx)
				}),
			)
		}),
	)
}

// captureCard tells at a glance whether chat is being saved: a colored dot and a short
// status. Collapsed, only the dot remains.
type captureCard struct {
	status       capture.Status
	labelOpacity float32
}

func (c captureCard) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	title, detail, dotColor := captureCardTexts(c.status)
	if c.labelOpacity < 1 {
		gtx.Constraints.Min.Y = gtx.Dp(navItemHeight)
		gtx.Constraints.Min.X = gtx.Dp(sidebarCollapsedWidth - 2*sidebarPadding)
		return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return dot(gtx, statusDotSize, dotColor)
		})
	}
	box := surfaceBox{fill: colorSurface, border: colorDivider, radius: radiusControl + spacingTiny/2, padding: spacingMedium}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X // the box's Stack clears Min for its content
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
				return row.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return dot(gtx, statusDotSize, dotColor) }),
					layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
					layout.Rigid(styledLabel(theme, textSizeSmall, font.SemiBold, colorTextPrimary, title).Layout),
				)
			}),
			layout.Rigid(layout.Spacer{Height: spacingTiny + spacingTiny/2}.Layout),
			layout.Rigid(caption(theme, detail).Layout),
		)
	})
}

// captureCardTexts sums up the capture status for the sidebar card.
func captureCardTexts(status capture.Status) (title, detail string, dotColor color.NRGBA) {
	switch status.State {
	case capture.StateStarting:
		return "Starting…", "Looking for FiveM's chat", colorTextTertiary
	case capture.StateNeedsDataDir:
		return "Not capturing", "FiveM data folder not found", colorDanger
	case capture.StateArchiveInUse:
		return "Not capturing", "Another WorldKit is running", colorDanger
	case capture.StateFailed:
		return "Capture stopped", "See Settings for details", colorDanger
	}
	switch {
	case status.Err != nil:
		return "Retrying…", "Couldn't read FiveM just now", colorWarning
	case status.LastSaved.IsZero():
		return "Watching", "Waiting for GTA World chat", colorLive
	default:
		return "Capturing", "Last line saved at " + status.LastSaved.Format("15:04:05"), colorLive
	}
}

// navRow draws one clickable sidebar destination: icon, then label.
type navRow struct {
	click        *widget.Clickable
	icon         strokeIcon
	label        string
	labelOpacity float32
	isActive     bool
	hasBadge     bool
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
		rounded := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusControl))
		paint.FillShape(gtx.Ops, colorElevatedSurface, rounded.Op(gtx.Ops))
	}
	if r.isActive {
		barHeight := gtx.Dp(activeIndicatorHeight)
		top := (size.Y - barHeight) / 2
		indicator := image.Rect(0, top, gtx.Dp(activeIndicatorWidth), top+barHeight)
		paint.FillShape(gtx.Ops, colorPrimary, clip.UniformRRect(indicator, gtx.Dp(activeIndicatorWidth)/2).Op(gtx.Ops))
	}
}

// layoutIconAndLabel keeps the icon at a fixed left position and vertically centers the row.
func (r navRow) layoutIconAndLabel(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.W.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		// layout.W keeps Min.Y; without clearing it the label stretches to the
		// full row height and draws its text at the top.
		gtx.Constraints.Min.Y = 0
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				dims := r.icon.Layout(gtx, iconSize, r.iconColor())
				if r.hasBadge {
					side := gtx.Dp(navBadgeSize)
					badge := image.Rect(dims.Size.X-side/2, -side/2, dims.Size.X+side/2, side/2)
					paint.FillShape(gtx.Ops, colorPrimary, clip.Ellipse(badge).Op(gtx.Ops))
				}
				return dims
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if r.labelOpacity == 0 {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return r.layoutLabel(gtx, theme)
				})
			}),
		)
	})
}

// layoutLabel always measures the label at full sidebar width so it never re-wraps
// mid-animation; the sidebar clip hides whatever does not fit.
func (r navRow) layoutLabel(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = 0
	gtx.Constraints.Max.X = gtx.Dp(sidebarExpandedWidth)
	label := styledLabel(theme, textSizeBody, font.Medium, withOpacity(r.labelColor(), r.labelOpacity), r.label)
	label.MaxLines = 1
	label.Alignment = text.Start
	return label.Layout(gtx)
}

func (r navRow) iconColor() color.NRGBA {
	switch {
	case r.isActive:
		return colorPrimary
	case r.click.Hovered():
		return colorTextPrimary
	default:
		return colorTextTertiary
	}
}

func (r navRow) labelColor() color.NRGBA {
	if r.isActive || r.click.Hovered() {
		return colorTextPrimary
	}
	return colorTextSecondary
}

func drawRightBorder(gtx layout.Context, size image.Point) {
	border := image.Rect(size.X-gtx.Dp(borderWidth), 0, size.X, size.Y)
	paint.FillShape(gtx.Ops, colorDivider, clip.Rect(border).Op())
}

func lerp(from, to int, progress float32) int {
	return from + int(float32(to-from)*progress)
}
