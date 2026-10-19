package ui

import (
	"fmt"
	"image"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"gioui.org/font"
	"gioui.org/io/pointer"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/compose"
	"world-kit/internal/exports"
)

// maxCachedThumbnails bounds the Saved Screenshots thumbnails kept in memory (each is at
// most thumbnailMaxSide pixels a side, ~64 KB).
const maxCachedThumbnails = 128

// exportRowState is one row's widget state, kept by file path across reloads.
type exportRowState struct {
	open   widget.Clickable
	folder widget.Clickable
	remove widget.Clickable // Delete, or Remove from list for a missing file
	hover  hoverArea
}

// ExportsPage is Saved Screenshots: everything the editor exported, newest first, grouped
// by day, with the same actions as Log Screenshots (open, show in folder, delete).
type ExportsPage struct {
	items      []exports.Item
	hasLoaded  bool
	errorText  string
	loader     *backgroundLoad[[]exports.Item]
	remover    *backgroundLoad[struct{}]
	thumbs     *imageLoader[string]
	thumbnails map[string]loadedImage // by thumbnailKey
	requested  map[string]bool
	rows       map[string]*exportRowState
	list       widget.List
	openFolder widget.Clickable
	openEditor widget.Clickable
	view       *exportView
	deletion   shotDeletion
	invalidate func()
}

// NewExportsPage creates the page. invalidate redraws the window when a read finishes.
func NewExportsPage(invalidate func()) *ExportsPage {
	return &ExportsPage{
		loader:     newBackgroundLoad[[]exports.Item](invalidate),
		remover:    newBackgroundLoad[struct{}](invalidate),
		thumbs:     newImageLoader[string](invalidate),
		thumbnails: map[string]loadedImage{},
		requested:  map[string]bool{},
		rows:       map[string]*exportRowState{},
		list:       widget.List{List: layout.List{Axis: layout.Vertical}},
		deletion:   newShotDeletion(invalidate),
		invalidate: invalidate,
	}
}

// Reload re-reads the export list in the background and returns to the list.
func (p *ExportsPage) Reload(list ExportList) {
	p.view = nil
	p.loader.Start(list.List)
}

// OpenEditorClicked reports whether the empty page's Open Screenshot Editor was clicked.
func (p *ExportsPage) OpenEditorClicked(gtx layout.Context) bool {
	return p.openEditor.Clicked(gtx)
}

// thumbnailKey identifies a thumbnail by file and version, so a re-export to the same path
// shows the new picture.
func thumbnailKey(item exports.Item) string {
	return item.Path + "|" + item.Modified.UTC().Format(time.RFC3339Nano)
}

// Layout handles input, then draws the open export or the list, and the delete dialog.
func (p *ExportsPage) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	p.receive()
	if p.deletion.Update(gtx, services.Exports) {
		p.Reload(services.Exports)
	}
	if _, ok := p.remover.Result(); ok {
		p.Reload(services.Exports)
	}
	dims := p.layoutContent(gtx, theme, services)
	p.deletion.Layout(gtx, theme)
	return dims
}

func (p *ExportsPage) receive() {
	for _, result := range p.thumbs.Receive() {
		p.thumbnails[result.key] = result.image
	}
	result, ok := p.loader.Result()
	if !ok {
		return
	}
	p.hasLoaded, p.errorText = true, ""
	if result.err != nil {
		slog.Error("list exported screenshots", "error", result.err)
		p.errorText = "Couldn't read the list of exported screenshots."
		return
	}
	p.items = result.value
	for _, item := range p.items {
		if p.rows[item.Path] == nil {
			p.rows[item.Path] = &exportRowState{}
		}
	}
	if len(p.thumbnails) > maxCachedThumbnails {
		clear(p.thumbnails)
		clear(p.requested)
	}
}

// thumbnail returns the item's thumbnail, asking for it the first time it is drawn, so
// only rows that are actually shown are decoded.
func (p *ExportsPage) thumbnail(item exports.Item) (loadedImage, bool) {
	key := thumbnailKey(item)
	if loaded, ok := p.thumbnails[key]; ok {
		return loaded, true
	}
	if !p.requested[key] && !item.IsMissing {
		p.requested[key] = true
		p.thumbs.Load(key, item.Path, thumbnailMaxSide)
	}
	return loadedImage{}, false
}

func (p *ExportsPage) layoutContent(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	if p.view != nil {
		switch p.view.Update(gtx) {
		case viewClosed:
			p.view = nil
		case viewDeleteRequested:
			p.deletion.Ask(p.view.item.Path)
		}
	}
	if p.view != nil {
		return p.view.Layout(gtx, theme)
	}
	p.handleRowClicks(gtx, services)
	return p.layoutList(gtx, theme, services)
}

func (p *ExportsPage) handleRowClicks(gtx layout.Context, services Services) {
	if p.openFolder.Clicked(gtx) {
		openFolder(services.ExportDir())
	}
	for _, item := range p.items {
		state := p.rows[item.Path]
		if state.open.Clicked(gtx) && !item.IsMissing {
			p.view = newExportView(item, p.invalidate)
		}
		if state.folder.Clicked(gtx) {
			revealFile(item.Path)
		}
		if state.remove.Clicked(gtx) {
			p.removeOrAsk(services, item)
		}
	}
}

// removeOrAsk forgets a missing file right away; an existing one is deleted after asking.
func (p *ExportsPage) removeOrAsk(services Services, item exports.Item) {
	if !item.IsMissing && !item.IsChanged {
		p.deletion.Ask(item.Path)
		return
	}
	path := item.Path
	p.remover.Start(func() (struct{}, error) { return struct{}{}, services.Exports.Remove(path) })
}

func (p *ExportsPage) layoutList(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	inset := layout.Inset{Left: spacingPage, Right: spacingPage, Top: spacingSmall}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		items := p.listItems(gtx, theme)
		return styledList(theme, &p.list).Layout(gtx, len(items), func(gtx layout.Context, index int) layout.Dimensions {
			gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(contentMaxWidth))
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			return items[index](gtx)
		})
	})
}

func (p *ExportsPage) listItems(gtx layout.Context, theme *material.Theme) []layout.Widget {
	now := gtx.Now
	items := []layout.Widget{func(gtx layout.Context) layout.Dimensions {
		return pageHeader(gtx, theme, "Saved Screenshots", "Everything you exported from the Screenshot Editor.",
			secondaryButton(&p.openFolder, "Open folder").withIcon(iconFolder).Widget(theme))
	}}
	if p.errorText != "" {
		items = append(items, func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, p.errorText) })
	}
	if len(p.items) == 0 {
		return append(items, func(gtx layout.Context) layout.Dimensions { return p.layoutEmpty(gtx, theme) })
	}
	var lastLabel string
	for _, item := range p.items {
		if label := dayGroupLabel(item.Saved, now); label != lastLabel {
			lastLabel = label
			items = append(items, func(gtx layout.Context) layout.Dimensions {
				inset := layout.Inset{Top: spacingXLarge, Bottom: spacingMedium, Left: spacingTiny}
				return inset.Layout(gtx, groupLabel(theme, label).Layout)
			})
		}
		items = append(items, func(gtx layout.Context) layout.Dimensions {
			thumb, _ := p.thumbnail(item)
			row := exportRow{item: item, state: p.rows[item.Path], thumb: thumb, now: now}
			return layout.Inset{Bottom: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				return row.Layout(gtx, theme)
			})
		})
	}
	return append(items, layout.Spacer{Height: spacingXLarge}.Layout)
}

func (p *ExportsPage) layoutEmpty(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if !p.hasLoaded {
		return layout.Inset{Top: spacingXLarge}.Layout(gtx, secondaryText(theme, "Loading…").Layout)
	}
	return layout.Inset{Top: spacingXLarge}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return iconImages.Layout(gtx, iconSize*2, colorTextTertiary)
			}),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Rigid(bodyText(theme, "Nothing exported yet.").Layout),
			layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
			layout.Rigid(secondaryText(theme, "Lay chat images over a game screenshot in the editor, then Export.").Layout),
			layout.Rigid(layout.Spacer{Height: spacingLarge}.Layout),
			layout.Rigid(secondaryButton(&p.openEditor, "Open Screenshot Editor").withIcon(iconLayers).Widget(theme)),
		)
	})
}

// exportRow is one export in the list: thumbnail, file name over its folder, size over
// format, file size over time, and while hovered, Show in folder and Delete.
type exportRow struct {
	item  exports.Item
	state *exportRowState
	thumb loadedImage
	now   time.Time
}

func (r exportRow) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return r.state.hover.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Stack{}.Layout(gtx,
			layout.Expanded(func(gtx layout.Context) layout.Dimensions {
				return r.state.open.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					fill := colorSurface
					if r.state.hover.isHovered {
						fill = colorElevatedSurface
					}
					dims := outlinedSurface(gtx, fill, colorDivider, radiusControl+spacingTiny/2)
					defer clip.Rect{Max: dims.Size}.Push(gtx.Ops).Pop()
					if !r.item.IsMissing {
						pointer.CursorPointer.Add(gtx.Ops)
					}
					return dims
				})
			}),
			layout.Stacked(func(gtx layout.Context) layout.Dimensions {
				gtx.Constraints.Min.X = gtx.Constraints.Max.X
				inset := layout.Inset{Top: spacingMedium, Bottom: spacingMedium, Left: spacingMedium, Right: spacingLarge}
				return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions { return r.layoutColumns(gtx, theme) })
			}),
		)
	})
}

func (r exportRow) layoutColumns(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.layoutThumb(gtx) }),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return r.layoutName(gtx, theme) }),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutStatColumn(gtx, theme, fmt.Sprintf("%d × %d", r.item.Width, r.item.Height), exportFormat(r.item.Entry).Describe())
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return layoutStatColumn(gtx, theme, fileSizeText(r.item.Bytes), r.item.Saved.UTC().Format(clockLayout)+" UTC")
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return r.layoutActions(gtx, theme) }),
	)
}

func (r exportRow) layoutThumb(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Dp(exportThumbWidth), gtx.Dp(exportThumbHeight))
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusMenu)).Push(gtx.Ops).Pop()
	drawChecker(gtx, size)
	switch {
	case r.item.IsMissing || r.thumb.err != nil:
		gtx.Constraints = layout.Exact(size)
		layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
			return iconAlert.Layout(gtx, iconSize, colorTextTertiary)
		})
	case len(r.thumb.versions) > 0:
		drawFittedImage(gtx, r.thumb.versions[0], size)
	}
	return layout.Dimensions{Size: size}
}

func (r exportRow) layoutName(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := styledLabel(theme, textSizeBody, font.SemiBold, colorTextPrimary, filepath.Base(r.item.Path))
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: spacingTiny - spacingTiny/4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			text, textColor := filepath.Dir(r.item.Path), colorTextSecondary
			switch {
			case r.item.IsMissing:
				text, textColor = "File moved or deleted", colorWarning
			case r.item.IsChanged:
				text, textColor = "Changed since it was exported", colorWarning
			}
			label := monoText(theme, textSizeCaption, text)
			label.Color = textColor
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
	)
}

// layoutStatColumn is a value over a caption in a fixed-width column.
func layoutStatColumn(gtx layout.Context, theme *material.Theme, value, detail string) layout.Dimensions {
	gtx.Constraints.Min.X = gtx.Dp(sessionStatsColumnWidth * 3 / 4)
	gtx.Constraints.Max.X = gtx.Constraints.Min.X
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := monoText(theme, textSizeSmall, value)
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
		layout.Rigid(layout.Spacer{Height: spacingTiny - spacingTiny/4}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			label := caption(theme, detail)
			label.MaxLines = 1
			return label.Layout(gtx)
		}),
	)
}

func (r exportRow) layoutActions(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	actionWidth := gtx.Dp(controlHeightSmall*2 + spacingTiny + spacingSmall)
	if r.item.IsMissing || r.item.IsChanged {
		return layout.Inset{Left: spacingSmall}.Layout(gtx, ghostButton(&r.state.remove, "Remove from list").Widget(theme))
	}
	gtx.Constraints.Min.X = actionWidth
	if !r.state.hover.isHovered {
		return layout.Dimensions{Size: image.Pt(actionWidth, 0)}
	}
	return layout.E.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		gtx.Constraints.Min.Y = 0
		return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
			layout.Rigid(iconButton(&r.state.folder, iconFolder).withKind(buttonGhost).withHeight(controlHeightSmall).Widget(theme)),
			layout.Rigid(layout.Spacer{Width: spacingTiny}.Layout),
			layout.Rigid(iconButton(&r.state.remove, iconTrash).withKind(buttonGhost).withHeight(controlHeightSmall).asDanger().Widget(theme)),
		)
	})
}

// exportFormat is an entry's format as the editor describes it.
func exportFormat(entry exports.Entry) compose.Format {
	return compose.Format{Type: compose.FileType(entry.Format), Quality: entry.Quality, IsLossless: entry.IsLossless}.Normalized()
}

// fileSizeText is a file size for people: "640 KB", "2.4 MB".
func fileSizeText(bytes int64) string {
	switch {
	case bytes >= 1<<20:
		return strings.Replace(fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20)), ".0 ", " ", 1)
	case bytes >= 1<<10:
		return fmt.Sprintf("%d KB", (bytes+512)>>10)
	}
	return fmt.Sprintf("%d B", bytes)
}

// exportView shows one export fitted to the page, with its details, Show in folder and
// Delete.
type exportView struct {
	item         exports.Item
	preview      *backgroundLoad[imageVersion]
	image        imageVersion
	errText      string
	back         widget.Clickable
	showInFolder widget.Clickable
	delete       widget.Clickable
}

func newExportView(item exports.Item, invalidate func()) *exportView {
	view := &exportView{item: item, preview: newBackgroundLoad[imageVersion](invalidate)}
	path := item.Path
	view.preview.Start(func() (imageVersion, error) {
		loaded := decodeVersions(path, []int{previewMaxSide})
		if loaded.err != nil {
			return imageVersion{}, loaded.err
		}
		return loaded.versions[0], nil
	})
	return view
}

// Update handles the view's buttons.
func (v *exportView) Update(gtx layout.Context) viewOutcome {
	if result, ok := v.preview.Result(); ok {
		if result.err != nil {
			slog.Warn("open exported screenshot", "path", v.item.Path, "error", result.err)
			v.errText = "Couldn't read this image. You can still delete it."
		}
		v.image = result.value
	}
	if v.showInFolder.Clicked(gtx) {
		revealFile(v.item.Path)
	}
	if v.delete.Clicked(gtx) {
		return viewDeleteRequested
	}
	if v.back.Clicked(gtx) {
		return viewClosed
	}
	return viewOpen
}

// Layout draws the header, then the fitted preview beside the details card.
func (v *exportView) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	inset := layout.Inset{Left: spacingPage, Right: spacingPage, Top: spacingSmall, Bottom: spacingXLarge}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutHeader(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingXLarge - spacingTiny}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutPreview(gtx, theme) }),
					layout.Rigid(layout.Spacer{Width: spacingXLarge - spacingTiny}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						gtx.Constraints.Max.X = min(gtx.Dp(screenshotDetailsWidth), gtx.Constraints.Max.X*2/5)
						return v.layoutDetails(gtx, theme)
					}),
				)
			}),
		)
	})
}

func (v *exportView) layoutHeader(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(iconButton(&v.back, iconArrowLeft).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					title := screenTitle(theme, filepath.Base(v.item.Path))
					title.MaxLines = 1
					return title.Layout(gtx)
				}),
				layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
				layout.Rigid(secondaryText(theme, savedText(v.item.Saved, gtx.Now)).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(primaryButton(&v.showInFolder, "Show in folder").withIcon(iconFolder).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(iconButton(&v.delete, iconTrash).asDanger().Widget(theme)),
	)
}

// layoutPreview draws the image scaled to fit on a rounded checkerboard.
func (v *exportView) layoutPreview(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	size := gtx.Constraints.Max
	defer clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusCard)).Push(gtx.Ops).Pop()
	paint.Fill(gtx.Ops, colorSidebar)
	gtx.Constraints = layout.Exact(size)
	switch {
	case v.errText != "":
		return layout.Center.Layout(gtx, secondaryText(theme, v.errText).Layout)
	case v.image.size == (image.Point{}):
		return layout.Center.Layout(gtx, caption(theme, "Loading…").Layout)
	}
	padding := gtx.Dp(spacingLarge)
	box := size.Sub(image.Pt(2*padding, 2*padding))
	shown := compose.FitInside(v.image.size, compose.Vec{X: float64(box.X), Y: float64(box.Y)})
	shownSize := image.Pt(int(shown.X), int(shown.Y))
	layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		drawChecker(gtx, shownSize)
		drawFittedImage(gtx, v.image, shownSize)
		return layout.Dimensions{Size: shownSize}
	})
	return layout.Dimensions{Size: size}
}

func (v *exportView) layoutDetails(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
	rows := [][2]string{
		{"Saved", v.item.Saved.UTC().Format(fullLayout)},
		{"Size", fmt.Sprintf("%d × %d px", v.item.Width, v.item.Height)},
		{"Format", exportFormat(v.item.Entry).Describe()},
		{"File", fileSizeText(v.item.Bytes)},
		{"Folder", filepath.Dir(v.item.Path)},
	}
	return card(gtx, spacingXLarge-spacingTiny, func(gtx layout.Context) layout.Dimensions {
		children := []layout.FlexChild{
			layout.Rigid(styledLabel(theme, textSizeBody, font.SemiBold, colorTextPrimary, "Details").Layout),
		}
		for _, row := range rows {
			children = append(children,
				layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return detailRow(gtx, theme, row[0], row[1]) }),
			)
		}
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}
