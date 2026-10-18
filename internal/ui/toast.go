package ui

import (
	"image"
	"image/color"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// toast is a short confirmation that floats over a corner for a few seconds: "Copied 4
// lines", "Screenshot saved · View". It needs no dismissing.
type toast struct {
	message     string
	icon        strokeIcon
	iconColor   color.NRGBA
	actionLabel string // optional link, e.g. "View"
	action      widget.Clickable
	hideAt      time.Time
}

// Show replaces whatever toast is up with a confirmation.
func (t *toast) Show(now time.Time, message, actionLabel string) {
	t.show(now, message, actionLabel, iconCheck, colorLive)
}

// ShowError replaces whatever toast is up with a problem.
func (t *toast) ShowError(now time.Time, message string) {
	t.show(now, message, "", iconClose, colorDanger)
}

func (t *toast) show(now time.Time, message, actionLabel string, icon strokeIcon, iconColor color.NRGBA) {
	*t = toast{message: message, actionLabel: actionLabel, icon: icon, iconColor: iconColor, hideAt: now.Add(toastDuration)}
}

// ActionClicked reports whether the toast's link was clicked, hiding the toast if so.
func (t *toast) ActionClicked(gtx layout.Context) bool {
	if t.actionLabel == "" || !t.action.Clicked(gtx) {
		return false
	}
	t.hideAt = time.Time{}
	return true
}

// Layout draws the toast while it is up, and asks for one more frame when it should hide.
func (t *toast) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if !gtx.Now.Before(t.hideAt) {
		return layout.Dimensions{}
	}
	gtx.Execute(op.InvalidateCmd{At: t.hideAt})
	gtx.Constraints.Min = image.Point{}
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(floatingSurface),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			inset := layout.Inset{Top: spacingMedium - spacingTiny/2, Bottom: spacingMedium - spacingTiny/2, Left: spacingMedium + spacingTiny/2, Right: spacingMedium + spacingTiny/2}
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
				return row.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return t.icon.Layout(gtx, iconSizeButton, t.iconColor) }),
					layout.Rigid(layout.Spacer{Width: spacingMedium - spacingTiny/2}.Layout),
					layout.Rigid(styledLabel(theme, textSizeSmall, font.Normal, colorTextPrimary, t.message).Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						if t.actionLabel == "" {
							return layout.Dimensions{}
						}
						return layout.Inset{Left: spacingMedium - spacingTiny/2}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
							return linkButton{click: &t.action, label: t.actionLabel}.Layout(gtx, theme)
						})
					}),
				)
			})
		}),
	)
}
