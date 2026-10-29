package ui

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log/slog"
	"path/filepath"
	"time"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/compose"
	"world-kit/internal/exports"
	"world-kit/internal/filedialog"
	"world-kit/internal/settings"
)

// exportNameLayout names exported files by their UTC export time, like chat screenshots.
const exportNameLayout = "worldkit-2006-01-02_150405"

// Export dialog choices, in segmented-control order.
const (
	fileTypePNG = iota
	fileTypeJPEG
)

const (
	pngLossless = iota
	pngSmaller
)

// customPresetLabel names the preset menu's state when the choices match no preset.
const customPresetLabel = "Custom"

// exportResult is a finished export, or why it didn't happen.
type exportResult struct {
	path       string
	isCanceled bool
}

// exportDialog picks the file format and quality (or a saved preset), saves the choices as
// a preset, and exports: Save As, then render and encode in the background.
type exportDialog struct {
	isOpen     bool
	dialog     dialog
	presetMenu dropdownMenu
	presetName string // the preset the choices came from; "" when changed since
	fileType   *segmentedControl
	pngMode    *segmentedControl
	quality    *numberStepper
	naming     *presetNaming
	export     widget.Clickable
	cancel     widget.Clickable
	exporter   *backgroundLoad[exportResult]
	errText    string
}

func newExportDialog(invalidate func()) *exportDialog {
	return &exportDialog{
		fileType: newSegmentedControl("PNG", "JPEG"),
		pngMode:  newSegmentedControl("Lossless", "Smaller file"),
		quality:  &numberStepper{value: compose.DefaultJPEGQuality, min: compose.MinQuality, max: compose.MaxQuality, increment: 5, unit: "%"},
		naming:   newPresetNaming(),
		exporter: newBackgroundLoad[exportResult](invalidate),
	}
}

// Open shows the dialog. A preset picked with the canvas size sets the choices; otherwise
// they stay as last used, or start from the default preset the first time.
func (d *exportDialog) Open(store SettingsStore, preset string, isFirstOpen bool) {
	d.isOpen, d.errText = true, ""
	d.naming.Reset()
	current := store.Get()
	if preset == "" && isFirstOpen {
		preset = current.DefaultPreset
	}
	if found, ok := current.Preset(preset); ok {
		d.applyFormat(presetFormat(found))
		d.presetName = found.Name
	}
}

// IsExporting reports whether an export is still being written.
func (d *exportDialog) IsExporting() bool {
	return d.exporter.IsLoading()
}

func (d *exportDialog) format() compose.Format {
	if d.fileType.selected == fileTypeJPEG {
		return compose.Format{Type: compose.FileTypeJPEG, Quality: d.quality.value}
	}
	return compose.Format{Type: compose.FileTypePNG, Quality: d.quality.value, IsLossless: d.pngMode.selected == pngLossless}
}

func (d *exportDialog) applyFormat(format compose.Format) {
	d.fileType.selected = fileTypePNG
	if format.Type == compose.FileTypeJPEG {
		d.fileType.selected = fileTypeJPEG
	}
	d.pngMode.selected = pngSmaller
	if format.IsLossless {
		d.pngMode.selected = pngLossless
	}
	d.quality.value = format.Quality
}

// Update handles the dialog and returns a successful export. Errors stay in the dialog.
func (d *exportDialog) Update(gtx layout.Context, services Services, comp compose.Composition) (exportResult, bool) {
	if result, ok := d.exporter.Result(); ok {
		return d.finish(result)
	}
	if !d.isOpen {
		return exportResult{}, false
	}
	presets := services.Settings.Get().ExportPresets
	if index, ok := d.presetMenu.Update(gtx, len(presets)); ok {
		d.applyFormat(presetFormat(presets[index]))
		d.presetName = presets[index].Name
	}
	isChanged := d.fileType.Update(gtx)
	isChanged = d.pngMode.Update(gtx) || isChanged
	isChanged = d.quality.Update(gtx) || isChanged
	if isChanged {
		d.presetName = ""
	}
	d.updateNaming(gtx, services, comp.Size)
	if d.cancel.Clicked(gtx) && !d.IsExporting() {
		d.isOpen = false
	}
	if d.export.Clicked(gtx) && !d.IsExporting() {
		d.startExport(services, comp)
	}
	return exportResult{}, false
}

// finish closes the dialog after a successful export and keeps it open on an error.
func (d *exportDialog) finish(result loadResult[exportResult]) (exportResult, bool) {
	if result.err != nil {
		slog.Error("export screenshot", "error", result.err)
		d.errText = exportErrorText(result.err)
		return exportResult{}, false
	}
	if result.value.isCanceled {
		return result.value, false
	}
	d.isOpen = false
	return result.value, true
}

func exportErrorText(err error) string {
	var layerErr *compose.LayerError
	switch {
	case errors.Is(err, compose.ErrTooLarge):
		return "One of the images is too large to export."
	case errors.As(err, &layerErr):
		return fmt.Sprintf("Can't read the image of “%s”. Hide or delete that layer, then export again.", layerErr.Name)
	}
	return "Couldn't write the file. Check the folder and that there is free space."
}

func (d *exportDialog) updateNaming(gtx layout.Context, services Services, size image.Point) {
	name, ok := d.naming.Update(gtx, defaultPresetName(d.format(), size))
	if !ok {
		return
	}
	preset := newPreset(name, d.format(), size)
	if err := services.Settings.Update(func(s *settings.Settings) { s.SavePreset(preset) }); err != nil {
		slog.Error("save export preset", "error", err)
		d.errText = "Couldn't save the preset."
		return
	}
	d.presetName = name
	d.naming.Saved(name)
}

func newPreset(name string, format compose.Format, size image.Point) settings.ExportPreset {
	return settings.ExportPreset{
		Name: name, Format: string(format.Type), Quality: format.Quality, IsLossless: format.IsLossless,
		Width: size.X, Height: size.Y,
	}
}

// defaultPresetName suggests "JPEG 92% · 1920×1080".
func defaultPresetName(format compose.Format, size image.Point) string {
	name := "PNG"
	switch {
	case format.Type == compose.FileTypeJPEG:
		name = fmt.Sprintf("JPEG %d%%", format.Quality)
	case !format.IsLossless:
		name = fmt.Sprintf("PNG %d%%", format.Quality)
	}
	return fmt.Sprintf("%s · %d×%d", name, size.X, size.Y)
}

// startExport asks where to save, then renders, encodes and records the export, all off
// the UI goroutine.
func (d *exportDialog) startExport(services Services, comp compose.Composition) {
	d.errText = ""
	format := d.format()
	owner := services.window
	folder := services.Settings.Get().LastExportDir
	if folder == "" {
		folder = services.ExportDir()
	}
	comp = comp.Clone() // the canvas may change while this runs
	d.exporter.Start(func() (exportResult, error) {
		return runExport(services, owner, comp, format, folder)
	})
}

func runExport(services Services, owner uintptr, comp compose.Composition, format compose.Format, folder string) (exportResult, error) {
	path, err := services.Files.Save(owner, saveOptions(format, folder))
	if errors.Is(err, filedialog.ErrCanceled) {
		return exportResult{isCanceled: true}, nil
	}
	if err != nil {
		return exportResult{}, err
	}
	info, err := compose.Export(context.Background(), comp, format, path)
	if err != nil {
		return exportResult{}, err
	}
	entry := exports.Entry{
		Path: path, Saved: time.Now().UTC(), Format: string(format.Type), Quality: format.Quality,
		IsLossless: format.IsLossless, Width: comp.Size.X, Height: comp.Size.Y,
		Bytes: info.Size(), Modified: info.ModTime(),
	}
	if err := services.Exports.Add(entry); err != nil {
		slog.Error("record export", "path", path, "error", err) // the file is saved; only the list missed it
	}
	if err := services.Settings.Update(func(s *settings.Settings) { s.LastExportDir = filepath.Dir(path) }); err != nil {
		slog.Warn("remember export folder", "error", err)
	}
	return exportResult{path: path}, nil
}

// saveOptions offers only the chosen format, so the file always matches the quality
// settings picked in the dialog.
func saveOptions(format compose.Format, folder string) filedialog.SaveOptions {
	name := time.Now().UTC().Format(exportNameLayout) + format.Extension()
	if format.Type == compose.FileTypeJPEG {
		return filedialog.SaveOptions{
			Title: "Export image", Folder: folder, FileName: name,
			Type: filedialog.FileType{Name: "JPEG image", Pattern: "*.jpg;*.jpeg"}, Extensions: []string{"jpg", "jpeg"},
		}
	}
	return filedialog.SaveOptions{
		Title: "Export image", Folder: folder, FileName: name,
		Type: filedialog.FileType{Name: "PNG image", Pattern: "*.png"}, Extensions: []string{"png"},
	}
}

// Layout draws the dialog while it is open.
func (d *exportDialog) Layout(gtx layout.Context, theme *material.Theme, services Services, comp compose.Composition) {
	if !d.isOpen {
		return
	}
	message := fmt.Sprintf("%d × %d px · %s", comp.Size.X, comp.Size.Y, plural(len(comp.Layers), "layer", "layers"))
	d.dialog.width = extractDialogWidth * 3 / 4
	d.dialog.Layout(gtx, theme, "Export image", message, func(gtx layout.Context) layout.Dimensions {
		return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.layoutOptions(gtx, theme, services, comp) }),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, d.errText) }),
			layout.Rigid(layout.Spacer{Height: spacingXLarge}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions { return d.layoutButtons(gtx, theme) }),
		)
	})
}

func (d *exportDialog) layoutOptions(gtx layout.Context, theme *material.Theme, services Services, comp compose.Composition) layout.Dimensions {
	presets := services.Settings.Get().ExportPresets
	children := []layout.FlexChild{}
	row := func(widget layout.Widget) {
		if len(children) > 0 {
			children = append(children, layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout))
		}
		children = append(children, layout.Rigid(widget))
	}
	if len(presets) > 0 {
		row(func(gtx layout.Context) layout.Dimensions {
			return layoutOptionRow(gtx, theme, "Preset", func(gtx layout.Context, theme *material.Theme) layout.Dimensions {
				return d.presetMenu.LayoutButton(gtx, theme, d.presetLabel(), false, presetMenuItems(presets))
			}, "Format and quality saved earlier.")
		})
	}
	row(func(gtx layout.Context) layout.Dimensions {
		return layoutOptionRow(gtx, theme, "Format", d.fileType.Layout, d.formatHint())
	})
	if d.fileType.selected == fileTypePNG {
		row(func(gtx layout.Context) layout.Dimensions {
			return layoutOptionRow(gtx, theme, "Quality", d.pngMode.Layout, d.pngHint())
		})
	}
	if d.fileType.selected == fileTypeJPEG || d.pngMode.selected == pngSmaller {
		row(func(gtx layout.Context) layout.Dimensions {
			return layoutOptionRow(gtx, theme, d.qualityLabel(), d.quality.Layout, d.qualityHint())
		})
	}
	if d.fileType.selected == fileTypeJPEG && comp.Background == compose.BackgroundTransparent {
		row(func(gtx layout.Context) layout.Dimensions {
			label := caption(theme, "JPEG has no transparency: empty parts of the canvas become black.")
			label.Color = colorWarning
			return label.Layout(gtx)
		})
	}
	row(func(gtx layout.Context) layout.Dimensions { return d.naming.Layout(gtx, theme) })
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx, children...)
}

func presetMenuItems(presets []settings.ExportPreset) []menuItem {
	items := make([]menuItem, len(presets))
	for i, preset := range presets {
		items[i] = menuItem{label: preset.Name, detail: presetFormat(preset).Describe()}
	}
	return items
}

func (d *exportDialog) presetLabel() string {
	if d.presetName == "" {
		return customPresetLabel
	}
	return d.presetName
}

func (d *exportDialog) formatHint() string {
	if d.fileType.selected == fileTypeJPEG {
		return "Smaller files; colored chat text may blur a little."
	}
	return "Keeps transparency and sharp chat text."
}

func (d *exportDialog) pngHint() string {
	if d.pngMode.selected == pngLossless {
		return "Every pixel exactly as shown."
	}
	return "Fewer colors: a much smaller file."
}

func (d *exportDialog) qualityLabel() string {
	if d.fileType.selected == fileTypeJPEG {
		return "Quality"
	}
	return "Colors"
}

func (d *exportDialog) qualityHint() string {
	if d.fileType.selected == fileTypeJPEG {
		return "Always lossy; 100% loses the least."
	}
	return "Lower keeps fewer colors."
}

func (d *exportDialog) layoutButtons(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	label := "Export…"
	if d.IsExporting() {
		label = "Exporting…"
	}
	return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
		layout.Rigid(primaryButton(&d.export, label).withIcon(iconExport).Widget(theme)),
		layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
		layout.Rigid(secondaryButton(&d.cancel, "Cancel").Widget(theme)),
	)
}
