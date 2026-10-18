package ui

import (
	"slices"
	"testing"

	"world-kit/internal/chatlog"
)

func TestLogFiltersFilter(t *testing.T) {
	filters := newLogFilters()
	filters.SetCharacters([]chatlog.Character{{Name: "John Doe", LineCount: 4}, {Name: "Jane Roe", LineCount: 2}})
	if got := filters.Filter(); got.Names != nil || got.Kinds != nil || filters.ActiveCount() != 0 {
		t.Fatalf("new filters = %+v, want the zero filter", got)
	}

	filters.actions[0].isOn = true // /me
	filters.actions[3].isOn = true // IC
	filters.characters[1].isOn = true
	got := filters.Filter()
	wantKinds := []chatlog.Kind{chatlog.KindMe, chatlog.KindSay, chatlog.KindShout, chatlog.KindWhisper,
		chatlog.KindMe, chatlog.KindDo}
	if !slices.Equal(got.Kinds, wantKinds) || !slices.Equal(got.Names, []string{"Jane Roe"}) {
		t.Errorf("got %+v, want kinds %v and names [Jane Roe]", got, wantKinds)
	}
	if filters.ActiveCount() != 3 {
		t.Errorf("ActiveCount = %d, want 3", filters.ActiveCount())
	}
}

func TestLogFiltersKeepCharacterChoices(t *testing.T) {
	filters := newLogFilters()
	filters.SetCharacters([]chatlog.Character{{Name: "John Doe", LineCount: 1}, {Name: "Jane Roe", LineCount: 1}})
	filters.characters[1].isOn = true

	filters.SetCharacters([]chatlog.Character{{Name: "Jane Roe", LineCount: 5}, {Name: "Mark Lee", LineCount: 1}})
	if !filters.characters[0].isOn || filters.characters[1].isOn {
		t.Error("Jane Roe should stay selected and Mark Lee start unselected")
	}
	if filters.characters[0].label != "Jane Roe · 5" {
		t.Errorf("label = %q, want the new line count", filters.characters[0].label)
	}
}

func TestFilterButtonLabel(t *testing.T) {
	if got := filterButtonLabel(0); got != "Filters" {
		t.Errorf("got %q", got)
	}
	if got := filterButtonLabel(3); got != "Filters (3)" {
		t.Errorf("got %q", got)
	}
}

func TestTimestampsAreADisplayOption(t *testing.T) {
	filters := newLogFilters()
	if !filters.ShowsTimestamps() {
		t.Fatal("timestamps should show by default")
	}
	filters.timestamps.isOn = false
	if filters.ActiveCount() != 0 {
		t.Errorf("hiding timestamps counted as %d active filters, want 0", filters.ActiveCount())
	}
}

func TestChatLinesWithoutTimestamps(t *testing.T) {
	records := []chatlog.Record{{Raw: "[09:57:03] * Jane Roe waves."}, {Raw: "(( Jane Roe: brb ))"}}
	want := "* Jane Roe waves.\n(( Jane Roe: brb ))"
	if got := joinChatLines(newChatLines(records, false)); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
