package ui

import (
	"log/slog"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// SettingsPage shows chat capture status and the preferences that control it.
type SettingsPage struct {
	list             layout.List
	openLogs         widget.Clickable
	dataDirForm      *dataDirForm
	startupSwitch    widget.Bool
	startupErrorText string
}

// NewSettingsPage creates the page; call SyncStartupSwitch once services are available.
func NewSettingsPage() *SettingsPage {
	return &SettingsPage{
		list:        layout.List{Axis: layout.Vertical},
		dataDirForm: newDataDirForm(),
	}
}

// SyncStartupSwitch shows whether WorldKit currently starts with Windows.
func (p *SettingsPage) SyncStartupSwitch(startup StartupEntry) {
	if startup == nil {
		return
	}
	isEnabled, err := startup.IsEnabled()
	if err != nil {
		slog.Warn("read start with Windows", "error", err)
	}
	p.startupSwitch.Value = isEnabled
}

// Layout handles input, then draws the page.
func (p *SettingsPage) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	p.handleInput(gtx, services)
	sections := []layout.Widget{
		pageTitle(theme, "Settings").Layout,
		func(gtx layout.Context) layout.Dimensions { return p.layoutCaptureSection(gtx, theme, services) },
		func(gtx layout.Context) layout.Dimensions { return p.layoutDataDirSection(gtx, theme, services) },
		func(gtx layout.Context) layout.Dimensions { return p.layoutStartupSection(gtx, theme, services) },
	}
	return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(contentMaxWidth))
		return p.list.Layout(gtx, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			if index == 0 {
				return sections[index](gtx)
			}
			return layout.Inset{Top: spacingMedium}.Layout(gtx, sections[index])
		})
	})
}

func (p *SettingsPage) handleInput(gtx layout.Context, services Services) {
	if p.openLogs.Clicked(gtx) {
		openFolder(services.Capture.Status().ArchiveDir)
	}
	if services.Startup == nil || !p.startupSwitch.Update(gtx) {
		return
	}
	p.startupErrorText = ""
	if err := services.Startup.SetEnabled(p.startupSwitch.Value); err != nil {
		slog.Error("change start with Windows", "error", err)
		p.startupSwitch.Value = !p.startupSwitch.Value
		p.startupErrorText = "Couldn't change Windows startup apps."
	}
}

func (p *SettingsPage) layoutCaptureSection(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	status := services.Capture.Status()
	return card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(sectionTitle(theme, "Chat capture").Layout),
			layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
			layout.Rigid(bodyText(theme, captureStatusText(status)).Layout),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(caption(theme, "Sessions are saved as text files in "+status.ArchiveDir).Layout),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return secondaryButton(&p.openLogs, "Open logs folder").Layout(gtx, theme)
			}),
		)
	})
}

func (p *SettingsPage) layoutDataDirSection(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	return card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(sectionTitle(theme, "FiveM data folder").Layout),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(caption(theme, "Where FiveM keeps its data (CitizenFX). Found automatically when it's in the usual place.").Layout),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return p.dataDirForm.Layout(gtx, theme, services)
			}),
		)
	})
}

func (p *SettingsPage) layoutStartupSection(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	if services.Startup == nil {
		return layout.Dimensions{}
	}
	return card(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
				return row.Layout(gtx,
					layout.Flexed(1, sectionTitle(theme, "Start with Windows").Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						toggle := material.Switch(theme, &p.startupSwitch, "Start with Windows")
						toggle.Color.Enabled = colorPrimary
						toggle.Color.Disabled = colorTextSecondary
						toggle.Color.Track = colorBorder
						return toggle.Layout(gtx)
					}),
				)
			}),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(caption(theme, "Chat is only saved while WorldKit is running. Starts minimized.").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return errorCaption(gtx, theme, p.startupErrorText)
			}),
		)
	})
}

func sectionTitle(theme *material.Theme, text string) material.LabelStyle {
	title := material.Label(theme, textSizeBody, text)
	title.Font.Weight = font.SemiBold
	return title
}

func bodyText(theme *material.Theme, text string) material.LabelStyle {
	return material.Label(theme, textSizeBody, text)
}
