// Package ui contains WorldKit's screens, widgets and theme.
package ui

import (
	"log/slog"

	"gioui.org/app"
	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget/material"
	"golang.org/x/exp/shiny/materialdesign/icons"

	"world-kit/assets"
	"world-kit/internal/capture"
)

// Run drives the window's event loop until the window is closed.
func Run(window *app.Window, services Services) error {
	theme := NewTheme()
	shell := NewShell(services, window.Invalidate)
	var ops op.Ops

	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			slog.Info("window destroyed", "error", event.Err)
			return event.Err
		case *app.ClosingEvent:
			slog.Info("window closing")
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			shell.Layout(gtx, theme)
			event.Frame(gtx.Ops)
		}
	}
}

// Shell is the top-level layout: sidebar on the left, the selected page on the right,
// and any prompt that needs an answer on top.
type Shell struct {
	services        Services
	logsItem        *NavItem
	screenshotsItem *NavItem
	settingsItem    *NavItem
	shownItem       *NavItem
	sidebar         *Sidebar
	logsPage        *LogsPage
	screenshotsPage *ScreenshotsPage
	settingsPage    *SettingsPage
	dataDirPrompt   *dataDirPrompt
	startupPrompt   *startupPrompt
}

// NewShell creates the shell with WorldKit's navigation destinations. invalidate redraws
// the window when background work finishes.
func NewShell(services Services, invalidate func()) *Shell {
	logsItem := NewNavItem("Logs", icons.ActionDescription)
	screenshotsItem := NewNavItem("Log Screenshots", icons.ImagePhotoLibrary)
	settingsItem := NewNavItem("Settings", icons.ActionSettings)
	settingsPage := NewSettingsPage()
	settingsPage.SyncStartupSwitch(services.Startup)
	return &Shell{
		services:        services,
		logsItem:        logsItem,
		screenshotsItem: screenshotsItem,
		settingsItem:    settingsItem,
		sidebar:         NewSidebar(NewLogo(assets.Logo()), logsItem, screenshotsItem, settingsItem),
		logsPage:        NewLogsPage(invalidate),
		screenshotsPage: NewScreenshotsPage(invalidate),
		settingsPage:    settingsPage,
		dataDirPrompt:   &dataDirPrompt{form: newDataDirForm()},
		startupPrompt:   &startupPrompt{},
	}
}

// Layout draws the full window.
func (s *Shell) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if s.services.Startup != nil && s.startupPrompt.Update(gtx, s.services) {
		s.settingsPage.SyncStartupSwitch(s.services.Startup)
	}
	paint.Fill(gtx.Ops, colorAppBackground)
	dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.sidebar.Layout(gtx, theme)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return s.layoutPage(gtx, theme)
		}),
	)
	s.layoutPrompt(gtx, theme)
	return dims
}

func (s *Shell) layoutPage(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	selected := s.sidebar.SelectedItem()
	if selected != s.shownItem {
		s.shownItem = selected
		s.refreshPage(selected)
	}
	switch selected {
	case s.settingsItem:
		return s.settingsPage.Layout(gtx, theme, s.services)
	case s.screenshotsItem:
		return s.screenshotsPage.Layout(gtx, theme, s.services)
	}
	return s.logsPage.Layout(gtx, theme, s.services)
}

// refreshPage reloads what a page shows each time the user opens it.
func (s *Shell) refreshPage(item *NavItem) {
	switch item {
	case s.logsItem:
		s.logsPage.Reload(s.services.Logs)
	case s.screenshotsItem:
		s.screenshotsPage.Reload(s.services.Screenshots)
	}
}

// layoutPrompt shows at most one prompt: the data folder first, since capture needs it.
func (s *Shell) layoutPrompt(gtx layout.Context, theme *material.Theme) {
	if s.services.Capture.Status().State == capture.StateNeedsDataDir {
		s.dataDirPrompt.Layout(gtx, theme, s.services)
		return
	}
	if s.services.Startup != nil && !s.services.Settings.Get().HasAnsweredStartupPrompt {
		s.startupPrompt.Layout(gtx, theme)
	}
}

func pageTitle(theme *material.Theme, text string) material.LabelStyle {
	title := material.Label(theme, textSizePageTitle, text)
	title.Font.Weight = font.SemiBold
	return title
}

func caption(theme *material.Theme, text string) material.LabelStyle {
	label := material.Label(theme, textSizeCaption, text)
	label.Color = colorTextSecondary
	return label
}
