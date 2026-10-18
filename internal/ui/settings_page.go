package ui

import (
	"log/slog"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/capture"
)

// appVersion is shown at the bottom of Settings; keep it in step with the exe's version
// resource (see the go-winres command in CLAUDE.md).
const appVersion = "0.1.0"

// SettingsPage shows where chat comes from, when WorldKit runs and where its files go.
type SettingsPage struct {
	list             widget.List
	openLogs         widget.Clickable
	openScreenshots  widget.Clickable
	dataDirForm      *dataDirForm
	startupSwitch    toggleSwitch
	startupErrorText string
}

// NewSettingsPage creates the page; call SyncStartupSwitch once services are available.
// invalidate redraws the window when the folder picker closes.
func NewSettingsPage(invalidate func()) *SettingsPage {
	return &SettingsPage{
		list:        widget.List{List: layout.List{Axis: layout.Vertical}},
		dataDirForm: newDataDirForm(invalidate, buttonSecondary),
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
	p.startupSwitch.value.Value = isEnabled
}

// Layout handles input, then draws the page.
func (p *SettingsPage) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	p.handleInput(gtx, services)
	sections := []layout.Widget{
		func(gtx layout.Context) layout.Dimensions {
			return pageHeader(gtx, theme, "Settings", "Where chat comes from, when WorldKit runs, and where files go.", nil)
		},
		func(gtx layout.Context) layout.Dimensions { return p.layoutCaptureSection(gtx, theme, services) },
	}
	if services.Startup != nil {
		sections = append(sections, func(gtx layout.Context) layout.Dimensions { return p.layoutStartupSection(gtx, theme) })
	}
	sections = append(sections,
		func(gtx layout.Context) layout.Dimensions { return p.layoutFilesSection(gtx, theme, services) },
		styledLabel(theme, textSizeCaption, font.Normal, colorTextTertiary,
			"WorldKit "+appVersion+" · Reads FiveM's chat store passively; never touches the game.").Layout,
	)
	inset := layout.Inset{Left: spacingPage, Right: spacingPage, Top: spacingSmall}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return styledList(theme, &p.list).Layout(gtx, len(sections), func(gtx layout.Context, index int) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(settingsMaxWidth))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.Inset{Bottom: spacingXLarge}.Layout(gtx, sections[index])
		})
	})
}

func (p *SettingsPage) handleInput(gtx layout.Context, services Services) {
	if p.openLogs.Clicked(gtx) {
		openFolder(services.Capture.Status().ArchiveDir)
	}
	if p.openScreenshots.Clicked(gtx) {
		openFolder(services.Screenshots.Dir())
	}
	if services.Startup == nil || !p.startupSwitch.Update(gtx) {
		return
	}
	p.startupErrorText = ""
	isEnabled := p.startupSwitch.value.Value
	if err := services.Startup.SetEnabled(isEnabled); err != nil {
		slog.Error("change start with Windows", "error", err)
		p.startupSwitch.value.Value = !isEnabled
		p.startupErrorText = "Couldn't change Windows startup apps."
	}
}

func (p *SettingsPage) layoutCaptureSection(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	status := services.Capture.Status()
	title, _, dotColor := captureCardTexts(status)
	return settingsCard{
		title: "Chat capture",
		trailing: func(gtx layout.Context) layout.Dimensions {
			return statusPill{label: title, color: dotColor}.Layout(gtx, theme)
		},
		rows: []layout.Widget{func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(styledLabel(theme, textSizeSmall, font.Medium, colorTextPrimary, "FiveM data folder").Layout),
				layout.Rigid(layout.Spacer{Height: spacingMedium - spacingTiny/2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.dataDirForm.Layout(gtx, theme, services) }),
				layout.Rigid(layout.Spacer{Height: spacingMedium - spacingTiny/2}.Layout),
				layout.Rigid(caption(theme, dataDirHint(status)).Layout),
			)
		}},
	}.Layout(gtx, theme)
}

func dataDirHint(status capture.Status) string {
	if status.State == capture.StateNeedsDataDir {
		return captureStatusText(status)
	}
	return "Found automatically. Change it only if you moved FiveM or use a -cl2 client."
}

func (p *SettingsPage) layoutStartupSection(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return settingsCard{
		title: "Startup",
		rows: []layout.Widget{func(gtx layout.Context) layout.Dimensions {
			return settingRow(gtx, theme, "Start with Windows",
				"Chat is only saved while WorldKit runs. It starts in the taskbar, out of your way.",
				p.startupErrorText, p.startupSwitch.Layout)
		}},
	}.Layout(gtx, theme)
}

func (p *SettingsPage) layoutFilesSection(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	folderButton := func(click *widget.Clickable) layout.Widget {
		return func(gtx layout.Context) layout.Dimensions {
			return secondaryButton(click, "Open folder").withHeight(controlHeightCompact).Layout(gtx, theme)
		}
	}
	return settingsCard{
		title: "Files",
		rows: []layout.Widget{
			func(gtx layout.Context) layout.Dimensions {
				return pathRow(gtx, theme, "Chat logs", services.Capture.Status().ArchiveDir, folderButton(&p.openLogs))
			},
			func(gtx layout.Context) layout.Dimensions {
				return pathRow(gtx, theme, "Screenshots", services.Screenshots.Dir(), folderButton(&p.openScreenshots))
			},
		},
	}.Layout(gtx, theme)
}

// settingsCard is a titled card whose rows are separated by dividers.
type settingsCard struct {
	title    string
	trailing layout.Widget // optional, right of the title
	rows     []layout.Widget
}

func (c settingsCard) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return outlinedSurface(gtx, colorSurface, colorDivider, radiusCard)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			children := []layout.FlexChild{layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.layoutTitle(gtx, theme) })}
			for _, row := range c.rows {
				children = append(children,
					layout.Rigid(horizontalDivider),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						return layout.Inset{Top: spacingLarge + spacingTiny/2, Bottom: spacingLarge + spacingTiny/2, Left: spacingXLarge - spacingTiny, Right: spacingXLarge - spacingTiny}.Layout(gtx, row)
					}),
				)
			}
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
		}),
	)
}

func (c settingsCard) layoutTitle(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	inset := layout.Inset{Top: spacingLarge + spacingTiny/2, Bottom: spacingLarge + spacingTiny/2, Left: spacingXLarge - spacingTiny, Right: spacingXLarge - spacingTiny}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			layout.Flexed(1, sectionTitle(theme, c.title).Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if c.trailing == nil {
					return layout.Dimensions{}
				}
				return c.trailing(gtx)
			}),
		)
	})
}

// settingRow is a setting's name and description on the left, its control on the right.
func settingRow(gtx layout.Context, theme *material.Theme, name, description, errorText string, control layout.Widget) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(styledLabel(theme, textSizeBody, font.Medium, colorTextPrimary, name).Layout),
				layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
				layout.Rigid(caption(theme, description).Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, errorText) }),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(control),
	)
}

// pathRow names a folder, shows its path in mono, and offers a button for it.
func pathRow(gtx layout.Context, theme *material.Theme, name, path string, button layout.Widget) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(styledLabel(theme, textSizeBody, font.Medium, colorTextPrimary, name).Layout),
				layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := monoText(theme, textSizeCaption, path)
					label.Color = colorTextSecondary
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(button),
	)
}
