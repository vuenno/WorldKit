package ui

import (
	"image/color"
	"time"

	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Color tokens. See DESIGN.md for when to use each one.
var (
	colorAppBackground   = rgb(0x0D0D0F)
	colorSurface         = rgb(0x18181B)
	colorElevatedSurface = rgb(0x27272A)
	colorPrimary         = rgb(0xFF6B00)
	colorPrimaryHover    = rgb(0xFF8A33)
	colorTextPrimary     = rgb(0xF5F5F5)
	colorTextSecondary   = rgb(0xA1A1AA)
	colorBorder          = rgb(0x3F3F46)
	colorError           = rgb(0xF87171)
	colorScrim           = withOpacity(colorAppBackground, 0.8)
)

// selectionOpacity tints selected text with the accent without hiding it.
const selectionOpacity = 0.4

const appName = "WorldKit"

// Text sizes.
const (
	textSizeBrand     = unit.Sp(14)
	textSizePageTitle = unit.Sp(16)
	textSizeBody      = unit.Sp(13)
	textSizeNavLabel  = unit.Sp(13)
	textSizeCaption   = unit.Sp(11)
)

// Spacing and component sizes.
const (
	spacingTiny   = unit.Dp(4)
	spacingSmall  = unit.Dp(8)
	spacingMedium = unit.Dp(12)
	spacingLarge  = unit.Dp(24)

	iconSize      = unit.Dp(18)
	navItemHeight = unit.Dp(36)
	cornerRadius  = unit.Dp(6)
	borderWidth   = unit.Dp(1)

	logoSizeSidebar = unit.Dp(28)
	logoSizeHero    = unit.Dp(160)

	buttonHeight    = unit.Dp(32)
	buttonPaddingX  = spacingMedium
	inputPaddingY   = spacingSmall
	dialogWidth     = unit.Dp(420)
	contentMaxWidth = unit.Dp(640)
)

// Sidebar dimensions and motion.
const (
	sidebarPadding        = spacingSmall
	navItemPaddingX       = spacingMedium
	sidebarExpandedWidth  = unit.Dp(200)
	sidebarCollapsedWidth = 2*(sidebarPadding+navItemPaddingX) + iconSize
	activeIndicatorWidth  = unit.Dp(3)
	sidebarToggleDuration = 200 * time.Millisecond
)

// NewTheme returns the single Material theme shared by every screen.
func NewTheme() *material.Theme {
	theme := material.NewTheme()
	theme.Palette = material.Palette{
		Bg:         colorAppBackground,
		Fg:         colorTextPrimary,
		ContrastBg: colorPrimary,
		ContrastFg: colorTextPrimary,
	}
	theme.TextSize = textSizeBody
	return theme
}

func rgb(hex uint32) color.NRGBA {
	return color.NRGBA{R: uint8(hex >> 16), G: uint8(hex >> 8), B: uint8(hex), A: 0xFF}
}

func withOpacity(c color.NRGBA, opacity float32) color.NRGBA {
	c.A = uint8(float32(c.A) * opacity)
	return c
}
