package ui

import (
	"fmt"
	"image"
	"log/slog"
	"slices"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// chatPicker is the dialog that adds chat images (from Log Screenshots) to the canvas:
// click cards to select them, then Add.
type chatPicker struct {
	isOpen    bool
	dialog    dialog
	loader    *backgroundLoad[[]loadedShot]
	shots     []loadedShot
	hasLoaded bool
	selected  []string // paths, in the order they were picked
	clicks    map[string]*widget.Clickable
	add       widget.Clickable
	cancel    widget.Clickable
	list      widget.List
	errText   string
}

func newChatPicker(invalidate func()) *chatPicker {
	return &chatPicker{
		dialog: dialog{width: extractDialogWidth},
		loader: newBackgroundLoad[[]loadedShot](invalidate),
		clicks: map[string]*widget.Clickable{},
		list:   widget.List{List: layout.List{Axis: layout.Vertical}},
	}
}

// Open shows the dialog and reads the saved chat images.
func (p *chatPicker) Open(store ScreenshotStore) {
	p.isOpen, p.selected, p.errText = true, nil, ""
	p.loader.Start(func() ([]loadedShot, error) { return loadShots(store) })
}

// Update handles the dialog and returns the picked images once Add is clicked.
func (p *chatPicker) Update(gtx layout.Context) ([]loadedShot, bool) {
	p.receive()
	if !p.isOpen {
		return nil, false
	}
	for _, shot := range p.shots {
		if p.click(shot.shot.Path).Clicked(gtx) && shot.err == nil {
			p.toggle(shot.shot.Path)
		}
	}
	if p.cancel.Clicked(gtx) {
		p.isOpen = false
	}
	if !p.add.Clicked(gtx) || len(p.selected) == 0 {
		return nil, false
	}
	p.isOpen = false
	picked := make([]loadedShot, 0, len(p.selected))
	for _, path := range p.selected {
		if index := slices.IndexFunc(p.shots, func(s loadedShot) bool { return s.shot.Path == path }); index >= 0 {
			picked = append(picked, p.shots[index])
		}
	}
	return picked, true
}

func (p *chatPicker) receive() {
	result, ok := p.loader.Result()
	if !ok {
		return
	}
	p.hasLoaded = true
	if result.err != nil {
		slog.Error("list chat screenshots", "error", result.err)
		p.errText = "Couldn't read the saved chat images."
		return
	}
	p.shots = result.value
}

func (p *chatPicker) click(path string) *widget.Clickable {
	click := p.clicks[path]
	if click == nil {
		click = &widget.Clickable{}
		p.clicks[path] = click
	}
	return click
}

func (p *chatPicker) toggle(path string) {
	if index := slices.Index(p.selected, path); index >= 0 {
		p.selected = slices.Delete(p.selected, index, index+1)
		return
	}
	p.selected = append(p.selected, path)
}

// Layout draws the dialog while it is open: the cards scroll, the buttons stay in view.
func (p *chatPicker) Layout(gtx layout.Context, theme *material.Theme) {
	if !p.isOpen {
		return
	}
	p.dialog.Layout(gtx, theme, "Add chat images",
		"Pick one or more. They're added at their real size, on top of the other layers.",
		func(gtx layout.Context) layout.Dimensions {
			buttonsHeight := gtx.Dp(spacingXLarge + controlHeight)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					gtx.Constraints.Max.Y = max(0, gtx.Constraints.Max.Y-buttonsHeight)
					return p.layoutBody(gtx, theme)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, p.errText) }),
				layout.Rigid(layout.Spacer{Height: spacingXLarge}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutButtons(gtx, theme) }),
			)
		})
}

func (p *chatPicker) layoutBody(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if len(p.shots) == 0 {
		text := "Loading chat images…"
		if p.hasLoaded {
			text = "No chat images yet. Open a log, select lines and choose Extract image."
		}
		return layout.Inset{Top: spacingMedium, Bottom: spacingMedium}.Layout(gtx, secondaryText(theme, text).Layout)
	}
	columns := galleryColumns(gtx.Constraints.Max.X, gtx.Dp(screenshotCardMinWidth*3/4), gtx.Dp(spacingMedium))
	rows := chunkShots(p.shots, columns)
	return styledList(theme, &p.list).Layout(gtx, len(rows), func(gtx layout.Context, index int) layout.Dimensions {
		return layout.Inset{Bottom: spacingMedium}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return p.layoutRow(gtx, theme, rows[index], columns, gtx.Now)
		})
	})
}

func (p *chatPicker) layoutRow(gtx layout.Context, theme *material.Theme, row []loadedShot, columns int, now time.Time) layout.Dimensions {
	gap := gtx.Dp(spacingMedium)
	width := (gtx.Constraints.Max.X - gap*(columns-1)) / columns
	children := make([]layout.FlexChild, 0, 2*len(row))
	for i, shot := range row {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout))
		}
		card := pickableShot{shot: shot, click: p.click(shot.shot.Path), isSelected: slices.Contains(p.selected, shot.shot.Path), now: now}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(width, gtx.Constraints.Max.Y))
			gtx.Constraints.Min.Y = 0
			return card.Layout(gtx, theme)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

func (p *chatPicker) layoutButtons(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	label := "Add images"
	if count := len(p.selected); count > 0 {
		label = fmt.Sprintf("Add %s", plural(count, "image", "images"))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(primaryButton(&p.add, label).withIcon(iconPlus).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(secondaryButton(&p.cancel, "Cancel").Widget(theme)),
	)
}

// pickableShot is a chat image card that toggles selection when clicked: an orange
// outline and check badge while selected.
type pickableShot struct {
	shot       loadedShot
	click      *widget.Clickable
	isSelected bool
	now        time.Time
}

func (c pickableShot) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return c.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		recording := op.Record(gtx.Ops)
		dims := c.layoutContent(gtx, theme)
		content := recording.Stop()
		gtx.Constraints.Min = dims.Size
		border := colorDivider
		switch {
		case c.isSelected:
			border = colorPrimary
		case c.click.Hovered():
			border = colorBorderStrong
		}
		outlinedSurface(gtx, colorSurface, border, radiusCard)
		content.Add(gtx.Ops)
		if c.isSelected {
			shape := clip.UniformRRect(image.Rectangle{Max: dims.Size}, gtx.Dp(radiusCard))
			paint.FillShape(gtx.Ops, colorPrimary, clip.Stroke{Path: shape.Path(gtx.Ops), Width: float32(gtx.Dp(swatchRingWidth))}.Op())
			c.drawCheckBadge(gtx, dims.Size, gtx.Dp(screenshotPreviewHeight*3/4))
		}
		defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		return dims
	})
}

func (c pickableShot) layoutContent(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Constraints.Max.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(screenshotPreviewHeight*3/4))
			radius := gtx.Dp(radiusCard)
			defer clip.RRect{Rect: image.Rectangle{Max: size}, NW: radius, NE: radius}.Push(gtx.Ops).Pop()
			drawChecker(gtx, size)
			gtx.Constraints = layout.Exact(size)
			if c.shot.err != nil {
				return layout.Center.Layout(gtx, errorCaptionText(theme, "Couldn't read this image.").Layout)
			}
			layout.UniformInset(spacingMedium).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutImageAtPixelSize(gtx, c.shot.image, c.shot.size)
			})
			return layout.Dimensions{Size: size}
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(spacingMedium).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						label := styledLabel(theme, textSizeSmall, font.SemiBold, colorTextPrimary, dayAndTimeText(c.shot.shot.Saved, c.now))
						label.MaxLines = 1
						return label.Layout(gtx)
					}),
					layout.Rigid(caption(theme, shotSizeText(c.shot)).Layout),
				)
			})
		}),
	)
}

// drawCheckBadge draws an orange circle with a check at the right of the card's footer,
// where it never covers the chat text.
func (c pickableShot) drawCheckBadge(gtx layout.Context, card image.Point, previewHeight int) {
	side := gtx.Dp(checkBadgeSize)
	margin := gtx.Dp(spacingMedium)
	top := previewHeight + (card.Y-previewHeight-side)/2
	defer op.Offset(image.Pt(card.X-side-margin, top)).Push(gtx.Ops).Pop()
	paint.FillShape(gtx.Ops, colorPrimary, clip.Ellipse(image.Rect(0, 0, side, side)).Op(gtx.Ops))
	inset := (side - gtx.Dp(iconSizeTiny)) / 2
	defer op.Offset(image.Pt(inset, inset)).Push(gtx.Ops).Pop()
	iconCheck.Layout(gtx, iconSizeTiny, colorOnPrimary)
}
