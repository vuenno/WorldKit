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
)

// Run drives the window's event loop until the window is closed.
func Run(window *app.Window) error {
	theme := NewTheme()
	shell := NewShell()
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

// Shell is the top-level layout: sidebar on the left, the selected page on the right.
type Shell struct {
	logo     *Logo
	homeItem *NavItem
	sidebar  *Sidebar
}

// NewShell creates the shell with WorldKit's navigation destinations.
func NewShell() *Shell {
	logo := NewLogo(assets.Logo())
	homeItem := NewNavItem("Home", icons.ActionHome)
	return &Shell{
		logo:     logo,
		homeItem: homeItem,
		sidebar: NewSidebar(logo,
			homeItem,
			NewNavItem("Explore", icons.ActionExplore),
			NewNavItem("Map", icons.MapsMap),
			NewNavItem("Library", icons.AVLibraryBooks),
			NewNavItem("Settings", icons.ActionSettings),
		),
	}
}

// Layout draws the full window.
func (s *Shell) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	paint.Fill(gtx.Ops, colorAppBackground)
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return s.sidebar.Layout(gtx, theme)
		}),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			if s.sidebar.SelectedItem() == s.homeItem {
				return s.layoutHomePage(gtx, theme)
			}
			return s.layoutPlaceholderPage(gtx, theme)
		}),
	)
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
