package ui

import (
	"fmt"
	"image"
	"strings"

	"gioui.org/font"
	"gioui.org/io/event"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/chatlog"
)

// kindChoice is one segment of the line-kind filter.
type kindChoice struct {
	label string
	kinds []chatlog.Kind // nil keeps every kind
}

// inCharacterKinds is everything said or done in character: all lines except OOC (/b), PMs
// and server messages.
var inCharacterKinds = []chatlog.Kind{
	chatlog.KindSay, chatlog.KindShout, chatlog.KindWhisper, chatlog.KindMe, chatlog.KindDo,
}

var kindChoices = []kindChoice{
	{label: "All"},
	{label: "IC", kinds: inCharacterKinds},
	{label: "/me", kinds: []chatlog.Kind{chatlog.KindMe}},
	{label: "/do", kinds: []chatlog.Kind{chatlog.KindDo}},
	{label: "/b", kinds: []chatlog.Kind{chatlog.KindOOC}},
	{label: "PMs", kinds: []chatlog.Kind{chatlog.KindPM}},
}

// characterOption keeps lines spoken by or mentioning one character.
type characterOption struct {
	name      string
	lineCount int
	isOn      bool
	toggle    widget.Clickable // in the characters menu
	remove    widget.Clickable // on its chip
}

// logFilters is the log viewer's toolbar: find text, one line kind, any number of
// characters (picked from a menu, shown as removable chips) and the timestamps switch.
// Characters combine with "or"; kind, characters and find text combine with "and".
//
// Timestamps change how lines look, not which lines show, so they don't count as a filter
// and Clear filters leaves them alone.
type logFilters struct {
	find            widget.Editor
	kinds           *segmentedControl
	characters      []*characterOption
	characterButton widget.Clickable
	isMenuOpen      bool
	menuList        widget.List
	dismissMenu     widget.Clickable
	clear           widget.Clickable
	timestamps      toggleSwitch
}

func newLogFilters() *logFilters {
	labels := make([]string, len(kindChoices))
	for i, choice := range kindChoices {
		labels[i] = choice.label
	}
	return &logFilters{
		find:     widget.Editor{SingleLine: true},
		kinds:    newSegmentedControl(labels...),
		menuList: widget.List{List: layout.List{Axis: layout.Vertical}},
	}
}

// ShowsTimestamps reports whether lines keep their "[HH:MM:SS]" timestamps.
func (f *logFilters) ShowsTimestamps() bool {
	return f.timestamps.value.Value
}

// SetCharacters lists the session's characters, keeping earlier choices by name.
func (f *logFilters) SetCharacters(characters []chatlog.Character) {
	wasOn := map[string]bool{}
	for _, option := range f.characters {
		wasOn[option.name] = option.isOn
	}
	f.characters = make([]*characterOption, len(characters))
	for i, character := range characters {
		f.characters[i] = &characterOption{name: character.Name, lineCount: character.LineCount, isOn: wasOn[character.Name]}
	}
}

// Update handles every filter control and reports whether the shown lines or their look changed.
func (f *logFilters) Update(gtx layout.Context) bool {
	isChanged := f.kinds.Update(gtx) || f.timestamps.Update(gtx)
	if f.characterButton.Clicked(gtx) {
		f.isMenuOpen = !f.isMenuOpen
	}
	if f.dismissMenu.Clicked(gtx) {
		f.isMenuOpen = false
	}
	for _, option := range f.characters {
		if option.toggle.Clicked(gtx) {
			option.isOn = !option.isOn
			isChanged = true
		}
		if option.remove.Clicked(gtx) {
			option.isOn = false
			isChanged = true
		}
	}
	if f.clear.Clicked(gtx) {
		f.Clear()
		isChanged = true
	}
	for {
		ev, ok := f.find.Update(gtx)
		if !ok {
			return isChanged
		}
		if _, ok := ev.(widget.ChangeEvent); ok {
			isChanged = true
		}
	}
}

// Clear turns every filter off: all kinds, no characters, no find text.
func (f *logFilters) Clear() {
	f.kinds.selected = 0
	for _, option := range f.characters {
		option.isOn = false
	}
	f.find.SetText("")
}

// Filter returns the kind and character choices as a chatlog filter.
func (f *logFilters) Filter() chatlog.Filter {
	filter := chatlog.Filter{Kinds: kindChoices[f.kinds.selected].kinds}
	for _, option := range f.selectedCharacters() {
		filter.Names = append(filter.Names, option.name)
	}
	return filter
}

// Matches reports whether a record contains the find text (ignoring case).
func (f *logFilters) Matches(record chatlog.Record) bool {
	query := strings.ToLower(strings.TrimSpace(f.find.Text()))
	return query == "" || strings.Contains(strings.ToLower(record.Raw), query)
}

// IsActive reports whether any filter hides lines.
func (f *logFilters) IsActive() bool {
	return f.kinds.selected != 0 || len(f.selectedCharacters()) > 0 || strings.TrimSpace(f.find.Text()) != ""
}

func (f *logFilters) selectedCharacters() []*characterOption {
	var selected []*characterOption
	for _, option := range f.characters {
		if option.isOn {
			selected = append(selected, option)
		}
	}
	return selected
}

// LayoutToolbar draws find, kinds and the characters button (and its menu while open),
// wrapping onto more rows in a narrow window, and at the right the timestamps switch,
// which always keeps its room.
func (f *logFilters) LayoutToolbar(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Start}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layoutWrappedCentered(gtx, spacingMedium-spacingTiny/2, []layout.Widget{
				func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Min.X = min(gtx.Dp(findFieldWidth), gtx.Constraints.Max.X)
					gtx.Constraints.Max.X = gtx.Constraints.Min.X
					return textField{editor: &f.find, hint: "Find in log", icon: iconSearch, height: controlHeightCompact}.Layout(gtx, theme)
				},
				func(gtx layout.Context) layout.Dimensions { return f.kinds.Layout(gtx, theme) },
				func(gtx layout.Context) layout.Dimensions { return f.layoutCharacterButton(gtx, theme) },
			})
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			// Centered on the first toolbar row: the switch is shorter than the controls.
			inset := layout.Inset{Top: (controlHeightCompact - switchHeight) / 2}
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return f.layoutTimestampsSwitch(gtx, theme)
			})
		}),
	)
}

// layoutTimestampsSwitch is the "Timestamps" label and its switch, side by side.
func (f *logFilters) layoutTimestampsSwitch(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := secondaryText(theme, "Timestamps")
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Width: spacingMedium - spacingTiny/2}.Layout),
		layout.Rigid(f.timestamps.Layout),
	)
}

func (f *logFilters) layoutCharacterButton(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	selectedCount := len(f.selectedCharacters())
	dims := f.characterButton.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		box := surfaceBox{fill: colorSurface, border: colorBorder, radius: radiusControl, height: controlHeightCompact, paddingX: spacingMedium}
		if f.characterButton.Hovered() || f.isMenuOpen {
			box.fill = colorElevatedSurface
		}
		return box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			pointer.CursorPointer.Add(gtx.Ops)
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconUsers.Layout(gtx, iconSizeSmall, colorTextPrimary)
				}),
				layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
				layout.Rigid(styledLabel(theme, textSizeSmall, font.Medium, colorTextPrimary, "Characters").Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					if selectedCount == 0 {
						return layout.Dimensions{}
					}
					return layout.Inset{Left: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
						return countBadge(gtx, theme, fmt.Sprint(selectedCount))
					})
				}),
				layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return iconChevronDown.Layout(gtx, iconSizeTiny, colorTextTertiary)
				}),
			)
		})
	})
	if f.isMenuOpen {
		f.deferCharacterMenu(gtx, theme, dims.Size.Y)
	}
	return dims
}

// deferCharacterMenu draws the characters menu below the button, on top of everything else
// in the frame, behind a transparent full-window layer that closes it when clicked.
func (f *logFilters) deferCharacterMenu(gtx layout.Context, theme *material.Theme, buttonHeight int) {
	recording := op.Record(gtx.Ops)
	f.layoutDismissLayer(gtx)
	offset := op.Offset(image.Pt(0, buttonHeight+gtx.Dp(spacingTiny+spacingTiny/2))).Push(gtx.Ops)
	f.layoutCharacterMenu(gtx, theme)
	offset.Pop()
	op.Defer(gtx.Ops, recording.Stop())
}

// layoutDismissLayer covers the whole window (the menu's origin is somewhere inside it).
func (f *logFilters) layoutDismissLayer(gtx layout.Context) {
	const reach = 1 << 14 // far beyond any window size, in both directions
	defer op.Offset(image.Pt(-reach, -reach)).Push(gtx.Ops).Pop()
	gtx.Constraints = layout.Exact(image.Pt(2*reach, 2*reach))
	f.dismissMenu.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Dimensions{Size: gtx.Constraints.Min}
	})
}

func (f *logFilters) layoutCharacterMenu(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints = layout.Constraints{Max: image.Pt(gtx.Dp(characterMenuWidth), gtx.Dp(characterMenuMaxHeight))}
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			dims := floatingSurface(gtx)
			defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
			event.Op(gtx.Ops, &f.menuList) // a solid area: clicks between rows don't close the menu
			return dims
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return layout.UniformInset(spacingTiny).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				if len(f.characters) == 0 {
					return layout.UniformInset(spacingSmall).Layout(gtx, caption(theme, "No one spoke in this session.").Layout)
				}
				return styledList(theme, &f.menuList).Layout(gtx, len(f.characters), func(gtx layout.Context, index int) layout.Dimensions {
					return characterMenuRow{option: f.characters[index]}.Layout(gtx, theme)
				})
			})
		}),
	)
}

// characterMenuRow is one character in the menu: a check when on, the name and line count.
type characterMenuRow struct {
	option *characterOption
}

func (r characterMenuRow) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	click := &r.option.toggle
	return click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(controlHeightSmall))
		gtx.Constraints = layout.Exact(size)
		shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusMenu))
		defer shape.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		if click.Hovered() {
			paint.Fill(gtx.Ops, colorPressed)
		}
		inset := layout.Inset{Left: spacingSmall, Right: spacingMedium}
		return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					side := gtx.Dp(iconSize)
					if !r.option.isOn {
						return layout.Dimensions{Size: image.Pt(side, side)}
					}
					return iconCheck.Layout(gtx, iconSize, colorPrimary)
				}),
				layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					label := bodyText(theme, r.option.name)
					label.TextSize = textSizeSmall
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					label := monoText(theme, textSizeCaption, countText(r.option.lineCount))
					label.Color = colorTextTertiary
					return label.Layout(gtx)
				}),
			)
		})
	})
}

// LayoutSummary draws "Showing", a chip per chosen character, the line count and, while
// any filter is on, Clear filters.
func (f *logFilters) LayoutSummary(gtx layout.Context, theme *material.Theme, shown, total int) layout.Dimensions {
	selected := f.selectedCharacters()
	children := []layout.Widget{}
	if len(selected) > 0 {
		children = append(children, caption(theme, "Showing").Layout)
	}
	for _, option := range selected {
		chip := removableChip{remove: &option.remove, label: option.name}
		children = append(children, func(gtx layout.Context) layout.Dimensions { return chip.Layout(gtx, theme) })
	}
	countLabel := fmt.Sprintf("%s of %s", countText(shown), plural(total, "line", "lines"))
	if len(selected) > 0 {
		countLabel = "· " + countLabel
	} else {
		countLabel = "Showing " + countLabel
	}
	children = append(children, caption(theme, countLabel).Layout)
	if f.IsActive() {
		link := linkButton{click: &f.clear, label: "Clear filters"}
		children = append(children, func(gtx layout.Context) layout.Dimensions { return link.Layout(gtx, theme) })
	}
	return layoutWrappedCentered(gtx, spacingSmall, children)
}
