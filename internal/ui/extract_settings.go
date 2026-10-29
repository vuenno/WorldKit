package ui

import (
	"log/slog"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget/material"

	"world-kit/internal/settings"
)

// extractSettings is the Settings card for Log Screenshots: the saved Extract image
// presets (made in the Extract dialog; here they are made the default or deleted).
type extractSettings struct {
	rows      presetRows
	errorText string
}

func newExtractSettings() *extractSettings {
	return &extractSettings{rows: presetRows{}}
}

func (e *extractSettings) update(gtx layout.Context, store SettingsStore) {
	for _, preset := range store.Get().ExtractPresets {
		state, name := e.rows.row(preset.Name), preset.Name
		if state.makeDefault.Clicked(gtx) {
			e.change(store, func(s *settings.Settings) { s.DefaultExtractPreset = name })
		}
		if state.remove.Clicked(gtx) {
			e.change(store, func(s *settings.Settings) { s.DeleteExtractPreset(name) })
		}
	}
}

func (e *extractSettings) change(store SettingsStore, change func(*settings.Settings)) {
	e.errorText = ""
	if err := store.Update(change); err != nil {
		slog.Error("save extract presets", "error", err)
		e.errorText = "Couldn't save the change."
	}
}

// Layout handles input, then draws the card.
func (e *extractSettings) Layout(gtx layout.Context, theme *material.Theme, store SettingsStore) layout.Dimensions {
	e.update(gtx, store)
	current := store.Get()
	return settingsCard{title: "Log Screenshots", rows: []layout.Widget{
		func(gtx layout.Context) layout.Dimensions { return e.layoutPresets(gtx, theme, current) },
	}}.Layout(gtx, theme)
}

func (e *extractSettings) layoutPresets(gtx layout.Context, theme *material.Theme, current settings.Settings) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(styledLabel(theme, textSizeSmall, font.Medium, colorTextPrimary, "Extract presets").Layout),
		layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
		layout.Rigid(caption(theme, "A background, font size and max width to reuse. Save one from Extract image; the default one is picked first.").Layout),
	}
	if len(current.ExtractPresets) == 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(secondaryText(theme, "No presets yet.").Layout))
	}
	for _, preset := range current.ExtractPresets {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				isDefault := preset.Name == current.DefaultExtractPreset
				return layoutPresetRow(gtx, theme, e.rows.row(preset.Name), preset.Name, describeExtractPreset(preset), isDefault)
			}))
	}
	children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
		return errorCaption(gtx, theme, e.errorText)
	}))
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}
