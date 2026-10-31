package ui

import (
	"errors"
	"image"
	"image/color"
	"log/slog"
	"os/exec"
	"path/filepath"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op/clip"
	"gioui.org/op/paint"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/exports"
	"world-kit/internal/screenshot"
)

// viewOutcome tells the screenshots page what happened in the open screenshot.
type viewOutcome int

const (
	viewOpen            viewOutcome = iota
	viewClosed                      // Back was clicked
	viewDeleteRequested             // Delete was clicked; the page asks for confirmation
)

// Backdrops, in segmented-control order: transparent draws nothing behind the text; dark
// and light draw a box behind each row of it, in saved images and in previews.
const (
	backdropTransparent = iota
	backdropDark
	backdropLight
)

// backdropColor is the box color for a backdrop choice; zero (transparent) for none.
func backdropColor(backdrop int) color.NRGBA {
	switch backdrop {
	case backdropDark:
		return colorBackdropDark
	case backdropLight:
		return colorBackdropLight
	default:
		return color.NRGBA{}
	}
}

// screenshotView shows one screenshot in full at its real pixel size, with its details,
// Copy image, Show in folder and Delete.
type screenshotView struct {
	shot         loadedShot
	back         widget.Clickable
	copyImage    widget.Clickable
	copier       *imageCopy
	toast        toast
	showInFolder widget.Clickable
	delete       widget.Clickable
	backdrop     *segmentedControl
	list         widget.List
}

func newScreenshotView(shot loadedShot, invalidate func()) *screenshotView {
	return &screenshotView{
		shot:     shot,
		copier:   newImageCopy(invalidate),
		backdrop: newSegmentedControl("Transparent", "Dark", "Light"),
		list:     widget.List{List: layout.List{Axis: layout.Vertical}},
	}
}

// Update handles the view's buttons.
func (v *screenshotView) Update(gtx layout.Context) viewOutcome {
	v.backdrop.Update(gtx)
	v.copier.ShowResult(gtx, &v.toast)
	if v.copyImage.Clicked(gtx) && v.canCopy() {
		v.startCopy()
	}
	if v.showInFolder.Clicked(gtx) {
		revealFile(v.shot.shot.Path)
	}
	if v.delete.Clicked(gtx) {
		return viewDeleteRequested
	}
	if v.back.Clicked(gtx) {
		return viewClosed
	}
	return viewOpen
}

// canCopy reports whether Copy image can run: the image was read, and no copy is running.
func (v *screenshotView) canCopy() bool {
	return v.shot.pixels != nil && !v.copier.IsCopying()
}

// startCopy copies the shot as the preview shows it, on the chosen background.
func (v *screenshotView) startCopy() {
	pixels, boxes, backdrop := v.shot.pixels, v.shot.rowBoxes, backdropColor(v.backdrop.selected)
	v.copier.Start(func() (image.Image, error) {
		return screenshot.OnBackdrop(pixels, boxes, backdrop), nil
	})
}

// Layout draws the header, then the preview beside the details card, and the copy toast
// in the bottom-right corner.
func (v *screenshotView) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	defer layoutCornerToast(gtx, theme, &v.toast)
	inset := layout.Inset{Left: spacingPage, Right: spacingPage, Top: spacingSmall, Bottom: spacingXLarge}
	return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutHeader(gtx, theme) }),
			layout.Rigid(layout.Spacer{Height: spacingXLarge - spacingTiny}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
				row := layout.Flex{Axis: layout.Horizontal}
				return row.Layout(gtx,
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutPreview(gtx, theme) }),
					layout.Rigid(layout.Spacer{Width: spacingXLarge - spacingTiny}.Layout),
					layout.Rigid(func(gtx layout.Context) layout.Dimensions {
						// Narrow windows give the preview and its controls room first.
						gtx.Constraints.Max.X = min(gtx.Dp(screenshotDetailsWidth), gtx.Constraints.Max.X*2/5)
						return v.layoutDetails(gtx, theme)
					}),
				)
			}),
		)
	})
}

func (v *screenshotView) layoutHeader(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return iconButton(&v.back, iconArrowLeft).Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(screenTitle(theme, savedText(v.shot.shot.Saved, gtx.Now)).Layout),
				layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
				layout.Rigid(secondaryText(theme, shotSizeText(v.shot)).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingLarge}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			copyButton := secondaryButton(&v.copyImage, "Copy image").withIcon(iconCopy)
			return layoutEnabled(gtx, v.canCopy(), copyButton.Widget(theme))
		}),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return primaryButton(&v.showInFolder, "Show in folder").withIcon(iconFolder).Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return iconButton(&v.delete, iconTrash).asDanger().Layout(gtx, theme)
		}),
	)
}

// layoutPreview draws the image centered on the chosen backdrop, scrolling when it is
// taller than the page, with the backdrop switch below.
func (v *screenshotView) layoutPreview(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min = gtx.Constraints.Max
			size := gtx.Constraints.Max
			shape := clip.UniformRRect(image.Rectangle{Max: size}, gtx.Dp(radiusCard))
			defer shape.Push(gtx.Ops).Pop()
			drawChecker(gtx, size)
			if v.shot.err != nil {
				return layout.Center.Layout(gtx, secondaryText(theme, "Couldn't read this screenshot. You can still delete it.").Layout)
			}
			return styledList(theme, &v.list).Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
				gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, max(size.Y, v.shot.size.Y))
				return layout.Center.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
					return layoutShotOnBackdrop(gtx, v.shot, backdropColor(v.backdrop.selected))
				})
			})
		}),
		layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Rigid(secondaryText(theme, "Background").Layout),
				layout.Rigid(layout.Spacer{Width: spacingMedium - spacingTiny/2}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.backdrop.Layout(gtx, theme) }),
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return layout.Dimensions{Size: image.Pt(gtx.Constraints.Min.X, 0)}
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					// One line, cut off when there's no room: wrapped, it grew tall enough to
					// squeeze the preview above.
					label := caption(theme, "Shown at actual size")
					label.MaxLines = 1
					return label.Layout(gtx)
				}),
			)
		}),
	)
}

// layoutShotOnBackdrop draws the shot at 1:1 with a box of backdrop behind each row of
// its text, where Extract would draw them into the file. A zero backdrop draws none.
func layoutShotOnBackdrop(gtx layout.Context, shot loadedShot, backdrop color.NRGBA) layout.Dimensions {
	if backdrop.A > 0 {
		shown := image.Rectangle{Max: image.Pt(min(shot.size.X, gtx.Constraints.Max.X), min(shot.size.Y, gtx.Constraints.Max.Y))}
		for _, box := range shot.rowBoxes {
			paint.FillShape(gtx.Ops, backdrop, clip.Rect(box.Intersect(shown)).Op())
		}
	}
	return layoutImageAtPixelSize(gtx, shot.image, shot.size)
}

func (v *screenshotView) layoutDetails(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	gtx.Constraints.Min = image.Pt(gtx.Constraints.Max.X, 0)
	rows := [][2]string{
		{"Saved", v.shot.shot.Saved.UTC().Format(fullLayout)},
		{"Size", shotSizeText(v.shot)},
		{"File", filepath.Base(v.shot.shot.Path)},
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
		children = append(children,
			layout.Rigid(layout.Spacer{Height: spacingLarge}.Layout),
			layout.Rigid(horizontalDivider),
			layout.Rigid(layout.Spacer{Height: spacingLarge}.Layout),
			layout.Rigid(caption(theme, "Paste it over a screenshot in any editor. The black outline keeps the text readable on bright scenes.").Layout),
		)
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
	})
}

// detailRow is a label/value pair in a details card.
func detailRow(gtx layout.Context, theme *material.Theme, label, value string) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Dp(detailLabelWidth)
			return secondaryText(theme, label).Layout(gtx)
		}),
		layout.Flexed(1, styledLabel(theme, textSizeSmall, font.Normal, colorTextPrimary, value).Layout),
	)
}

func horizontalDivider(gtx layout.Context) layout.Dimensions {
	size := image.Pt(gtx.Constraints.Max.X, gtx.Dp(borderWidth))
	paint.FillShape(gtx.Ops, colorDivider, clip.Rect{Max: size}.Op())
	return layout.Dimensions{Size: size}
}

// shotDeletion asks before deleting a screenshot, then deletes it off the UI goroutine.
// The file is removed for good, so the dialog names that plainly.
type shotDeletion struct {
	path    string // the screenshot being asked about; "" when no dialog is up
	dialog  dialog
	confirm widget.Clickable
	cancel  widget.Clickable
	deleter *backgroundLoad[struct{}]
	errText string
}

func newShotDeletion(invalidate func()) shotDeletion {
	return shotDeletion{deleter: newBackgroundLoad[struct{}](invalidate)}
}

// Ask shows the confirmation for the screenshot at path.
func (d *shotDeletion) Ask(path string) {
	d.path = path
	d.errText = ""
}

// Update handles the dialog and reports whether a screenshot was deleted.
func (d *shotDeletion) Update(gtx layout.Context, files fileDeleter) bool {
	if result, ok := d.deleter.Result(); ok {
		if result.err != nil {
			slog.Error("delete screenshot", "path", d.path, "error", result.err)
			d.errText = deleteErrorText(result.err)
			return false
		}
		d.path = ""
		return true
	}
	if d.cancel.Clicked(gtx) {
		d.path = ""
	}
	if d.confirm.Clicked(gtx) && d.path != "" && !d.deleter.IsLoading() {
		path := d.path
		d.deleter.Start(func() (struct{}, error) { return struct{}{}, files.Delete(path) })
	}
	return false
}

// Layout draws the dialog while a deletion is being asked about.
func (d *shotDeletion) Layout(gtx layout.Context, theme *material.Theme) {
	if d.path == "" {
		return
	}
	d.dialog.Layout(gtx, theme, "Delete this screenshot?",
		"The image file is removed from your computer for good.",
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return dangerButton(&d.confirm, "Delete").withIcon(iconTrash).Layout(gtx, theme)
						}),
						layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return secondaryButton(&d.cancel, "Cancel").Layout(gtx, theme)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, d.errText) }),
			)
		})
}

// revealFile opens File Explorer with path selected.
func revealFile(path string) {
	// Explorer returns exit code 1 even on success, so only failing to start it is an error.
	if err := exec.Command("explorer.exe", "/select,", path).Start(); err != nil {
		slog.Error("show file in folder", "path", path, "error", err)
	}
}

// fileDeleter deletes one of the files it manages: chat screenshots or exports.
type fileDeleter interface {
	Delete(path string) error
}

// deleteErrorText explains a failed delete.
func deleteErrorText(err error) string {
	if errors.Is(err, exports.ErrChanged) {
		return "This file changed since it was exported, so WorldKit won't delete it. Remove it from the list instead."
	}
	return "Couldn't delete this screenshot."
}
