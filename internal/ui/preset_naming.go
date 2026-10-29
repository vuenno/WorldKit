package ui

import (
	"strings"

	"gioui.org/io/key"
	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// presetNameMaxLength bounds a preset's name, so it fits menus and Settings rows.
const presetNameMaxLength = 40

// presetNaming is a dialog's "Save as preset…" link. Clicked, it turns into a name field
// (filled with a suggestion) with Save preset and Cancel; after saving it confirms with a
// short note.
type presetNaming struct {
	isNaming  bool
	editor    widget.Editor
	start     widget.Clickable
	save      widget.Clickable
	cancel    widget.Clickable
	savedName string // shown as "Saved …" until the next naming starts
}

func newPresetNaming() *presetNaming {
	return &presetNaming{editor: widget.Editor{SingleLine: true, Submit: true, MaxLen: presetNameMaxLength}}
}

// Reset closes the name field and clears the note, e.g. when the dialog opens again.
func (n *presetNaming) Reset() {
	n.isNaming, n.savedName = false, ""
}

// Update handles the link, the field and its buttons. It returns the name to save the
// preset under when the user clicks Save preset or presses Enter; suggestion fills the
// field when naming starts.
func (n *presetNaming) Update(gtx layout.Context, suggestion string) (string, bool) {
	if n.start.Clicked(gtx) {
		n.isNaming, n.savedName = true, ""
		n.editor.SetText(suggestion)
		gtx.Execute(key.FocusCmd{Tag: &n.editor})
	}
	if n.cancel.Clicked(gtx) {
		n.isNaming = false
	}
	isSubmitted := wasSubmitted(gtx, &n.editor)
	if !n.isNaming || (!n.save.Clicked(gtx) && !isSubmitted) {
		return "", false
	}
	name := strings.TrimSpace(n.editor.Text())
	return name, name != ""
}

// Saved closes the field and confirms that the preset was saved.
func (n *presetNaming) Saved(name string) {
	n.isNaming, n.savedName = false, name
}

// Layout draws the link (and the note after a save), or the name field with its buttons.
func (n *presetNaming) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	if !n.isNaming {
		return row.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return linkButton{click: &n.start, label: "Save as preset…"}.Layout(gtx, theme)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if n.savedName == "" {
					return layout.Dimensions{}
				}
				return layout.Inset{Left: spacingMedium}.Layout(gtx, caption(theme, "Saved “"+n.savedName+"”. Find it in Settings.").Layout)
			}),
		)
	}
	return row.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return textField{editor: &n.editor, hint: "Preset name", height: controlHeightSmall}.Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(secondaryButton(&n.save, "Save preset").withHeight(controlHeightSmall).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
		layout.Rigid(ghostButton(&n.cancel, "Cancel").Widget(theme)),
	)
}
