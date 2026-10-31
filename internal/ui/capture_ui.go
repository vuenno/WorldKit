package ui

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/capture"
	"world-kit/internal/chatlog"
	"world-kit/internal/exports"
	"world-kit/internal/filedialog"
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
	Files       FileDialogs
	Exports     ExportList
	// DefaultExportDir is where exports go until the user picks a folder in Settings.
	DefaultExportDir string
	Updates          UpdateService // nil: no update checks
	Version          string        // this build's version; "dev" for a local build

	window uintptr // the window's handle, so dialogs can be modal to it; set by Run
}

// FileDialogs shows the system's file dialogs over owner. Each blocks until the user
// chooses (or cancels: filedialog.ErrCanceled), so call them off the UI goroutine.
type FileDialogs interface {
	PickFolder(owner uintptr, title, startDir string) (string, error)
	OpenFiles(owner uintptr, title, startDir string, types []filedialog.FileType) ([]string, error)
	Save(owner uintptr, options filedialog.SaveOptions) (string, error)
}

// ExportList records the Screenshot Editor's exports and lists, forgets and deletes them.
type ExportList interface {
	Add(entry exports.Entry) error
	List() ([]exports.Item, error)
	Remove(path string) error
	Delete(path string) error
}

// ExportDir is the folder exports start in: the one picked in Settings, or the default.
func (s Services) ExportDir() string {
	if dir := s.Settings.Get().ExportDir; dir != "" {
		return dir
	}
	return s.DefaultExportDir
}

// LogLibrary lists the saved chat sessions and reads their lines.
type LogLibrary interface {
	Sessions() ([]chatlog.Session, error)
	Records(path string) ([]chatlog.Record, error)
}

// ScreenshotStore saves chat lines as images and lists, reads and deletes the saved ones.
type ScreenshotStore interface {
	Save(lines []screenshot.Line, style screenshot.Style) (screenshot.Shot, error)
	List() ([]screenshot.Shot, error)
	Load(path string) (screenshot.SavedImage, error)
	Delete(path string) error
	Dir() string // the folder screenshots are saved in
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

// dataDirForm picks FiveM's data folder with Browse… (or a typed path and Enter) and saves it.
type dataDirForm struct {
	editor       widget.Editor
	browse       widget.Clickable
	browseKind   buttonKind
	picker       *backgroundLoad[string]
	errorText    string
	hasPrefilled bool
}

// newDataDirForm creates the form; browseKind styles Browse… (primary where picking the
// folder is the screen's main action).
func newDataDirForm(invalidate func(), browseKind buttonKind) *dataDirForm {
	return &dataDirForm{
		editor:     widget.Editor{SingleLine: true, Submit: true},
		browseKind: browseKind,
		picker:     newBackgroundLoad[string](invalidate),
	}
}

func (f *dataDirForm) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	f.prefill(services.Capture.Status().DataDir)
	f.receivePickedFolder(services)
	if f.browse.Clicked(gtx) && !f.picker.IsLoading() {
		f.startPicker(services)
	}
	if f.wasSubmitted(gtx) {
		f.errorText = saveDataDir(services, f.editor.Text())
	}
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
			return row.Layout(gtx,
				layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
					return textField{editor: &f.editor, hint: `C:\Users\you\AppData\Roaming\CitizenFX`, isMono: true}.Layout(gtx, theme)
				}),
				layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return secondaryButton(&f.browse, "Browse…").withKind(f.browseKind).Layout(gtx, theme)
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

// startPicker opens the folder dialog in the background, starting at the typed folder.
func (f *dataDirForm) startPicker(services Services) {
	owner, startDir := services.window, f.editor.Text()
	f.picker.Start(func() (string, error) {
		return services.Files.PickFolder(owner, "Select FiveM's data folder (CitizenFX)", startDir)
	})
}

func (f *dataDirForm) receivePickedFolder(services Services) {
	result, ok := f.picker.Result()
	if !ok || errors.Is(result.err, filedialog.ErrCanceled) {
		return
	}
	if result.err != nil {
		slog.Error("pick FiveM data folder", "error", result.err)
		f.errorText = "Couldn't open the folder picker. Type the path and press Enter instead."
		return
	}
	f.editor.SetText(result.value)
	f.errorText = saveDataDir(services, result.value)
}

// wasSubmitted reports whether Enter was pressed in the path field.
func (f *dataDirForm) wasSubmitted(gtx layout.Context) bool {
	isSubmitted := false
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
			"find it in the usual place. Pick the folder with Browse…, or paste its path and press Enter.",
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

// openFolder shows dir in File Explorer.
func openFolder(dir string) {
	// Explorer returns exit code 1 even on success, so only failing to start it is an error.
	if err := exec.Command("explorer.exe", dir).Start(); err != nil {
		slog.Error("open folder", "path", dir, "error", err)
	}
}
