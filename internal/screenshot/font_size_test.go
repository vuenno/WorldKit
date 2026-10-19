package screenshot

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestClampFontSize(t *testing.T) {
	tests := []struct{ size, want int }{
		{0, DefaultFontSize},
		{MinFontSize, MinFontSize},
		{18, 18},
		{MaxFontSize, MaxFontSize},
		{3, MinFontSize},
		{-5, MinFontSize},
		{40, MaxFontSize},
	}
	for _, test := range tests {
		if got := ClampFontSize(test.size); got != test.want {
			t.Errorf("ClampFontSize(%d) = %d, want %d", test.size, got, test.want)
		}
	}
}

func TestRenderGrowsWithFontSize(t *testing.T) {
	lines := []Line{sayLine("John Doe says: Hello there."), sayLine("Jane Doe says: Hi.")}
	small := render(t, lines, Style{FontSize: MinFontSize})
	normal := render(t, lines, Style{})
	large := render(t, lines, Style{FontSize: MaxFontSize})
	if small.FontSize != MinFontSize || normal.FontSize != DefaultFontSize || large.FontSize != MaxFontSize {
		t.Errorf("FontSize = %d, %d, %d; want %d, %d, %d", small.FontSize, normal.FontSize, large.FontSize,
			MinFontSize, DefaultFontSize, MaxFontSize)
	}
	smallSize, normalSize, largeSize := small.Image.Bounds().Size(), normal.Image.Bounds().Size(), large.Image.Bounds().Size()
	if smallSize.X >= normalSize.X || normalSize.X >= largeSize.X {
		t.Errorf("widths %d, %d, %d; want growing with the font", smallSize.X, normalSize.X, largeSize.X)
	}
	if smallSize.Y >= normalSize.Y || normalSize.Y >= largeSize.Y {
		t.Errorf("heights %d, %d, %d; want growing with the font", smallSize.Y, normalSize.Y, largeSize.Y)
	}
}

func TestRenderWrapsAtTheSameWordsAtAnySize(t *testing.T) {
	lines := []Line{sayLine(longText(300))}
	for _, size := range []int{MinFontSize, DefaultFontSize, MaxFontSize} {
		picture := render(t, lines, Style{FontSize: size})
		if got, want := len(picture.rows), len(render(t, lines, Style{}).rows); got != want {
			t.Errorf("size %d: %d rows, want %d like the default size", size, got, want)
		}
	}
}

func longText(words int) string {
	text := "John Doe says:"
	for range words {
		text += " word"
	}
	return text
}

func TestRowBoxesAtFontSize(t *testing.T) {
	lines := []Line{sayLine("A long first line of text"), sayLine("short"), sayLine("third")}
	picture := render(t, lines, Style{FontSize: MaxFontSize})
	if boxes := RowBoxes(picture.Image, MaxFontSize); len(boxes) != len(lines) {
		t.Errorf("RowBoxes at the drawn size = %d boxes, want %d", len(boxes), len(lines))
	}
}

func TestStoreKeepsFontSize(t *testing.T) {
	store := NewStore(t.TempDir())
	shot, err := store.Save([]Line{sayLine("John Doe says: Hi.")}, Style{FontSize: 20})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := store.Load(shot.Path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if saved.FontSize != 20 {
		t.Errorf("loaded FontSize = %d, want 20", saved.FontSize)
	}
	if saved.Image.Bounds().Empty() {
		t.Error("loaded image is empty")
	}
}

func TestStoreLoadsOldScreenshotsAtDefaultSize(t *testing.T) {
	dir := t.TempDir()
	picture := render(t, []Line{sayLine("John Doe says: Hi.")}, Style{})
	path := filepath.Join(dir, "2026-10-18_101530.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	// A plain PNG, as saved before font sizes were recorded.
	if err := png.Encode(file, picture.Image); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	saved, err := NewStore(dir).Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if saved.FontSize != DefaultFontSize {
		t.Errorf("FontSize = %d, want %d", saved.FontSize, DefaultFontSize)
	}
}
