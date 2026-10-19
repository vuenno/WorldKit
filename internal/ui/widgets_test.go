package ui

import (
	"errors"
	"testing"
	"time"

	"gioui.org/op"
	"gioui.org/op/clip"

	"world-kit/internal/capture"
	"world-kit/internal/screenshot"
)

func TestIconPathsParse(t *testing.T) {
	icons := []strokeIcon{iconArrowLeft, iconSearch, iconCopy, iconImage, iconFolder, iconTrash, iconClose,
		iconChevronRight, iconChevronDown, iconPanelLeft, iconFileText, iconSliders, iconUsers, iconCheck,
		iconArrowRight, iconPencil, iconItalic, iconMinus, iconPlus, glyphMinimize, glyphMaximize, glyphRestore, glyphClose,
		iconLayers, iconImages, iconChat, iconUndo, iconRedo, iconExport, iconFit, iconEye, iconEyeOff,
		iconLock, iconUnlock, iconRotate, iconChevronUp, iconAlert}
	for _, icon := range icons {
		var ops op.Ops
		var path clip.Path
		path.Begin(&ops)
		if err := appendIconPath(&path, string(icon), 1); err != nil {
			t.Error(err)
		}
	}
}

func TestCaptureCardTexts(t *testing.T) {
	saved := time.Date(2026, 10, 18, 18, 45, 12, 0, time.UTC)
	tests := []struct {
		name      string
		status    capture.Status
		wantTitle string
		wantColor string
	}{
		{"capturing", capture.Status{State: capture.StateWatching, LastSaved: saved}, "Capturing", "live"},
		{"waiting", capture.Status{State: capture.StateWatching}, "Watching", "live"},
		{"no folder", capture.Status{State: capture.StateNeedsDataDir}, "Not capturing", "danger"},
		{"retrying", capture.Status{State: capture.StateWatching, Err: errTest}, "Retrying…", "warning"},
	}
	colors := map[string]any{"live": colorLive, "danger": colorDanger, "warning": colorWarning}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			title, _, dotColor := captureCardTexts(test.status)
			if title != test.wantTitle || dotColor != colors[test.wantColor] {
				t.Errorf("got %q in %v, want %q in %s", title, dotColor, test.wantTitle, test.wantColor)
			}
		})
	}
}

func TestGalleryColumns(t *testing.T) {
	tests := []struct{ width, want int }{{200, 1}, {576, 2}, {575, 1}, {900, 3}}
	for _, test := range tests {
		if got := galleryColumns(test.width, 280, 16); got != test.want {
			t.Errorf("galleryColumns(%d) = %d, want %d", test.width, got, test.want)
		}
	}
}

func TestGroupShotsByDay(t *testing.T) {
	now := time.Date(2026, 10, 18, 14, 0, 0, 0, time.UTC)
	shot := func(day int) loadedShot {
		return loadedShot{shot: screenshot.Shot{Saved: time.Date(2026, 10, day, 12, 0, 0, 0, time.UTC)}}
	}
	groups := groupShotsByDay([]loadedShot{shot(18), shot(18), shot(17), shot(2)}, now)
	var labels []string
	for _, group := range groups {
		labels = append(labels, group.label)
	}
	if len(groups) != 3 || len(groups[0].shots) != 2 || labels[2] != "October 2026" {
		t.Errorf("groups = %v, want Today (2), Yesterday, October 2026", labels)
	}
}

var errTest = errors.New("busy")
