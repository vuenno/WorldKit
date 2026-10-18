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
	sidebar *Sidebar
}

// NewShell creates the shell with WorldKit's navigation destinations.
func NewShell() *Shell {
	return &Shell{
		sidebar: NewSidebar(
			NewNavItem("Home", icons.ActionHome),
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
			return s.layoutPage(gtx, theme)
		}),
	)
}

func (s *Shell) layoutPage(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	pageName := s.sidebar.SelectedItem().Label
	return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				title := material.Label(theme, textSizePageTitle, pageName)
				title.Font.Weight = font.SemiBold
				return title.Layout(gtx)
			}),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				caption := material.Label(theme, textSizeCaption, "This page is empty for now.")
				caption.Color = colorTextSecondary
				return caption.Layout(gtx)
			}),
		)
	})
}
