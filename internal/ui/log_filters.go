package ui

import (
	"fmt"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
)

// filterOption is one on/off choice in the filter panel.
type filterOption struct {
	label string
	click widget.Clickable
	isOn  bool
}

// actionOption keeps lines of the given kinds.
type actionOption struct {
	filterOption
	kinds []chatlog.Kind
}

// characterOption keeps lines spoken by or mentioning one character.
type characterOption struct {
	filterOption
	name string
}

// inCharacterKinds is everything said or done in character: all lines except OOC (/b), PMs
// and server messages.
var inCharacterKinds = []chatlog.Kind{
	chatlog.KindSay, chatlog.KindShout, chatlog.KindWhisper, chatlog.KindMe, chatlog.KindDo,
}

// logFilters is the log viewer's filter panel: action (line kind) options and one option per
// character detected in the session. Options in a group combine with "or"; the two groups
// combine with "and". Nothing selected in a group keeps everything.
//
// The Display options change how lines look, not which lines show, so they don't count as
// active filters and Clear leaves them alone.
type logFilters struct {
	actions       []*actionOption
	characters    []*characterOption
	timestamps    filterOption
	clear         widget.Clickable
	characterList layout.List
}

func newLogFilters() *logFilters {
	return &logFilters{
		actions: []*actionOption{
			{filterOption: filterOption{label: "/me"}, kinds: []chatlog.Kind{chatlog.KindMe}},
			{filterOption: filterOption{label: "/do"}, kinds: []chatlog.Kind{chatlog.KindDo}},
			{filterOption: filterOption{label: "/b (OOC)"}, kinds: []chatlog.Kind{chatlog.KindOOC}},
			{filterOption: filterOption{label: "IC"}, kinds: inCharacterKinds},
		},
		timestamps:    filterOption{label: "Timestamps", isOn: true},
		characterList: layout.List{Axis: layout.Vertical},
	}
}

// ShowsTimestamps reports whether lines keep their "[HH:MM:SS]" timestamps.
func (f *logFilters) ShowsTimestamps() bool {
	return f.timestamps.isOn
}

// SetCharacters lists the session's characters as options, keeping earlier choices by name.
func (f *logFilters) SetCharacters(characters []chatlog.Character) {
	wasOn := map[string]bool{}
	for _, option := range f.characters {
		wasOn[option.name] = option.isOn
	}
	f.characters = make([]*characterOption, len(characters))
	for i, character := range characters {
		f.characters[i] = &characterOption{
			filterOption: filterOption{label: characterLabel(character), isOn: wasOn[character.Name]},
			name:         character.Name,
		}
	}
}

func characterLabel(character chatlog.Character) string {
	return fmt.Sprintf("%s · %d", character.Name, character.LineCount)
}

// Update handles option clicks and reports whether the filter or display changed.
func (f *logFilters) Update(gtx layout.Context) bool {
	isChanged := false
	for _, option := range append(f.options(), &f.timestamps) {
		if option.click.Clicked(gtx) {
			option.isOn = !option.isOn
			isChanged = true
		}
	}
	if f.clear.Clicked(gtx) {
		for _, option := range f.options() {
			option.isOn = false
		}
		isChanged = true
	}
	return isChanged
}

func (f *logFilters) options() []*filterOption {
	return append(f.actionOptions(), f.characterOptions()...)
}

// Filter returns the chosen options as a chatlog filter.
func (f *logFilters) Filter() chatlog.Filter {
	var filter chatlog.Filter
	for _, action := range f.actions {
		if action.isOn {
			filter.Kinds = append(filter.Kinds, action.kinds...)
		}
	}
	for _, character := range f.characters {
		if character.isOn {
			filter.Names = append(filter.Names, character.name)
		}
	}
	return filter
}

// ActiveCount returns how many options are on.
func (f *logFilters) ActiveCount() int {
	count := 0
	for _, option := range f.options() {
		if option.isOn {
			count++
		}
	}
	return count
}

// Layout draws the panel: actions, then characters in a height-capped scrolling area.
func (f *logFilters) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return panel(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.layoutActionsHeader(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutWrapped(gtx, spacingSmall, f.optionWidgets(theme, f.actionOptions()))
			}),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.layoutCharactersHeader(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return f.layoutCharacters(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(sectionTitle(theme, "Display").Layout),
			layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return layoutWrapped(gtx, spacingSmall, f.optionWidgets(theme, []*filterOption{&f.timestamps}))
			}),
		)
	})
}

func (f *logFilters) layoutActionsHeader(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		// Reserves the Clear button's height so the panel doesn't jump when it appears.
		layout.Rigid(layout.Spacer{Height: buttonHeight}.Layout),
		layout.Flexed(1, sectionTitle(theme, "Actions").Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			if f.ActiveCount() == 0 {
				return layout.Dimensions{}
			}
			return secondaryButton(&f.clear, "Clear").Layout(gtx, theme)
		}),
	)
}

func (f *logFilters) layoutCharactersHeader(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}
	return row.Layout(gtx,
		layout.Rigid(sectionTitle(theme, "Characters").Layout),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(caption(theme, fmt.Sprintf("%d detected", len(f.characters))).Layout),
	)
}

func (f *logFilters) layoutCharacters(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if len(f.characters) == 0 {
		return caption(theme, "No characters found in this session.").Layout(gtx)
	}
	gtx.Constraints.Max.Y = min(gtx.Constraints.Max.Y, gtx.Dp(characterOptionsMaxHeight))
	return f.characterList.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return layoutWrapped(gtx, spacingSmall, f.optionWidgets(theme, f.characterOptions()))
	})
}

func (f *logFilters) actionOptions() []*filterOption {
	options := make([]*filterOption, len(f.actions))
	for i, action := range f.actions {
		options[i] = &action.filterOption
	}
	return options
}

func (f *logFilters) characterOptions() []*filterOption {
	options := make([]*filterOption, len(f.characters))
	for i, character := range f.characters {
		options[i] = &character.filterOption
	}
	return options
}

func (f *logFilters) optionWidgets(theme *material.Theme, options []*filterOption) []layout.Widget {
	widgets := make([]layout.Widget, len(options))
	for i, option := range options {
		widgets[i] = func(gtx layout.Context) layout.Dimensions {
			return toggleButton(&option.click, option.label, option.isOn).Layout(gtx, theme)
		}
	}
	return widgets
}
