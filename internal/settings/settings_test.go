package settings

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "WorldKit", "settings.json")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Get(); got != (Settings{}) {
		t.Fatalf("defaults = %+v, want zero settings", got)
	}

	want := Settings{FiveMDataDir: `D:\CitizenFX\kvs`, HasAnsweredStartupPrompt: true}
	err = store.Update(func(s *Settings) {
		s.FiveMDataDir = want.FiveMDataDir
		s.HasAnsweredStartupPrompt = true
	})
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.Get(); got != want {
		t.Errorf("reopened settings = %+v, want %+v", got, want)
	}
}
