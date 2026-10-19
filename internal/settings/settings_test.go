package settings

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "WorldKit", "settings.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Get(); !reflect.DeepEqual(got, Settings{}) {
		t.Fatalf("defaults = %+v, want zero settings", got)
	}

	preset := ExportPreset{Name: "Forum", Format: "jpeg", Quality: 90, Width: 1920, Height: 1080}
	want := Settings{
		FiveMDataDir: `D:\CitizenFX\kvs`, HasAnsweredStartupPrompt: true,
		ExportDir: `D:\Shots`, ExportPresets: []ExportPreset{preset}, DefaultPreset: "Forum",
	}
	err = store.Update(func(s *Settings) {
		s.FiveMDataDir = want.FiveMDataDir
		s.HasAnsweredStartupPrompt = true
		s.ExportDir = want.ExportDir
		s.SavePreset(preset)
		s.DefaultPreset = "Forum"
	})
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Get(); !reflect.DeepEqual(got, want) {
		t.Errorf("reopened settings = %+v, want %+v", got, want)
	}
}

func TestPresets(t *testing.T) {
	var s Settings
	s.SavePreset(ExportPreset{Name: "A", Format: "png", Width: 1})
	s.SavePreset(ExportPreset{Name: "B", Format: "png", Width: 2})
	s.SavePreset(ExportPreset{Name: "A", Format: "jpeg", Width: 3})
	if len(s.ExportPresets) != 2 {
		t.Fatalf("presets = %+v, want A replaced in place", s.ExportPresets)
	}
	if got, _ := s.Preset("A"); got.Width != 3 {
		t.Errorf("Preset(A) = %+v, want the replacement", got)
	}
	s.DefaultPreset = "B"
	s.DeletePreset("B")
	if _, ok := s.Preset("B"); ok || s.DefaultPreset != "" {
		t.Errorf("after delete: %+v", s)
	}
}

func TestGetReturnsACopy(t *testing.T) {
	store, err := Open(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(s *Settings) { s.SavePreset(ExportPreset{Name: "A"}) }); err != nil {
		t.Fatal(err)
	}
	got := store.Get()
	got.ExportPresets[0].Name = "changed"
	if name := store.Get().ExportPresets[0].Name; name != "A" {
		t.Errorf("stored preset changed through a copy: %q", name)
	}
}
