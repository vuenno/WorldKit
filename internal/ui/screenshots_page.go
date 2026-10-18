package ui

import (
	"fmt"
	"image"
	"log/slog"
	"path/filepath"

	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/screenshot"
)

// loadedShot is a saved screenshot with its image decoded, ready to draw.
type loadedShot struct {
	shot  screenshot.Shot
	image paint.ImageOp
	size  image.Point
	err   error // why the image couldn't be read; the shot is still listed so it can be deleted
}

// loadShots lists the store's screenshots and decodes each one.
func loadShots(store ScreenshotStore) ([]loadedShot, error) {
	shots, err := store.List()
	if err != nil {
		return nil, err
	}
	loaded := make([]loadedShot, len(shots))
	for i, shot := range shots {
		loaded[i] = loadedShot{shot: shot}
		img, err := store.Load(shot.Path)
		if err != nil {
			slog.Warn("read chat screenshot", "path", shot.Path, "error", err)
			loaded[i].err = err
			continue
		}
		loaded[i].image = paint.NewImageOp(img)
		loaded[i].size = img.Bounds().Size()
	}
	return loaded, nil
}

// ScreenshotsPage lists the saved chat screenshots, newest first, and opens one in full.
type ScreenshotsPage struct {
	shots      []loadedShot
	hasLoaded  bool
	errorText  string
	loader     *backgroundLoad[[]loadedShot]
	cardClicks []widget.Clickable
	list       widget.List
	view       *screenshotView
	invalidate func()
}

// NewScreenshotsPage creates the page. invalidate redraws the window once a background
// read finishes.
func NewScreenshotsPage(invalidate func()) *ScreenshotsPage {
	return &ScreenshotsPage{
		loader:     newBackgroundLoad[[]loadedShot](invalidate),
		list:       widget.List{List: layout.List{Axis: layout.Vertical}},
		invalidate: invalidate,
	}
}

// Reload re-reads the screenshots in the background and returns to the list.
func (p *ScreenshotsPage) Reload(store ScreenshotStore) {
	p.view = nil
	p.loader.Start(func() ([]loadedShot, error) { return loadShots(store) })
}

// Layout handles input, then draws the open screenshot or the list.
func (p *ScreenshotsPage) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	p.receiveShots()
	if p.view != nil {
		return p.layoutView(gtx, theme, services.Screenshots)
	}
	p.openClickedShot(gtx, services.Screenshots)
	return p.layoutShotList(gtx, theme)
}

func (p *ScreenshotsPage) layoutView(gtx layout.Context, theme *material.Theme, store ScreenshotStore) layout.Dimensions {
	switch p.view.Update(gtx) {
	case viewClosed:
		p.view = nil
		return p.layoutShotList(gtx, theme)
	case viewDeleted:
		p.Reload(store)
		return p.layoutShotList(gtx, theme)
	}
	return p.view.Layout(gtx, theme)
}

func (p *ScreenshotsPage) receiveShots() {
	result, ok := p.loader.Result()
	if !ok {
		return
	}
	p.hasLoaded = true
	p.errorText = ""
	if result.err != nil {
		slog.Error("list chat screenshots", "error", result.err)
		p.errorText = "Couldn't read the saved screenshots."
		return
	}
	p.shots = result.value
	for len(p.cardClicks) < len(p.shots) {
		p.cardClicks = append(p.cardClicks, widget.Clickable{})
	}
}

func (p *ScreenshotsPage) openClickedShot(gtx layout.Context, store ScreenshotStore) {
	for index, shot := range p.shots {
		if p.cardClicks[index].Clicked(gtx) {
			p.view = newScreenshotView(shot, store, p.invalidate)
			return
		}
	}
}

func (p *ScreenshotsPage) layoutShotList(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(pageTitle(theme, "Log Screenshots").Layout),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(caption(theme, "Select lines in a log and choose Extract to save them as an image.").Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return errorCaption(gtx, theme, p.errorText)
			}),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return p.layoutShots(gtx, theme)
			}),
		)
	})
}

func (p *ScreenshotsPage) layoutShots(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if len(p.shots) == 0 && p.errorText != "" {
		return layout.Dimensions{}
	}
	if len(p.shots) == 0 {
		return caption(theme, emptyShotsText(p.hasLoaded)).Layout(gtx)
	}
	list := material.List(theme, &p.list)
	list.Indicator.Color = colorBorder
	list.Indicator.HoverColor = colorTextSecondary
	return list.Layout(gtx, len(p.shots), func(gtx layout.Context, index int) layout.Dimensions {
		card := shotCard{click: &p.cardClicks[index], shot: p.shots[index]}
		if index == 0 {
			return card.Layout(gtx, theme)
		}
		return layout.Inset{Top: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return card.Layout(gtx, theme)
		})
	})
}

func emptyShotsText(hasLoaded bool) string {
	if !hasLoaded {
		return "Loading saved screenshots…"
	}
	return "No screenshots yet. Select lines in a log and choose Extract to make one."
}

// shotCard is one clickable screenshot in the list: a preview of its top, then when it
// was saved and its size.
type shotCard struct {
	click *widget.Clickable
	shot  loadedShot
}

func (c shotCard) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return c.click.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.X = gtx.Constraints.Max.X
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(c.drawBackground),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.UniformInset(spacingMedium).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.layoutPreview(gtx, theme) }),
						layout.Rigid(layout.Spacer{Height: spacingSmall}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.layoutDetails(gtx, theme) }),
					)
				})
			}),
		)
	})
}

func (c shotCard) drawBackground(gtx layout.Context) layout.Dimensions {
	fill := colorSurface
	if c.click.Hovered() {
		fill = colorElevatedSurface
	}
	dims := outlinedSurface(gtx, fill)
	defer clip.Rect(image.Rectangle{Max: dims.Size}).Push(gtx.Ops).Pop()
	pointer.CursorPointer.Add(gtx.Ops)
	return dims
}

func (c shotCard) layoutPreview(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if c.shot.err != nil {
		return errorCaption(gtx, theme, "Couldn't read this screenshot.")
	}
	gtx.Constraints.Max.Y = gtx.Dp(screenshotPreviewMaxHeight)
	return shotBackdrop(gtx, func(gtx layout.Context) layout.Dimensions {
		return layoutImageAtPixelSize(gtx, c.shot.image, c.shot.size)
	})
}

func (c shotCard) layoutDetails(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Flexed(1, sectionTitle(theme, shotSavedText(c.shot.shot)).Layout),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Rigid(caption(theme, shotSizeText(c.shot)).Layout),
	)
}

// shotBackdrop draws content on the app background color, the dark backdrop chat text is
// meant to sit on, with a little padding.
func shotBackdrop(gtx layout.Context, content layout.Widget) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			size := gtx.Constraints.Min
			shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(cornerRadius))
			paint.FillShape(gtx.Ops, colorAppBackground, shape.Op(gtx.Ops))
			return layout.Dimensions{Size: size}
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(spacingSmall).Layout(gtx, content)
		}),
	)
}

// layoutImageAtPixelSize draws img one image pixel per screen pixel, so chat text stays
// as crisp as in the saved file, cut off where it doesn't fit.
func layoutImageAtPixelSize(gtx layout.Context, img paint.ImageOp, size image.Point) layout.Dimensions {
	shown := image.Pt(min(size.X, gtx.Constraints.Max.X), min(size.Y, gtx.Constraints.Max.Y))
	defer clip.Rect{Max: shown}.Push(gtx.Ops).Pop()
	img.Add(gtx.Ops)
	paint.PaintOp{}.Add(gtx.Ops)
	return layout.Dimensions{Size: shown}
}

func shotSavedText(shot screenshot.Shot) string {
	return "Saved " + shot.Saved.UTC().Format(sessionTimeLayout)
}

func shotSizeText(shot loadedShot) string {
	if shot.err != nil {
		return filepath.Base(shot.shot.Path)
	}
	return fmt.Sprintf("%d × %d px", shot.size.X, shot.size.Y)
}
