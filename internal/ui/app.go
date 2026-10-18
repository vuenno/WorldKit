// Package ui contains WorldKit's screens, widgets and theme.
package ui

import (
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
	shell := NewShell(services)
	var ops op.Ops

	for {
		switch event := window.Event().(type) {
		case app.DestroyEvent:
			return event.Err
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
	services      Services
	logo          *Logo
	homeItem      *NavItem
	settingsItem  *NavItem
	sidebar       *Sidebar
	settingsPage  *SettingsPage
	dataDirPrompt *dataDirPrompt
	startupPrompt *startupPrompt
}

// NewShell creates the shell with WorldKit's navigation destinations.
func NewShell(services Services) *Shell {
	logo := NewLogo(assets.Logo())
	homeItem := NewNavItem("Home", icons.ActionHome)
	settingsItem := NewNavItem("Settings", icons.ActionSettings)
	settingsPage := NewSettingsPage()
	settingsPage.SyncStartupSwitch(services.Startup)
	return &Shell{
		services:     services,
		logo:         logo,
		homeItem:     homeItem,
		settingsItem: settingsItem,
		sidebar: NewSidebar(logo,
			homeItem,
			NewNavItem("Explore", icons.ActionExplore),
			NewNavItem("Map", icons.MapsMap),
			NewNavItem("Library", icons.AVLibraryBooks),
			settingsItem,
		),
		settingsPage:  settingsPage,
		dataDirPrompt: &dataDirPrompt{form: newDataDirForm()},
		startupPrompt: &startupPrompt{},
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
	switch s.sidebar.SelectedItem() {
	case s.homeItem:
		return s.layoutHomePage(gtx, theme)
	case s.settingsItem:
		return s.settingsPage.Layout(gtx, theme, s.services)
	default:
		return s.layoutPlaceholderPage(gtx, theme)
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

// layoutHomePage shows the logo as a centered welcome hero.
func (s *Shell) layoutHomePage(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical, Alignment: layout.Middle}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return s.logo.Layout(gtx, logoSizeHero)
			}),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return pageTitle(theme, "Welcome to "+appName).Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return caption(theme, "Pick a section from the sidebar to get started.").Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return caption(theme, captureStatusText(s.services.Capture.Status())).Layout(gtx)
			}),
		)
	})
}

func (s *Shell) layoutPlaceholderPage(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	pageName := s.sidebar.SelectedItem().Label
	return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pageTitle(theme, pageName).Layout),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(caption(theme, "This page is empty for now.").Layout),
		)
	})
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
