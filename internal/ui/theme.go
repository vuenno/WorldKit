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

// Chat colors: how GTA World's in-game chat draws each kind of line, so saved logs look
// the way they did in game. Values from the GTA World Chatlog Magician, a community tool
// that recreates the in-game chat. See DESIGN.md.
var (
	colorChatSay     = rgb(0xF1F1F1) // says, shouts, server messages
	colorChatLow     = rgb(0xC6C4C4) // says [low]
	colorChatWhisper = rgb(0xEDA841)
	colorChatEmote   = rgb(0xC2A3DA) // /me and /do
	colorChatOOC     = rgb(0xB8BEC7) // /b
	colorChatPM      = rgb(0xFBF724)
	colorChatHeader  = rgb(0x939799) // session start line
)

// selectionOpacity tints selected text with the accent without hiding it.
const selectionOpacity = 0.4

// lineSelectionOpacity is lighter for whole selected chat lines, so their colors stay readable.
const lineSelectionOpacity = 0.18

const appName = "WorldKit"

// Text sizes.
const (
	textSizeBrand     = unit.Sp(14)
	textSizePageTitle = unit.Sp(16)
	textSizeBody      = unit.Sp(13)
	textSizeNavLabel  = unit.Sp(13)
	textSizeCaption   = unit.Sp(11)
	textSizeChatLine  = unit.Sp(12)
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

	buttonHeight    = unit.Dp(32)
	menuItemHeight  = unit.Dp(28) // items in popup menus, inside spacingTiny padding
	buttonPaddingX  = spacingMedium
	inputPaddingY   = spacingSmall
	dialogWidth     = unit.Dp(420)
	contentMaxWidth = unit.Dp(640)

	// characterOptionsMaxHeight shows about three and a half rows of character options, so a
	// cut-off row hints that the list scrolls.
	characterOptionsMaxHeight = unit.Dp(132)

	// screenshotPreviewMaxHeight shows about six chat lines of a saved screenshot in its card.
	screenshotPreviewMaxHeight = unit.Dp(120)
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
