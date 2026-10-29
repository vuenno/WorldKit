package ui

import (
	"fmt"
	"image"
	"log/slog"
	"time"

	"gioui.org/f32"
	"gioui.org/font"
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
	// rowBoxes are where a backdrop box goes behind each row of text (screenshot.RowBoxes).
	rowBoxes []image.Rectangle
	err      error // why the image couldn't be read; the shot is still listed so it can be deleted
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
		saved, err := store.Load(shot.Path)
		if err != nil {
			slog.Warn("read chat screenshot", "path", shot.Path, "error", err)
			loaded[i].err = err
			continue
		}
		loaded[i].image = paint.NewImageOp(saved.Image)
		loaded[i].size = saved.Image.Bounds().Size()
		loaded[i].rowBoxes = screenshot.RowBoxes(saved.Image, saved.Grid)
	}
	return loaded, nil
}

// shotCardState is the widget state of one card, kept by file path across reloads.
type shotCardState struct {
	open   widget.Clickable
	folder widget.Clickable
	delete widget.Clickable
	hover  hoverArea
}

// ScreenshotsPage shows the saved chat screenshots as a gallery grouped by day, opens one
// in full, and deletes them after asking.
type ScreenshotsPage struct {
	shots       []loadedShot
	cards       map[string]*shotCardState
	hasLoaded   bool
	errorText   string
	loader      *backgroundLoad[[]loadedShot]
	openFolder  widget.Clickable
	list        widget.List
	view        *screenshotView
	pendingOpen string // a shot to open once the next load arrives
	deletion    shotDeletion
	invalidate  func()
}

// NewScreenshotsPage creates the page. invalidate redraws the window once a background
// read finishes.
func NewScreenshotsPage(invalidate func()) *ScreenshotsPage {
	return &ScreenshotsPage{
		cards:      map[string]*shotCardState{},
		loader:     newBackgroundLoad[[]loadedShot](invalidate),
		list:       widget.List{List: layout.List{Axis: layout.Vertical}},
		deletion:   newShotDeletion(invalidate),
		invalidate: invalidate,
	}
}

// Reload re-reads the screenshots in the background and returns to the gallery.
func (p *ScreenshotsPage) Reload(store ScreenshotStore) {
	p.view = nil
	p.loader.Start(func() ([]loadedShot, error) { return loadShots(store) })
}

// Open reloads the gallery and then opens the screenshot at path.
func (p *ScreenshotsPage) Open(store ScreenshotStore, path string) {
	p.Reload(store)
	p.pendingOpen = path
}

// Layout handles input, then draws the open screenshot or the gallery, and the delete
// confirmation on top while it is up.
func (p *ScreenshotsPage) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	p.receiveShots()
	store := services.Screenshots
	if p.deletion.Update(gtx, store) {
		p.Reload(store)
	}
	dims := p.layoutContent(gtx, theme, store)
	p.deletion.Layout(gtx, theme)
	return dims
}

func (p *ScreenshotsPage) layoutContent(gtx layout.Context, theme *material.Theme, store ScreenshotStore) layout.Dimensions {
	if p.view != nil {
		switch p.view.Update(gtx) {
		case viewClosed:
			p.view = nil
		case viewDeleteRequested:
			p.deletion.Ask(p.view.shot.shot.Path)
		}
	}
	if p.view != nil {
		return p.view.Layout(gtx, theme)
	}
	p.handleCardClicks(gtx, store)
	return p.layoutGallery(gtx, theme, store)
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
	for _, shot := range p.shots {
		if p.cards[shot.shot.Path] == nil {
			p.cards[shot.shot.Path] = &shotCardState{}
		}
		if shot.shot.Path == p.pendingOpen {
			p.view = newScreenshotView(shot)
		}
	}
	p.pendingOpen = ""
}

func (p *ScreenshotsPage) handleCardClicks(gtx layout.Context, store ScreenshotStore) {
	if p.openFolder.Clicked(gtx) {
		openFolder(store.Dir())
	}
	for _, shot := range p.shots {
		state := p.cards[shot.shot.Path]
		if state.open.Clicked(gtx) {
			p.view = newScreenshotView(shot)
		}
		if state.folder.Clicked(gtx) {
			revealFile(shot.shot.Path)
		}
		if state.delete.Clicked(gtx) {
			p.deletion.Ask(shot.shot.Path)
		}
	}
}

func (p *ScreenshotsPage) layoutGallery(gtx layout.Context, theme *material.Theme, store ScreenshotStore) layout.Dimensions {
	inset := layout.Inset{Left: spacingPage, Right: spacingPage, Top: spacingSmall}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		items := p.galleryItems(gtx, theme)
		return styledList(theme, &p.list).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
			return items[index](gtx)
		})
	})
}

// galleryItems builds the scrolling page: header, then per day a label and rows of cards.
func (p *ScreenshotsPage) galleryItems(gtx layout.Context, theme *material.Theme) []layout.Widget {
	now := gtx.Now
	columns := galleryColumns(gtx.Constraints.Max.X, gtx.Dp(screenshotCardMinWidth), gtx.Dp(spacingLarge))
	items := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return pageHeader(gtx, theme, "Log Screenshots",
			"Transparent chat images in the in-game style, ready to lay over your screenshots.",
			func(gtx layout.Context) layout.Dimensions {
				return secondaryButton(&p.openFolder, "Open folder").withIcon(iconFolder).Layout(gtx, theme)
			})
	}}
	if p.errorText != "" {
		items = append(items, func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, p.errorText) })
	}
	if len(p.shots) == 0 {
		return append(items, func(gtx layout.Context) layout.Dimensions {
			return layout.Inset{Top: spacingXLarge}.Layout(gtx, secondaryText(theme, emptyShotsText(p.hasLoaded)).Layout)
		})
	}
	for _, group := range groupShotsByDay(p.shots, now) {
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			inset := layout.Inset{Top: spacingXLarge, Bottom: spacingMedium, Left: spacingTiny}
			return inset.Layout(gtx, groupLabel(theme, group.label).Layout)
		})
		for _, row := range chunkShots(group.shots, columns) {
			items = append(items, func(gtx layout.Context) layout.Dimensions {
				return layout.Inset{Bottom: spacingLarge}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return p.layoutCardRow(gtx, theme, row, columns, now)
				})
			})
		}
	}
	return append(items, layout.Spacer{Height: spacingXLarge}.Layout)
}

func emptyShotsText(hasLoaded bool) string {
	if !hasLoaded {
		return "Loading saved screenshots…"
	}
	return "No screenshots yet. Open a log, select lines and choose Extract image."
}

// shotGroup is the screenshots saved on one day (or in one week or month).
type shotGroup struct {
	label string
	shots []loadedShot
}

// groupShotsByDay splits newest-first shots into dayGroupLabel groups, keeping order.
func groupShotsByDay(shots []loadedShot, now time.Time) []shotGroup {
	var groups []shotGroup
	for _, shot := range shots {
		label := dayGroupLabel(shot.shot.Saved, now)
		if len(groups) == 0 || groups[len(groups)-1].label != label {
			groups = append(groups, shotGroup{label: label})
		}
		groups[len(groups)-1].shots = append(groups[len(groups)-1].shots, shot)
	}
	return groups
}

// galleryColumns fits as many cards of at least minCardWidth, gap apart, as width allows.
func galleryColumns(width, minCardWidth, gap int) int {
	return max(1, (width+gap)/(minCardWidth+gap))
}

func chunkShots(shots []loadedShot, size int) [][]loadedShot {
	var rows [][]loadedShot
	for start := 0; start < len(shots); start += size {
		rows = append(rows, shots[start:min(start+size, len(shots))])
	}
	return rows
}

func (p *ScreenshotsPage) layoutCardRow(gtx layout.Context, theme *material.Theme, row []loadedShot, columns int, now time.Time) layout.Dimensions {
	gap := gtx.Dp(spacingLarge)
	cardWidth := (gtx.Constraints.Max.X - gap*(columns-1)) / columns
	children := make([]layout.FlexChild, 0, 2*len(row))
	for i, shot := range row {
		if i > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout))
		}
		card := shotCard{shot: shot, state: p.cards[shot.shot.Path], now: now}
		children = append(children, layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints = layout.Exact(image.Pt(cardWidth, gtx.Constraints.Max.Y))
			gtx.Constraints.Min.Y = 0
			return card.Layout(gtx, theme)
		}))
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx, children...)
}

// shotCard is one screenshot in the gallery: a preview (click to open), when it was saved
// and its size, and while hovered, Show in folder and Delete.
type shotCard struct {
	shot  loadedShot
	state *shotCardState
	now   time.Time
}

func (c shotCard) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return c.state.hover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		border := colorDivider
		if c.state.hover.isHovered {
			border = colorBorderStrong
		}
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return outlinedSurface(gtx, colorSurface, border, radiusCard)
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.layoutPreview(gtx, theme) }),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return c.layoutFooter(gtx, theme) }),
				)
			}),
		)
	})
}

func (c shotCard) layoutPreview(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return c.state.open.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(screenshotPreviewHeight))
		gtx.Constraints = layout.Exact(size)
		// Only the top corners are rounded; the footer sits below.
		radius := float32(gtx.Dp(radiusCard))
		defer clip.RRect{Rect: image.Rectangle{Max: size}, NW: int(radius), NE: int(radius)}.Push(gtx.Ops).Pop()
		pointer.CursorPointer.Add(gtx.Ops)
		drawChecker(gtx, size)
		if c.shot.err != nil {
			layout.Center.Layout(gtx, errorCaptionText(theme, "Couldn't read this screenshot.").Layout)
		} else {
			layout.UniformInset(spacingMedium+spacingTiny/2).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return layoutImageAtPixelSize(gtx, c.shot.image, c.shot.size)
			})
		}
		return layout.Dimensions{Size: size}
	})
}

func (c shotCard) layoutFooter(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	inset := layout.Inset{Top: spacingMedium, Bottom: spacingMedium, Left: spacingMedium + spacingTiny/2, Right: spacingMedium}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = gtx.Dp(controlHeightSmall)
		row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
		return row.Layout(gtx,
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
					layout.Rigid(styledLabel(theme, textSizeBody, font.SemiBold, colorTextPrimary, shotTimeText(c.shot.shot, c.now)).Layout),
					layout.Rigid(layout.Spacer{Height: spacingTiny - spacingTiny/4}.Layout),
					layout.Rigid(caption(theme, shotSizeText(c.shot)).Layout),
				)
			}),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				if !c.state.hover.isHovered {
					return layout.Dimensions{}
				}
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Rigid(iconButton(&c.state.folder, iconFolder).withKind(buttonGhost).withHeight(controlHeightSmall).Widget(theme)),
					layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
					layout.Rigid(iconButton(&c.state.delete, iconTrash).withKind(buttonGhost).withHeight(controlHeightSmall).asDanger().Widget(theme)),
				)
			}),
		)
	})
}

// shotTimeText is the save time within its day group: "14:18 UTC", or with the date for
// groups that span several days.
func shotTimeText(shot screenshot.Shot, now time.Time) string {
	if daysBetween(shot.Saved, now) <= 1 {
		return shot.Saved.UTC().Format(clockLayout) + " UTC"
	}
	return dayAndTimeText(shot.Saved, now)
}

func shotSizeText(shot loadedShot) string {
	if shot.err != nil {
		return "Unreadable file"
	}
	return fmt.Sprintf("%d × %d px", shot.size.X, shot.size.Y)
}

// drawChecker fills size with the transparency checkerboard.
func drawChecker(gtx layout.Context, size image.Point) {
	paint.Fill(gtx.Ops, colorCheckerLight)
	square := gtx.Dp(checkerSquare)
	var path clip.Path
	path.Begin(gtx.Ops)
	for y := 0; y < size.Y; y += square {
		for x := (y / square % 2) * square; x < size.X; x += 2 * square {
			path.MoveTo(f32.Pt(float32(x), float32(y)))
			path.LineTo(f32.Pt(float32(x+square), float32(y)))
			path.LineTo(f32.Pt(float32(x+square), float32(y+square)))
			path.LineTo(f32.Pt(float32(x), float32(y+square)))
			path.Close()
		}
	}
	paint.FillShape(gtx.Ops, colorCheckerDark, clip.Outline{Path: path.End()}.Op())
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

func errorCaptionText(theme *material.Theme, text string) material.LabelStyle {
	label := caption(theme, text)
	label.Color = colorDanger
	return label
}
