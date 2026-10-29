package ui

import (
	"fmt"
	"log/slog"

	"gioui.org/layout"
	"gioui.org/widget/material"

	"world-kit/internal/screenshot"
	"world-kit/internal/settings"
)

// backdropNames are the backdrops as extract presets store them, in segmented-control
// order (backdropTransparent, backdropDark, backdropLight).
var backdropNames = []string{"transparent", "dark", "light"}

// backdropLabels name the backdrops for people, in the same order.
var backdropLabels = []string{"Transparent", "Dark", "Light"}

// backdropIndex returns the backdrop a preset names; unknown names are transparent.
func backdropIndex(name string) int {
	for i, known := range backdropNames {
		if known == name {
			return i
		}
	}
	return backdropTransparent
}

// updatePresets applies a preset picked from the menu and saves the current look as one,
// and reports whether the look changed.
func (d *extractDialog) updatePresets(gtx layout.Context) bool {
	presets := d.settings.Get().ExtractPresets
	isChanged := false
	if index, ok := d.presetMenu.Update(gtx, len(presets)); ok {
		d.applyPreset(presets[index])
		isChanged = true
	}
	name, ok := d.naming.Update(gtx, extractPresetSuggestion(d.currentPreset("")))
	if !ok {
		return isChanged
	}
	preset := d.currentPreset(name)
	if err := d.settings.Update(func(s *settings.Settings) { s.SaveExtractPreset(preset) }); err != nil {
		slog.Error("save extract preset", "error", err)
		d.errText = "Couldn't save the preset."
		return isChanged
	}
	d.presetName = name
	d.naming.Saved(name)
	return isChanged
}

// currentPreset is the dialog's look as a preset called name. The in-game width is saved
// as zero, so the preset keeps meaning "as in game".
func (d *extractDialog) currentPreset(name string) settings.ExtractPreset {
	look := d.look()
	preset := settings.ExtractPreset{Name: name, Backdrop: backdropNames[d.backdrop.selected], ChatFont: look.Font.String(), ChatScale: look.ChatScale}
	if d.widthLimit.value != screenshot.DefaultWidthLimit() {
		preset.WidthLimit = d.widthLimit.value
	}
	return preset
}

// applyPresetNamed applies the saved preset called name, if there is one.
func (d *extractDialog) applyPresetNamed(name string) {
	if preset, ok := d.settings.Get().ExtractPreset(name); ok {
		d.applyPreset(preset)
	}
}

// applyPreset sets the background, chat font and scale, and max width from preset.
func (d *extractDialog) applyPreset(preset settings.ExtractPreset) {
	look := presetLook(preset)
	d.backdrop.selected = backdropIndex(preset.Backdrop)
	d.chatFont.selected = chatFontIndex(look.Font)
	d.chatScale.value = look.ChatScale
	d.widthLimit.value = screenshot.ClampWidthLimit(preset.WidthLimit)
	d.presetName = preset.Name
}

// presetLook is a preset's chat font and scale. A preset saved before chat scales has a
// font size instead; it gets the nearest scale.
func presetLook(preset settings.ExtractPreset) screenshot.Look {
	scale := screenshot.ClampChatScale(preset.ChatScale)
	if preset.ChatScale == 0 && preset.FontSize > 0 {
		scale = screenshot.ChatScaleForFontSize(preset.FontSize)
	}
	return screenshot.Look{Font: screenshot.ParseChatFont(preset.ChatFont), ChatScale: scale}
}

func chatFontIndex(chatFont screenshot.ChatFont) int {
	for i, known := range screenshot.ChatFonts {
		if known == chatFont {
			return i
		}
	}
	return 0
}

// extractPresetSuggestion names a preset after its look: "Dark · Arial 60% · 500 wide".
func extractPresetSuggestion(preset settings.ExtractPreset) string {
	name := fmt.Sprintf("%s · %s", backdropLabels[backdropIndex(preset.Backdrop)], presetLook(preset))
	if preset.WidthLimit > 0 {
		name += fmt.Sprintf(" · %d wide", preset.WidthLimit)
	}
	return name
}

// describeExtractPreset sums up a preset for Settings and the menu:
// "Dark · Arial 60% · max 500 px".
func describeExtractPreset(preset settings.ExtractPreset) string {
	width := "in-game width"
	if preset.WidthLimit > 0 {
		width = fmt.Sprintf("max %d px", preset.WidthLimit)
	}
	return fmt.Sprintf("%s · %s · %s", backdropLabels[backdropIndex(preset.Backdrop)], presetLook(preset), width)
}

// layoutPresetRow is the Preset dropdown, shown once any extract preset is saved.
func (d *extractDialog) layoutPresetRow(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	presets := d.settings.Get().ExtractPresets
	if len(presets) == 0 {
		return layout.Dimensions{}
	}
	items := make([]menuItem, len(presets))
	for i, preset := range presets {
		items[i] = menuItem{label: preset.Name, detail: presetLook(preset).String()}
	}
	label := d.presetName
	if label == "" {
		label = customPresetLabel
	}
	return layout.Inset{Top: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layoutOptionRow(gtx, theme, "Preset", func(gtx layout.Context, theme *material.Theme) layout.Dimensions {
			return d.presetMenu.LayoutButton(gtx, theme, label, false, items)
		}, "A saved background, chat font, scale and width.")
	})
}
