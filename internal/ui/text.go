package ui

import (
	"image/color"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Text styles from DESIGN.md's type scale. Each returns a label ready to lay out.

func pageTitle(theme *material.Theme, text string) material.LabelStyle {
	return styledLabel(theme, textSizePageTitle, font.SemiBold, colorTextPrimary, text)
}

func screenTitle(theme *material.Theme, text string) material.LabelStyle {
	return styledLabel(theme, textSizeScreenTitle, font.SemiBold, colorTextPrimary, text)
}

func sectionTitle(theme *material.Theme, text string) material.LabelStyle {
	return styledLabel(theme, textSizeSection, font.SemiBold, colorTextPrimary, text)
}

func bodyText(theme *material.Theme, text string) material.LabelStyle {
	return styledLabel(theme, textSizeBody, font.Normal, colorTextPrimary, text)
}

// secondaryText is supporting text under a title: subtitles, metadata.
func secondaryText(theme *material.Theme, text string) material.LabelStyle {
	return styledLabel(theme, textSizeSmall, font.Normal, colorTextSecondary, text)
}

func caption(theme *material.Theme, text string) material.LabelStyle {
	return styledLabel(theme, textSizeCaption, font.Normal, colorTextSecondary, text)
}

// groupLabel heads a group of rows ("TODAY"): small, uppercase, spaced out.
func groupLabel(theme *material.Theme, text string) material.LabelStyle {
	label := styledLabel(theme, textSizeCaption, font.SemiBold, colorTextTertiary, strings.ToUpper(text))
	return label
}

// monoText sets numbers, times and paths in Geist Mono.
func monoText(theme *material.Theme, size unit.Sp, text string) material.LabelStyle {
	label := styledLabel(theme, size, font.Normal, colorTextPrimary, text)
	label.Font.Typeface = fontMono
	return label
}

func styledLabel(theme *material.Theme, size unit.Sp, weight font.Weight, c color.NRGBA, text string) material.LabelStyle {
	label := material.Label(theme, size, text)
	label.Font.Weight = weight
	label.Color = c
	return label
}

// errorCaption shows a problem under a control, or nothing when text is empty.
func errorCaption(gtx layout.Context, theme *material.Theme, text string) layout.Dimensions {
	if text == "" {
		return layout.Dimensions{}
	}
	return layout.Inset{Top: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := caption(theme, text)
		label.Color = colorDanger
		return label.Layout(gtx)
	})
}

// pageHeader is a top-level page's title and subtitle, with an optional control (search,
// a button) at the right, bottom-aligned with the subtitle.
func pageHeader(gtx layout.Context, theme *material.Theme, title, subtitle string, trailing layout.Widget) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.End}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pageTitle(theme, title).Layout),
				layout.Rigid(layout.Spacer{Height: spacingTiny + spacingTiny/2}.Layout),
				layout.Rigid(secondaryText(theme, subtitle).Layout),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if trailing == nil {
				return layout.Dimensions{}
			}
			return layout.Inset{Left: spacingLarge}.Layout(gtx, trailing)
		}),
	)
}
