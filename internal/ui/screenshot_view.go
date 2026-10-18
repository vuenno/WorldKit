package ui

import (
	"log/slog"
	"os/exec"
	"path/filepath"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
)

// viewOutcome tells the screenshots page what happened in the open screenshot.
type viewOutcome int

const (
	viewOpen    viewOutcome = iota
	viewClosed              // Back was clicked
	viewDeleted             // the screenshot was deleted
)

// screenshotView shows one screenshot in full, with Show in folder and Delete. Deleting asks
// for confirmation first, since the file is removed for good.
type screenshotView struct {
	shot               loadedShot
	store              ScreenshotStore
	back               widget.Clickable
	showInFolder       widget.Clickable
	delete             widget.Clickable
	isConfirmingDelete bool
	confirmDialog      dialog
	confirmDelete      widget.Clickable
	cancelDelete       widget.Clickable
	deleter            *backgroundLoad[struct{}]
	errorText          string
	list               widget.List
}

func newScreenshotView(shot loadedShot, store ScreenshotStore, invalidate func()) *screenshotView {
	return &screenshotView{
		shot:    shot,
		store:   store,
		deleter: newBackgroundLoad[struct{}](invalidate),
		list:    widget.List{List: layout.List{Axis: layout.Vertical}},
	}
}

// Update handles the view's buttons and reports whether it should close.
func (v *screenshotView) Update(gtx layout.Context) viewOutcome {
	if isDeleted := v.receiveDeleteResult(); isDeleted {
		return viewDeleted
	}
	if v.back.Clicked(gtx) {
		return viewClosed
	}
	if v.showInFolder.Clicked(gtx) {
		revealFile(v.shot.shot.Path)
	}
	if v.delete.Clicked(gtx) {
		v.isConfirmingDelete = true
	}
	if v.cancelDelete.Clicked(gtx) {
		v.isConfirmingDelete = false
	}
	if v.confirmDelete.Clicked(gtx) && !v.deleter.IsLoading() {
		v.errorText = ""
		path := v.shot.shot.Path
		v.deleter.Start(func() (struct{}, error) { return struct{}{}, v.store.Delete(path) })
	}
	return viewOpen
}

func (v *screenshotView) receiveDeleteResult() bool {
	result, ok := v.deleter.Result()
	if !ok {
		return false
	}
	v.isConfirmingDelete = false
	if result.err != nil {
		slog.Error("delete chat screenshot", "path", v.shot.shot.Path, "error", result.err)
		v.errorText = "Couldn't delete this screenshot."
		return false
	}
	return true
}

// Layout draws the header, the image and, while deleting, the confirmation dialog.
func (v *screenshotView) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	dims := layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return v.layoutHeader(gtx, theme) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return errorCaption(gtx, theme, v.errorText)
			}),
			layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
			layout.Flexed(1, func(gtx layout.Context) layout.Dimensions { return v.layoutImage(gtx, theme) }),
		)
	})
	if v.isConfirmingDelete {
		v.layoutConfirmDialog(gtx, theme)
	}
	return dims
}

func (v *screenshotView) layoutHeader(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return row.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return secondaryButton(&v.back, "Back").Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
			details := shotSizeText(v.shot) + " · " + filepath.Base(v.shot.shot.Path)
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(pageTitle(theme, shotSavedText(v.shot.shot)).Layout),
				layout.Rigid(caption(theme, details).Layout),
			)
		}),
		layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return secondaryButton(&v.showInFolder, "Show in folder").Layout(gtx, theme)
		}),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return secondaryButton(&v.delete, "Delete").Layout(gtx, theme)
		}),
	)
}

// layoutImage draws the screenshot at its real pixel size, scrolling when it is taller
// than the page.
func (v *screenshotView) layoutImage(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	if v.shot.err != nil {
		return caption(theme, "Couldn't read this screenshot. You can still delete it.").Layout(gtx)
	}
	list := material.List(theme, &v.list)
	list.Indicator.Color = colorBorder
	list.Indicator.HoverColor = colorTextSecondary
	return list.Layout(gtx, 1, func(gtx layout.Context, _ int) layout.Dimensions {
		return shotBackdrop(gtx, func(gtx layout.Context) layout.Dimensions {
			return layoutImageAtPixelSize(gtx, v.shot.image, v.shot.size)
		})
	})
}

func (v *screenshotView) layoutConfirmDialog(gtx layout.Context, theme *material.Theme) {
	v.confirmDialog.Layout(gtx, theme, "Delete this screenshot?",
		"The image file is removed from your computer for good.",
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return primaryButton(&v.confirmDelete, "Delete").Layout(gtx, theme)
				}),
				layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return secondaryButton(&v.cancelDelete, "Cancel").Layout(gtx, theme)
				}),
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
