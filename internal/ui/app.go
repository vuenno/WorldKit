// Package ui contains WorldKit's screens, widgets and theme.
package ui

import (
	"log/slog"

	"gioui.org/app"
	"gioui.org/io/system"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/paint"
	"gioui.org/widget/material"

	"world-kit/assets"
	"world-kit/internal/capture"
)

// Run drives the window's event loop until the window is closed. The window should be
// undecorated (app.Decorated(false)): the shell draws its own title bar.
func Run(window *app.Window, services Services) error {
	theme := NewTheme()
	shell := NewShell(services, window.Invalidate, window.Perform)
	var ops op.Ops

	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			slog.Info("window destroyed", "error", event.Err)
			return event.Err
		case *app.ClosingEvent:
			slog.Info("window closing")
		case app.Win32ViewEvent:
			shell.services.window = event.HWND // zero once the window is gone
		case app.ConfigEvent:
			shell.titleBar.SetMaximized(event.Config.Mode == app.Maximized)
		case app.FrameEvent:
			gtx := app.NewContext(&ops, event)
			shell.Layout(gtx, theme)
			event.Frame(gtx.Ops)
		}
	}
}

// Shell is the top-level layout: sidebar on the left; on the right the title bar above the
// selected page; and any prompt that needs an answer on top of everything.
type Shell struct {
	services        Services
	logsItem        *NavItem
	screenshotsItem *NavItem
	settingsItem    *NavItem
	shownItem       *NavItem
	sidebar         *Sidebar
	titleBar        *titleBar
	logsPage        *LogsPage
	screenshotsPage *ScreenshotsPage
	settingsPage    *SettingsPage
	dataDirPrompt   *dataDirPrompt
	startupPrompt   *startupPrompt
}

// NewShell creates the shell with WorldKit's navigation destinations. invalidate redraws
// the window when background work finishes; perform carries out the title bar's window
// actions (minimize, maximize, close).
func NewShell(services Services, invalidate func(), perform func(system.Action)) *Shell {
	logsItem := NewNavItem("Logs", iconFileText)
	screenshotsItem := NewNavItem("Log Screenshots", iconImage)
	settingsItem := NewNavItem("Settings", iconSliders)
	settingsPage := NewSettingsPage(invalidate)
	settingsPage.SyncStartupSwitch(services.Startup)
	return &Shell{
		services:        services,
		logsItem:        logsItem,
		screenshotsItem: screenshotsItem,
		settingsItem:    settingsItem,
		sidebar:         NewSidebar(NewLogo(assets.Logo()), []*NavItem{logsItem, screenshotsItem}, settingsItem),
		titleBar:        &titleBar{perform: perform},
		logsPage:        NewLogsPage(invalidate),
		screenshotsPage: NewScreenshotsPage(invalidate),
		settingsPage:    settingsPage,
		dataDirPrompt:   &dataDirPrompt{form: newDataDirForm(invalidate, buttonPrimary)},
		startupPrompt:   &startupPrompt{},
	}
}

// Layout draws the full window.
func (s *Shell) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if s.services.Startup != nil && s.startupPrompt.Update(gtx, s.services) {
		s.settingsPage.SyncStartupSwitch(s.services.Startup)
	}
	s.followViewShotLink(gtx)
	paint.Fill(gtx.Ops, colorAppBackground)
	dims := layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.sidebar.Layout(gtx, theme, s.services.Capture.Status())
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(s.titleBar.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return s.layoutPage(gtx, theme) }),
			)
		}),
	)
	s.layoutPrompt(gtx, theme)
	return dims
}

// followViewShotLink opens a screenshot in Log Screenshots when the log viewer's
// "Screenshot saved · View" toast is clicked.
func (s *Shell) followViewShotLink(gtx layout.Context) {
	viewer := s.logsPage.Viewer()
	if viewer == nil {
		return
	}
	if path, ok := viewer.ViewShotClicked(gtx); ok {
		s.sidebar.Select(s.screenshotsItem)
		s.shownItem = s.screenshotsItem
		s.screenshotsPage.Open(s.services.Screenshots, path)
	}
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
