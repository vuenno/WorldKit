package ui

import (
	"errors"
	"fmt"
	"image"
	"log/slog"
	"os/exec"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/capture"
	"world-kit/internal/chatlog"
	"world-kit/internal/fivem"
	"world-kit/internal/screenshot"
	"world-kit/internal/settings"
)

// Services are the domain services the screens use.
type Services struct {
	Capture     CaptureService
	Settings    SettingsStore
	Logs        LogLibrary
	Screenshots ScreenshotStore
	Startup     StartupEntry // nil when the startup entry is unavailable
}

// LogLibrary lists the saved chat sessions and reads their lines.
type LogLibrary interface {
	Sessions() ([]chatlog.Session, error)
	Records(path string) ([]chatlog.Record, error)
}

// ScreenshotStore saves chat lines as images and lists, reads and deletes the saved ones.
type ScreenshotStore interface {
	Save(lines []screenshot.Line) (screenshot.Shot, error)
	List() ([]screenshot.Shot, error)
	Load(path string) (image.Image, error)
	Delete(path string) error
}

// CaptureService reports chat capture progress and accepts a user-picked FiveM data folder.
type CaptureService interface {
	Status() capture.Status
	SetDataDir(path string) (string, error)
}

// SettingsStore reads and saves the user's preferences.
type SettingsStore interface {
	Get() settings.Settings
	Update(change func(*settings.Settings)) error
}

// StartupEntry turns starting WorldKit with Windows on and off.
type StartupEntry interface {
	IsEnabled() (bool, error)
	SetEnabled(isEnabled bool) error
}

// captureStatusText describes the capture status in one sentence for the user.
func captureStatusText(status capture.Status) string {
	switch status.State {
	case capture.StateStarting:
		return "Starting chat capture…"
	case capture.StateNeedsDataDir:
		return "FiveM data folder not found. Chat isn't being saved."
	case capture.StateArchiveInUse:
		return "Another WorldKit is already running and saving chat."
	case capture.StateFailed:
		return fmt.Sprintf("Chat capture stopped: %v", status.Err)
	}
	switch {
	case status.Err != nil:
		return "Couldn't read FiveM data just now. Retrying…"
	case status.LinesSaved == 0:
		return "Watching for GTA World chat."
	default:
		return fmt.Sprintf("Saved %d new lines, latest at %s.", status.LinesSaved, status.LastSaved.Format("15:04:05"))
	}
}

// dataDirForm lets the user type FiveM's data folder and save it.
type dataDirForm struct {
	editor       widget.Editor
	save         widget.Clickable
	errorText    string
	hasPrefilled bool
}

func newDataDirForm() *dataDirForm {
	return &dataDirForm{editor: widget.Editor{SingleLine: true, Submit: true}}
}

func (f *dataDirForm) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	f.prefill(services.Capture.Status().DataDir)
	if f.wasSubmitted(gtx) {
		f.errorText = saveDataDir(services, f.editor.Text())
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return textField(gtx, theme, &f.editor, `C:\Users\you\AppData\Roaming\CitizenFX`)
				}),
				layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return primaryButton(&f.save, "Save").Layout(gtx, theme)
				}),
			)
		}),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			return errorCaption(gtx, theme, f.errorText)
		}),
	)
}

// prefill shows the folder in use once it is known, without overwriting what the user typed.
func (f *dataDirForm) prefill(dataDir string) {
	if f.hasPrefilled || dataDir == "" || f.editor.Len() > 0 {
		return
	}
	f.editor.SetText(dataDir)
	f.hasPrefilled = true
}

func (f *dataDirForm) wasSubmitted(gtx layout.Context) bool {
	isSubmitted := f.save.Clicked(gtx)
	for {
		event, ok := f.editor.Update(gtx)
		if !ok {
			return isSubmitted
		}
		if _, ok := event.(widget.SubmitEvent); ok {
			isSubmitted = true
		}
	}
}

// saveDataDir starts capturing from path and remembers it. It returns an error message, or "".
func saveDataDir(services Services, path string) string {
	dir, err := services.Capture.SetDataDir(path)
	if errors.Is(err, fivem.ErrDataDirNotFound) {
		return `That folder has no FiveM data. Pick the CitizenFX folder, usually %AppData%\CitizenFX.`
	}
	if err != nil {
		slog.Error("set FiveM data folder", "error", err)
		return "Couldn't use that folder."
	}
	if err := services.Settings.Update(func(s *settings.Settings) { s.FiveMDataDir = dir }); err != nil {
		slog.Error("save FiveM data folder", "error", err)
		return "Saving chat from this folder now, but WorldKit couldn't remember it for next time."
	}
	return ""
}

// dataDirPrompt asks for FiveM's data folder when it isn't in the default location.
// Capture can't work without it, so the prompt only closes once a valid folder is saved.
type dataDirPrompt struct {
	dialog dialog
	form   *dataDirForm
}

func (p *dataDirPrompt) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	return p.dialog.Layout(gtx, theme, "Where is FiveM's data?",
		"WorldKit saves your GTA World chat from FiveM's data folder (CitizenFX), but couldn't "+
			"find it in the usual place. Paste the folder's path below.",
		func(gtx layout.Context) layout.Dimensions { return p.form.Layout(gtx, theme, services) })
}

// startupPrompt asks once whether WorldKit should start with Windows.
type startupPrompt struct {
	dialog    dialog
	accept    widget.Clickable
	decline   widget.Clickable
	errorText string
}

// Update handles the prompt's buttons and reports whether the user answered.
func (p *startupPrompt) Update(gtx layout.Context, services Services) bool {
	var isAnswered bool
	if p.accept.Clicked(gtx) {
		if err := services.Startup.SetEnabled(true); err != nil {
			slog.Error("enable start with Windows", "error", err)
			p.errorText = "Couldn't add WorldKit to startup apps."
			return false
		}
		isAnswered = true
	}
	isAnswered = isAnswered || p.decline.Clicked(gtx)
	if !isAnswered {
		return false
	}
	if err := services.Settings.Update(func(s *settings.Settings) { s.HasAnsweredStartupPrompt = true }); err != nil {
		slog.Error("save startup prompt answer", "error", err)
	}
	return true
}

func (p *startupPrompt) Layout(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	return p.dialog.Layout(gtx, theme, "Start WorldKit with Windows?",
		"WorldKit only saves chat while it's running. Starting it with Windows means "+
			"no session is missed. You can change this later in Settings.",
		func(gtx layout.Context) layout.Dimensions {
			return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return layout.Flex{Axis: layout.Horizontal}.Layout(gtx,
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return primaryButton(&p.accept, "Start with Windows").Layout(gtx, theme)
						}),
						layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
						layout.Rigid(func(gtx layout.Context) layout.Dimensions {
							return secondaryButton(&p.decline, "Not now").Layout(gtx, theme)
						}),
					)
				}),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return errorCaption(gtx, theme, p.errorText)
				}),
			)
		})
}

// textField draws a single editor on an input surface.
func textField(gtx layout.Context, theme *material.Theme, editor *widget.Editor, hint string) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(func(gtx layout.Context) layout.Dimensions {
			return outlinedSurface(gtx, colorElevatedSurface)
		}),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			gtx.Constraints.Min.X = gtx.Constraints.Max.X
			inset := layout.Inset{Left: spacingSmall, Right: spacingSmall, Top: inputPaddingY, Bottom: inputPaddingY}
			return inset.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				field := material.Editor(theme, editor, hint)
				field.HintColor = colorTextSecondary
				field.SelectionColor = withOpacity(colorPrimary, selectionOpacity)
				return field.Layout(gtx)
			})
		}),
	)
}

// errorCaption shows a problem under a control, or nothing when text is empty.
func errorCaption(gtx layout.Context, theme *material.Theme, text string) layout.Dimensions {
	if text == "" {
		return layout.Dimensions{}
	}
	return layout.Inset{Top: spacingSmall}.Layout(gtx, func(gtx layout.Context) layout.Dimensions {
		label := caption(theme, text)
		label.Color = colorError
		return label.Layout(gtx)
	})
}

// openFolder shows dir in File Explorer.
func openFolder(dir string) {
	// Explorer returns exit code 1 even on success, so only failing to start it is an error.
	if err := exec.Command("explorer.exe", dir).Start(); err != nil {
		slog.Error("open folder", "path", dir, "error", err)
	}
}
