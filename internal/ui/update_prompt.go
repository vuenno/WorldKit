package ui

import (
	"image"
	"log/slog"
	"os/exec"
	"time"

	"gioui.org/font"
	"gioui.org/layout"
	"gioui.org/op"
	"gioui.org/widget"
	"gioui.org/widget/material"

	"world-kit/internal/update"
)

// UpdateService finds newer WorldKit releases and installs them.
type UpdateService interface {
	Status() update.Status
	CheckNow()
	// Offer returns the release to show now, unless the user skipped it or said Later.
	Offer(now time.Time) (update.Release, bool)
	RemindLater(now time.Time) error
	Skip(version update.Version) error
	// Install downloads and starts the installer; WorldKit must quit when it succeeds.
	// It blocks, so call it off the UI goroutine.
	Install(release update.Release) error
}

// updatePrompt is a small card in the page area's bottom-left corner offering a newer
// WorldKit: Update now, Later, Skip this version and a link to the release notes. It
// waits there without blocking anything.
type updatePrompt struct {
	updateNow  widget.Clickable
	later      widget.Clickable
	skip       widget.Clickable
	notes      widget.Clickable
	installer  *backgroundLoad[struct{}]
	installing update.Release // the release being downloaded while installer runs
	errorText  string
}

func newUpdatePrompt(invalidate func()) *updatePrompt {
	return &updatePrompt{installer: newBackgroundLoad[struct{}](invalidate)}
}

// Update handles the prompt's buttons. It reports whether the installer has started,
// which means WorldKit must close now so it can be replaced.
func (p *updatePrompt) Update(gtx layout.Context, services Services) bool {
	if services.Updates == nil {
		return false
	}
	if result, ok := p.installer.Result(); ok {
		if result.err == nil {
			return true
		}
		slog.Error("install update", "version", p.installing.Version.String(), "error", result.err)
		p.errorText = "Couldn't download the update. Try again later."
	}
	release, ok := p.shownRelease(gtx.Now, services)
	if !ok {
		return false
	}
	if p.notes.Clicked(gtx) {
		openURL(release.NotesURL)
	}
	if p.installer.IsLoading() {
		return false
	}
	if p.updateNow.Clicked(gtx) {
		p.startInstall(services, release)
	}
	if p.later.Clicked(gtx) {
		logUpdateAnswerError(services.Updates.RemindLater(gtx.Now))
	}
	if p.skip.Clicked(gtx) {
		logUpdateAnswerError(services.Updates.Skip(release.Version))
	}
	return false
}

func (p *updatePrompt) startInstall(services Services, release update.Release) {
	p.errorText = ""
	p.installing = release
	p.installer.Start(func() (struct{}, error) {
		return struct{}{}, services.Updates.Install(release)
	})
}

// logUpdateAnswerError logs a Later/Skip answer that couldn't be saved; the prompt hides
// anyway this run once it is saved, and comes back next time if it wasn't.
func logUpdateAnswerError(err error) {
	if err != nil {
		slog.Error("save update answer", "error", err)
	}
}

// shownRelease is the release the prompt shows: the one being installed, or the offer.
func (p *updatePrompt) shownRelease(now time.Time, services Services) (update.Release, bool) {
	if p.installer.IsLoading() {
		return p.installing, true
	}
	return services.Updates.Offer(now)
}

// Layout draws the prompt in the bottom-left corner of the area while an update is on
// offer, and asks for a frame when a Later delay ends.
func (p *updatePrompt) Layout(gtx layout.Context, theme *material.Theme, services Services) layout.Dimensions {
	if services.Updates == nil {
		return layout.Dimensions{}
	}
	release, ok := p.shownRelease(gtx.Now, services)
	if !ok {
		p.wakeWhenOffered(gtx, services)
		return layout.Dimensions{}
	}
	recording := op.Record(gtx.Ops)
	gtx.Constraints.Min = image.Point{}
	gtx.Constraints.Max.X = min(gtx.Constraints.Max.X, gtx.Dp(updatePromptWidth))
	dims := p.layoutCard(gtx, theme, services, release)
	drawn := recording.Stop()
	margin := gtx.Dp(spacingLarge)
	defer op.Offset(image.Pt(margin, gtx.Constraints.Max.Y-dims.Size.Y-margin)).Push(gtx.Ops).Pop()
	drawn.Add(gtx.Ops)
	return dims
}

// wakeWhenOffered redraws when a "Later" ends, so the prompt comes back on time even if
// nothing else changes on screen.
func (p *updatePrompt) wakeWhenOffered(gtx layout.Context, services Services) {
	if !services.Updates.Status().HasNewer {
		return
	}
	if remindAfter := services.Settings.Get().RemindUpdateAfter; remindAfter.After(gtx.Now) {
		gtx.Execute(op.InvalidateCmd{At: remindAfter})
	}
}

func (p *updatePrompt) layoutCard(gtx layout.Context, theme *material.Theme, services Services, release update.Release) layout.Dimensions {
	return layout.Stack{}.Layout(gtx,
		layout.Expanded(floatingSurface),
		layout.Stacked(func(gtx layout.Context) layout.Dimensions {
			return layout.UniformInset(spacingLarge).Layout(gtx, func(gtx layout.Context) layout.Dimensions {
				row := layout.Flex{Axis: layout.Horizontal}
				return row.Layout(gtx,
					layout.Rigid(func(gtx layout.Context) layout.Dimensions { return iconDownload.Layout(gtx, iconSize, colorPrimary) }),
					layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
					layout.Flexed(1, func(gtx layout.Context) layout.Dimensions {
						return p.layoutBody(gtx, theme, services, release)
					}),
				)
			})
		}),
	)
}

func (p *updatePrompt) layoutBody(gtx layout.Context, theme *material.Theme, services Services, release update.Release) layout.Dimensions {
	title := "WorldKit " + release.Version.String() + " is available"
	return layout.Flex{Axis: layout.Vertical}.Layout(gtx,
		layout.Rigid(styledLabel(theme, textSizeBody, font.SemiBold, colorTextPrimary, title).Layout),
		layout.Rigid(layout.Spacer{Height: spacingTiny}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions {
			row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Baseline}
			return row.Layout(gtx,
				layout.Rigid(caption(theme, "You have "+services.Version+". ").Layout),
				layout.Rigid(func(gtx layout.Context) layout.Dimensions {
					return linkButton{click: &p.notes, label: "Release notes"}.Layout(gtx, theme)
				}),
			)
		}),
		layout.Rigid(layout.Spacer{Height: spacingMedium}.Layout),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return p.layoutActions(gtx, theme) }),
		layout.Rigid(func(gtx layout.Context) layout.Dimensions { return errorCaption(gtx, theme, p.errorText) }),
	)
}

func (p *updatePrompt) layoutActions(gtx layout.Context, theme *material.Theme) layout.Dimensions {
	isInstalling := p.installer.IsLoading()
	updateLabel := "Update now"
	if isInstalling {
		updateLabel = "Downloading…"
	}
	row := layout.Flex{Axis: layout.Horizontal, Alignment: layout.Middle}
	return layoutEnabled(gtx, !isInstalling, func(gtx layout.Context) layout.Dimensions {
		return row.Layout(gtx,
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return primaryButton(&p.updateNow, updateLabel).withHeight(controlHeightSmall).Layout(gtx, theme)
			}),
			layout.Rigid(layout.Spacer{Width: spacingSmall}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return secondaryButton(&p.later, "Later").withHeight(controlHeightSmall).Layout(gtx, theme)
			}),
			layout.Rigid(layout.Spacer{Width: spacingMedium}.Layout),
			layout.Rigid(func(gtx layout.Context) layout.Dimensions {
				return linkButton{click: &p.skip, label: "Skip this version"}.Layout(gtx, theme)
			}),
		)
	})
}

// updateStatusText describes Settings' update state in one sentence.
func updateStatusText(status update.Status, isAutomaticOff bool) string {
	switch status.State {
	case update.StateDevBuild:
		return "Development build: updates are off."
	case update.StateChecking:
		return "Checking for updates…"
	case update.StateAvailable:
		return "Version " + status.Latest.Version.String() + " is available."
	case update.StateUpToDate:
		return "You have the latest version."
	case update.StateFailed:
		return "Couldn't reach GitHub. Try again later."
	}
	if isAutomaticOff {
		return "Automatic checks are off."
	}
	return "WorldKit checks once a day."
}

// openURL shows a web page (a release on GitHub) in the default browser.
func openURL(url string) {
	if err := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", url).Start(); err != nil {
		slog.Error("open web page", "url", url, "error", err)
	}
}
