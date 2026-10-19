package ui

import (
	"image"
	"math"
	"strconv"
	"strings"

	"gioui.org/io/key"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// numberField edits one number in place: it shows the current value, and typing a new one
// then Enter (or clicking elsewhere) commits it. Anything that isn't a number snaps back.
type numberField struct {
	editor     widget.Editor
	wasFocused bool
	target     int  // what the field was focused for (a layer ID); a commit elsewhere is dropped
	isCompact  bool // panelControlHeight and caption-size text, for the editor's panels
}

func newNumberField() *numberField {
	return &numberField{editor: widget.Editor{SingleLine: true, Submit: true, Filter: "0123456789.-"}}
}

// newCompactNumberField is a numberField sized for the editor's panels.
func newCompactNumberField() *numberField {
	field := newNumberField()
	field.isCompact = true
	return field
}

// Update shows value (rounded to whole numbers) while the field isn't being edited, and
// returns a newly committed number. target names what value belongs to (e.g. the selected
// layer's ID): a number typed for one target is never applied to another.
func (f *numberField) Update(gtx layout.Context, value float64, target int) (float64, bool) {
	isSubmitted := false
	for {
		event, ok := f.editor.Update(gtx)
		if !ok {
			break
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			isSubmitted = true
		}
	}
	isFocused := gtx.Focused(&f.editor)
	if isFocused && !f.wasFocused {
		f.target = target
	}
	isCommitted := (isSubmitted || (f.wasFocused && !isFocused)) && f.target == target
	f.wasFocused = isFocused && !isSubmitted
	if isSubmitted {
		gtx.Execute(key.FocusCmd{}) // Enter finishes the edit
	}
	shown := formatNumber(value)
	typed, err := strconv.ParseFloat(strings.TrimSpace(f.editor.Text()), 64)
	if isCommitted || !isFocused {
		if f.editor.Text() != shown {
			f.editor.SetText(shown)
		}
	}
	if isCommitted && err == nil && formatNumber(typed) != shown {
		return typed, true
	}
	return value, false
}

// Abandon forgets an edit in progress, for a field that isn't shown this frame.
func (f *numberField) Abandon() {
	f.wasFocused = false
}

func formatNumber(value float64) string {
	return strconv.Itoa(int(math.Round(value)))
}

// Layout draws the field: a small prefix ("X", "W"), the number in Geist Mono and a unit.
func (f *numberField) Layout(gtx layout.Context, theme *material.Theme, prefix, unit string) layout.Dimensions {
	box := surfaceBox{fill: colorAppBackground, border: colorBorder, radius: radiusMenu, height: controlHeightSmall, paddingX: spacingSmall}
	textSize, gap := textSizeSmall, spacingTiny+spacingTiny/2 // gap: after the prefix
	if f.isCompact {
		box.height, box.radius, box.paddingX, textSize, gap = panelControlHeight, radiusSmall, spacingTiny, textSizeCaption, spacingTiny
	}
	if gtx.Focused(&f.editor) {
		box.border = colorPrimary
	}
	return box.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		affix := func(text string) layout.FlexChild {
			return layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if text == "" {
					return layout.Dimensions{}
				}
				label := caption(theme, text)
				label.Color = colorTextTertiary
				label.MaxLines = 1
				return label.Layout(gtx)
			})
		}
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			affix(prefix),
			layout.Rigid(layout.Spacer{Width: gap}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				field := material.Editor(theme, &f.editor, "")
				field.TextSize = textSize
				field.Font.Typeface = fontMono
				field.SelectionColor = withOpacity(colorPrimary, selectionOpacity)
				return field.Layout(gtx)
			}),
			affix(unit),
		)
	})
}

// slider picks a value from 0 to 1 by dragging or clicking along a track: the part
// before the knob is orange.
type slider struct {
	value widget.Float
}

// Update reports whether the user moved the slider.
func (s *slider) Update(gtx layout.Context) bool {
	return s.value.Update(gtx)
}

// Dragging reports whether the knob is held, so callers can group one drag as one change.
func (s *slider) Dragging() bool {
	return s.value.Dragging()
}

func (s *slider) Layout(gtx layout.Context) layout.Dimensions {
	knob := gtx.Dp(sliderKnobSize)
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(panelControlHeight))
	track := size.X - knob
	trackHeight := gtx.Dp(sliderTrackHeight)
	trackTop := (size.Y - trackHeight) / 2
	filled := int(float32(track) * s.value.Value)

	whole := image.Rect(knob/2, trackTop, knob/2+track, trackTop+trackHeight)
	paint.FillShape(gtx.Ops, colorPressed, clip.UniformRRect(whole, trackHeight/2).Op(gtx.Ops))
	done := image.Rect(knob/2, trackTop, knob/2+filled, trackTop+trackHeight)
	paint.FillShape(gtx.Ops, colorPrimary, clip.UniformRRect(done, trackHeight/2).Op(gtx.Ops))
	knobTop := (size.Y - knob) / 2
	paint.FillShape(gtx.Ops, colorTextPrimary, clip.Ellipse(image.Rect(filled, knobTop, filled+knob, knobTop+knob)).Op(gtx.Ops))

	defer clip.Rect{Max: size}.Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	gtx.Constraints = layout.Exact(image.Pt(track, size.Y))
	offset := op.Offset(image.Pt(knob/2, 0)).Push(gtx.Ops)
	s.value.Layout(gtx, layout.Horizontal, sliderKnobSize/2)
	offset.Pop()
	return layout.Dimensions{Size: size}
}
