package screenshot

import (
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var white = color.NRGBA{R: 0xF1, G: 0xF1, B: 0xF1, A: 0xFF}

func TestRender(t *testing.T) {
	longLine := strings.Repeat("word ", 400)
	tests := []struct {
		name          string
		lines         []Line
		wantMinHeight int
		wantMaxWidth  int
	}{
		{"one line", []Line{{"John Doe says: Hi.", white}}, lineHeightPx, maxTextWidthPx},
		{"two lines", []Line{{"a", white}, {"b", white}}, 2 * lineHeightPx, maxTextWidthPx},
		{"long line wraps", []Line{{longLine, white}}, 3 * lineHeightPx, maxTextWidthPx + 2*(paddingPx+outlinePx)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			img, err := Render(test.lines)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			size := img.Bounds().Size()
			if size.Y < test.wantMinHeight {
				t.Errorf("height = %d, want at least %d", size.Y, test.wantMinHeight)
			}
			if size.X > test.wantMaxWidth {
				t.Errorf("width = %d, want at most %d", size.X, test.wantMaxWidth)
			}
		})
	}
}

func TestRenderKeepsBackgroundTransparentAndDrawsText(t *testing.T) {
	img, err := Render([]Line{{"John Doe says: Hello there.", white}})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if corner := img.RGBAAt(0, 0); corner.A != 0 {
		t.Errorf("corner = %v, want transparent", corner)
	}
	var hasText, hasOutline bool
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			pixel := img.RGBAAt(x, y)
			hasText = hasText || (pixel.A == 0xFF && pixel.R > 0xE0)
			hasOutline = hasOutline || (pixel.A == 0xFF && pixel.R == 0)
		}
	}
	if !hasText || !hasOutline {
		t.Errorf("hasText = %v, hasOutline = %v, want both", hasText, hasOutline)
	}
}

func TestRenderWithoutLines(t *testing.T) {
	if _, err := Render(nil); !errors.Is(err, ErrNoLines) {
		t.Errorf("err = %v, want ErrNoLines", err)
	}
}

func TestStoreSaveListLoadDelete(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "screenshots"))
	if shots, err := store.List(); err != nil || len(shots) != 0 {
		t.Fatalf("List before saving = %v, %v; want empty", shots, err)
	}
	lines := []Line{{"* John Doe waves.", white}}
	first, err := store.Save(lines)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	second, err := store.Save(lines)
	if err != nil {
		t.Fatalf("Save again: %v", err)
	}
	if first.Path == second.Path {
		t.Fatalf("both saves wrote %q", first.Path)
	}
	if time.Since(first.Saved) > time.Minute {
		t.Errorf("Saved = %v, want about now", first.Saved)
	}

	shots, err := store.List()
	if err != nil || len(shots) != 2 || shots[0].Path != second.Path {
		t.Fatalf("List = %v, %v; want the 2 shots, newest first", shots, err)
	}
	if !shots[0].Saved.Equal(second.Saved) {
		t.Errorf("listed Saved = %v, want %v", shots[0].Saved, second.Saved)
	}
	if _, err := store.Load(first.Path); err != nil {
		t.Errorf("Load: %v", err)
	}

	if err := store.Delete(first.Path); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if shots, _ := store.List(); len(shots) != 1 {
		t.Errorf("List after delete = %v, want 1 shot", shots)
	}
}

func TestStoreDeleteRefusesOtherFiles(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(filepath.Join(dir, "screenshots"))
	other := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(other, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{other, filepath.Join(dir, "screenshots", "..", "settings.json")} {
		if err := store.Delete(path); !errors.Is(err, ErrNotInStore) {
			t.Errorf("Delete(%q) = %v, want ErrNotInStore", path, err)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("other file gone: %v", err)
	}
}

func TestRenderFallsBackWhenArialIsMissing(t *testing.T) {
	t.Setenv("WINDIR", t.TempDir())
	if _, err := Render([]Line{{"John Doe says: Hi.", white}}); err != nil {
		t.Errorf("Render without Arial: %v", err)
	}
}
