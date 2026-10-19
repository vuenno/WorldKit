package ui

import (
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/filedialog"
	"world-kit/internal/settings"
)

// presetRowState is one preset row's buttons, kept by preset name.
type presetRowState struct {
	makeDefault widget.Clickable
	remove      widget.Clickable
}

// exportSettings is the Settings card for the Screenshot Editor: where Export… opens and
// the saved presets (made from the export dialog; here they are made the default or
// deleted).
type exportSettings struct {
	folder       widget.Editor
	browse       widget.Clickable
	picker       *backgroundLoad[string]
	hasPrefilled bool
	errorText    string
	rows         map[string]*presetRowState
}

func newExportSettings(invalidate func()) *exportSettings {
	return &exportSettings{
		folder: widget.Editor{SingleLine: true, Submit: true},
		picker: newBackgroundLoad[string](invalidate),
		rows:   map[string]*presetRowState{},
	}
}

func (e *exportSettings) row(name string) *presetRowState {
	state := e.rows[name]
	if state == nil {
		state = &presetRowState{}
		e.rows[name] = state
	}
	return state
}

func (e *exportSettings) update(gtx layout.Context, services Services) {
	if !e.hasPrefilled {
		e.folder.SetText(services.ExportDir())
		e.hasPrefilled = true
	}
	e.receivePickedFolder(services)
	if e.browse.Clicked(gtx) && !e.picker.IsLoading() {
		owner, startDir := services.window, e.folder.Text()
		e.picker.Start(func() (string, error) {
			return services.Files.PickFolder(owner, "Select where exports are saved", startDir)
		})
	}
	if wasSubmitted(gtx, &e.folder) {
		e.saveFolder(services, e.folder.Text())
	}
	current := services.Settings.Get()
	for _, preset := range current.ExportPresets {
		state := e.row(preset.Name)
		name := preset.Name
		if state.makeDefault.Clicked(gtx) {
			e.changeSettings(services, func(s *settings.Settings) { s.DefaultPreset = name })
		}
		if state.remove.Clicked(gtx) {
			e.changeSettings(services, func(s *settings.Settings) { s.DeletePreset(name) })
		}
	}
}

func (e *exportSettings) receivePickedFolder(services Services) {
	result, ok := e.picker.Result()
	if !ok || errors.Is(result.err, filedialog.ErrCanceled) {
		return
	}
	if result.err != nil {
		slog.Error("pick export folder", "error", result.err)
		e.errorText = "Couldn't open the folder picker. Type the path and press Enter instead."
		return
	}
	e.folder.SetText(result.value)
	e.saveFolder(services, result.value)
}

// saveFolder remembers dir as where exports start; an empty field goes back to the default.
func (e *exportSettings) saveFolder(services Services, dir string) {
	dir = strings.TrimSpace(dir)
	if dir != "" && !filepath.IsAbs(dir) {
		e.errorText = `Type a full path, such as D:\Screenshots.`
		return
	}
	e.changeSettings(services, func(s *settings.Settings) { s.ExportDir, s.LastExportDir = dir, "" })
	if dir == "" {
		e.folder.SetText(services.DefaultExportDir)
	}
}

func (e *exportSettings) changeSettings(services Services, change func(*settings.Settings)) {
	e.errorText = ""
	if err := services.Settings.Update(change); err != nil {
		slog.Error("save export settings", "error", err)
		e.errorText = "Couldn't save the change."
	}
}

// wasSubmitted reports whether Enter was pressed in editor.
func wasSubmitted(gtx layout.Context, editor *widget.Editor) bool {
	isSubmitted := false
	for {
		event, ok := editor.Update(gtx)
		if !ok {
			return isSubmitted
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			isSubmitted = true
		}
	}
}

// Layout handles input, then draws the card.
func (e *exportSettings) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	e.update(gtx, services)
	current := services.Settings.Get()
	rows := []layout.Widget{func(gtx layout.Context) layout.Dimensions { return e.layoutFolder(gtx, theme, services) }}
	rows = append(rows, func(gtx layout.Context) layout.Dimensions { return e.layoutPresets(gtx, theme, current) })
	return settingsCard{title: "Screenshot Editor", rows: rows}.Layout(gtx, theme)
}

func (e *exportSettings) layoutFolder(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(styledLabel(theme, textSizeSmall, font.Medium, colorTextPrimary, "Export folder").Layout),
		layout.Rigid(layout.Spacer{Height: spacingMedium - spacingTiny/2}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return textField{editor: &e.folder, hint: services.DefaultExportDir, isMono: true}.Layout(gtx, theme)
				}),
				layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
				layout.Rigid(secondaryButton(&e.browse, "Browse…").Widget(theme)),
			)
		}),
		layout.Rigid(layout.Spacer{Height: spacingMedium - spacingTiny/2}.Layout),
		layout.Rigid(caption(theme, "Export… opens here; you can still pick another folder each time. Clear the field and press Enter for the default.").Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, e.errorText) }),
	)
}

func (e *exportSettings) layoutPresets(gtx layout.Context, theme *material.Theme, current settings.Settings) layout.Dimensions {
	children := []layout.FlexChild{
		layout.Rigid(styledLabel(theme, textSizeSmall, font.Medium, colorTextPrimary, "Export presets").Layout),
		layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
		layout.Rigid(caption(theme, "A format, quality and canvas size to reuse. Save one from the export dialog; the default one is picked first.").Layout),
	}
	if len(current.ExportPresets) == 0 {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(secondaryText(theme, "No presets yet.").Layout))
	}
	for _, preset := range current.ExportPresets {
		children = append(children,
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return e.layoutPresetRow(gtx, theme, preset, preset.Name == current.DefaultPreset)
			}))
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func (e *exportSettings) layoutPresetRow(gtx layout.Context, theme *material.Theme, preset settings.ExportPreset, isDefault bool) layout.Dimensions {
	state := e.row(preset.Name)
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := styledLabel(theme, textSizeBody, font.Medium, colorTextPrimary, preset.Name)
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					text := fmt.Sprintf("%s · %d × %d", presetFormat(preset).Describe(), preset.Width, preset.Height)
					label := monoText(theme, textSizeCaption, text)
					label.Color = colorTextSecondary
					return label.Layout(gtx)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if isDefault {
				return statusPill{label: "Default", color: colorPrimary}.Layout(gtx, theme)
			}
			return ghostButton(&state.makeDefault, "Make default").Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(iconButton(&state.remove, iconTrash).withKind(buttonGhost).withHeight(controlHeightSmall).asDanger().Widget(theme)),
	)
}
