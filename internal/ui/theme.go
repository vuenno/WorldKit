package ui

import (
	"image/color"
	"time"

	"gioui.org/unit"
	"gioui.org/widget/material"
)

// Color tokens. See DESIGN.md for when to use each one.
var (
	colorAppBackground   = rgb(0x0E0F12) // window ground, inputs on cards
	colorSidebar         = rgb(0x121317) // sidebar and the chat log panel
	colorSurface         = rgb(0x16181D) // cards, list rows, secondary buttons
	colorElevatedSurface = rgb(0x1D2027) // hovered rows, menus, chips, the active nav item
	colorPressed         = rgb(0x2A2E37) // the chosen segment of a segmented control
	colorBorder          = rgb(0x2A2E37) // control outlines
	colorDivider         = rgb(0x23262E) // card outlines and separators
	colorBorderStrong    = rgb(0x343843) // floating menus and toasts
	colorPrimary         = rgb(0xFF7A1A)
	colorPrimaryHover    = rgb(0xFF8F3D)
	colorOnPrimary       = rgb(0x1A0E05) // text and icons on orange; white on orange fails contrast
	colorLink            = rgb(0xFF9A4D) // text-only actions ("Clear filters", "View")
	colorTextPrimary     = rgb(0xECEDEF)
	colorTextSecondary   = rgb(0x9AA0AB)
	colorTextTertiary    = rgb(0x8A909B) // group labels, hints, icons at rest
	colorTimestamp       = rgb(0x6E7480) // chat timestamps and chevrons: decoration, not content
	colorLive            = rgb(0x3DD68C) // capturing / live session, always next to a text label
	colorWarning         = rgb(0xF5B54A) // capture retrying
	colorDanger          = rgb(0xF28B82) // delete actions and errors
	colorDangerHover     = rgb(0xF5A39C)
	colorCloseHover      = rgb(0xE81123) // the window close button on hover, as in Windows
	colorScrim           = withOpacity(colorAppBackground, 0.8)
	colorShadow          = color.NRGBA{A: 0x73}
)

// Chat colors: how GTA World's in-game chat draws each kind of line, so saved logs look
// the way they did in game. Values from the GTA World Chatlog Magician, a community tool
// that recreates the in-game chat. See DESIGN.md.
var (
	colorChatSay     = rgb(0xF1F1F1) // says, shouts, server messages
	colorChatLow     = rgb(0xC6C4C4) // says [low]
	colorChatLower   = rgb(0x939799) // says [lower]: quieter, so darker
	colorChatWhisper = rgb(0xEDA841)
	colorChatEmote   = rgb(0xC2A3DA) // /me and /do
	colorChatOOC     = rgb(0xB8BEC7) // /b
	colorChatPM      = rgb(0xFBF724)
	colorChatHeader  = rgb(0x939799) // session start line
)

// Extra text colors offered by the extract dialog's color picker: the in-game color codes
// (~r~, ~g~, ~b~, ~c~, ~m~) as the Chatlog Magician draws them.
var (
	colorChatRed   = rgb(0xF00000)
	colorChatGreen = rgb(0x56D64B)
	colorChatBlue  = rgb(0x3896F3)
	colorChatCyan  = rgb(0x33C1C9)
	colorChatPink  = rgb(0xFF00BC)
)

// textSwatches are the extract dialog's color picker, in order: chat colors first.
var textSwatches = []color.NRGBA{
	colorChatSay, colorChatLow, colorChatLower, colorChatEmote, colorChatWhisper, colorChatPM,
	colorChatRed, colorChatGreen, colorChatBlue, colorChatCyan, colorChatPink,
}

// Screenshot backdrops: a checkerboard says "transparent" at a glance. Dark and Light are
// boxes behind each row of text, in the saved PNG and in previews; dark is the Chatlog
// Magician's black.
var (
	colorCheckerLight  = rgb(0x22252C)
	colorCheckerDark   = rgb(0x1B1D23)
	colorBackdropDark  = rgb(0x000000)
	colorBackdropLight = rgb(0xD9DCE1)
)

// Opacities for accent tints.
const (
	selectionOpacity = 0.4 // selected text in inputs
	// Gio blends in linear light, so tints look stronger than the same alpha in a browser;
	// these values match the design's 12% tints.
	lineSelectionOpacity = 0.07 // selected chat lines, light enough to keep chat colors readable
	liveTintOpacity      = 0.06 // the background of status pills
	lineDragOpacity      = 0.15 // a chat line being dragged on a screenshot preview
)

const appName = "WorldKit"

// Text sizes.
const (
	textSizePageTitle   = unit.Sp(28) // top-level pages: Logs, Log Screenshots, Settings
	textSizeScreenTitle = unit.Sp(20) // opened items: a log, a screenshot; dialog titles
	textSizeSection     = unit.Sp(15) // card and section titles, the brand name
	textSizeBody        = unit.Sp(14) // body text, buttons, nav labels, chat lines
	textSizeSmall       = unit.Sp(13) // secondary text, compact controls
	textSizeCaption     = unit.Sp(12) // hints, group labels, timestamps
	textSizeTiny        = unit.Sp(11) // the brand subtitle, badges
	textSizeSessionTime = unit.Sp(16) // a session's start time in the list (Geist Mono)
)

// Spacing follows a 4 dp grid.
const (
	spacingTiny   = unit.Dp(4)
	spacingSmall  = unit.Dp(8)
	spacingMedium = unit.Dp(12)
	spacingLarge  = unit.Dp(16)
	spacingXLarge = unit.Dp(24)
	spacingPage   = unit.Dp(40) // horizontal page padding
)

// Component sizes.
const (
	iconSize       = unit.Dp(18)
	iconSizeSmall  = unit.Dp(15)
	iconSizeButton = unit.Dp(16)
	iconSizeTiny   = unit.Dp(14)
	borderWidth    = unit.Dp(1)

	radiusSmall    = unit.Dp(7)  // segments
	radiusMenu     = unit.Dp(8)  // small buttons in floating menus and cards
	radiusControl  = unit.Dp(10) // buttons, inputs, nav items
	radiusCard     = unit.Dp(14) // cards, the chat panel, dialogs
	radiusFloating = unit.Dp(12) // menus and toasts

	controlHeight        = unit.Dp(40) // buttons, inputs, nav items
	controlHeightCompact = unit.Dp(36) // toolbar controls in the log viewer
	controlHeightSmall   = unit.Dp(32) // buttons inside floating menus and cards
	segmentHeight        = unit.Dp(28)
	pillHeight           = unit.Dp(24)
	pillDotSize          = unit.Dp(6)
	chipPaddingX         = unit.Dp(10)
	chipIconSize         = unit.Dp(12)
	badgeHeight          = unit.Dp(20)
	segmentedPadding     = unit.Dp(3)  // around the segments, and the switch knob's inset
	dividerHeight        = unit.Dp(20) // vertical dividers in toolbars
	switchWidth          = unit.Dp(40)
	switchHeight         = unit.Dp(24)
	buttonPaddingX       = unit.Dp(14)
	stepperValueWidth    = unit.Dp(52) // the value between a number stepper's − and +

	logoSizeSidebar  = unit.Dp(32)
	dialogWidth      = unit.Dp(440)
	contentMaxWidth  = unit.Dp(960) // Logs list
	settingsMaxWidth = unit.Dp(720)
	searchFieldWidth = unit.Dp(300)
	findFieldWidth   = unit.Dp(240)

	chatLineHeight          = unit.Sp(22)
	chatLinePaddingY        = unit.Dp(3)
	chatTimestampWidth      = unit.Dp(76)
	sessionTimeColumnWidth  = unit.Dp(72)
	sessionStatsColumnWidth = unit.Dp(140)
	kindBarWidth            = unit.Dp(120)
	kindBarHeight           = unit.Dp(4)
	characterMenuWidth      = unit.Dp(260)
	characterMenuMaxHeight  = unit.Dp(300)

	screenshotCardMinWidth  = unit.Dp(280)
	screenshotPreviewHeight = unit.Dp(140)
	screenshotDetailsWidth  = unit.Dp(300)
	detailLabelWidth        = unit.Dp(64)
	checkerSquare           = unit.Dp(8)

	extractDialogWidth = unit.Dp(720)
	optionLabelWidth   = unit.Dp(88) // "Background", "Font size": lines up the controls after them
	swatchSize         = unit.Dp(22)
	swatchRingWidth    = unit.Dp(2) // the ring around a hovered color swatch
	caretWidth         = unit.Dp(1) // the caret when editing text on a screenshot preview
	// lineGripWidth is the margin left of an edited preview that holds each line's grip.
	lineGripWidth       = unit.Dp(20)
	dropIndicatorHeight = unit.Dp(2) // the bar marking where a dragged line will land
)

// Screenshot Editor and Saved Screenshots.
const (
	editorPagePadding    = spacingXLarge // narrower than spacingPage: the canvas needs the room
	inspectorWidth       = unit.Dp(216)  // compact, like a design tool's panel: the canvas gets the room
	inspectorFieldWidth  = unit.Dp(60)   // rotation and percent fields
	panelPadding         = spacingMedium
	panelControlHeight   = unit.Dp(24) // fields, segments, icon buttons in the editor's panels
	panelLabelWidth      = unit.Dp(52) // "Position", "Opacity": lines up the panel's controls
	panelRowGap          = unit.Dp(6)
	panelSegmentHeight   = unit.Dp(20) // a compact segment, inside segmentedPadding
	templateTileMinWidth = unit.Dp(168)
	templateShapeHeight  = unit.Dp(72) // the area a template tile draws its aspect outline in
	templateShapeStroke  = unit.Dp(1.5)
	canvasMargin         = unit.Dp(32) // around the fitted canvas in its area
	handleSize           = unit.Dp(9)
	handleHitSize        = unit.Dp(18) // handles are easier to grab than they look
	rotateHandleOffset   = unit.Dp(26) // the rotation handle above the frame's top edge
	snapDistance         = unit.Dp(6)
	layerRowHeight       = unit.Dp(34)
	layerThumbSize       = unit.Dp(24)
	exportThumbWidth     = unit.Dp(96)
	exportThumbHeight    = unit.Dp(54)
	sliderKnobSize       = unit.Dp(16)
	sliderTrackHeight    = unit.Dp(4)
	menuWidth            = unit.Dp(260)
	menuMaxHeight        = unit.Dp(360)
	navBadgeSize         = unit.Dp(6) // the dot on a nav item with work in progress
	checkBadgeSize       = unit.Dp(22)
	checkboxSize         = unit.Dp(16) // a toggle row in a menu (Characters)
	checkboxRadius       = unit.Dp(4)

	// offCanvasOpacity is how strongly the selected layer's parts outside the canvas show.
	offCanvasOpacity = 0.25
	// disabledOpacity fades controls that can't be used right now (Undo with nothing to undo).
	disabledOpacity = 0.4
)

// Pixel sizes of the editor's images (pixels, not dp: they describe decoded images).
const (
	previewMaxSide   = 2048 // a layer's preview texture: sharp enough, and safe on every GPU
	thumbnailMaxSide = 128  // layer rows and Saved Screenshots rows
	// memoryWarningBytes is how much image memory (CPU copies plus GPU textures with
	// their mipmaps) the editor holds before it suggests using fewer or smaller images.
	memoryWarningBytes = 600 << 20
)

// Window chrome: WorldKit draws its own title bar instead of the Windows one.
const (
	titleBarHeight    = unit.Dp(40)
	windowButtonWidth = unit.Dp(46) // the Windows caption button width
	windowGlyphSize   = unit.Dp(10)
	windowGlyphStroke = unit.Dp(1)
)

// Sidebar dimensions and motion.
const (
	sidebarPadding        = spacingMedium
	navItemPaddingX       = spacingMedium
	navItemHeight         = controlHeight
	sidebarExpandedWidth  = unit.Dp(232)
	sidebarCollapsedWidth = 2*(sidebarPadding+navItemPaddingX) + iconSize
	activeIndicatorWidth  = unit.Dp(3)
	activeIndicatorHeight = unit.Dp(20)
	statusDotSize         = unit.Dp(8)
	sidebarToggleDuration = 200 * time.Millisecond
)

// toastDuration is how long a confirmation toast ("Screenshot saved") stays up.
const toastDuration = 4 * time.Second

// NewTheme returns the single Material theme shared by every screen, set in Geist.
func NewTheme() *material.Theme {
	theme := material.NewTheme()
	theme.Shaper = newShaper()
	theme.Face = fontUI
	theme.Palette = material.Palette{
		Bg:         colorAppBackground,
		Fg:         colorTextPrimary,
		ContrastBg: colorPrimary,
		ContrastFg: colorOnPrimary,
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
