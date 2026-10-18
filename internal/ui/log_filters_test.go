package ui

import (
	"slices"
	"strings"
	"testing"

	"world-kit/internal/chatlog"
)

func TestLogFiltersFilter(t *testing.T) {
	filters := newLogFilters()
	filters.SetCharacters([]chatlog.Character{{Name: "John Doe", LineCount: 4}, {Name: "Jane Roe", LineCount: 2}})
	if got := filters.Filter(); got.Names != nil || got.Kinds != nil || filters.IsActive() {
		t.Fatalf("new filters = %+v, want the zero filter", got)
	}

	filters.kinds.selected = 1 // IC
	filters.characters[1].isOn = true
	got := filters.Filter()
	if !slices.Equal(got.Kinds, inCharacterKinds) || !slices.Equal(got.Names, []string{"Jane Roe"}) {
		t.Errorf("got %+v, want IC kinds and names [Jane Roe]", got)
	}
	if !filters.IsActive() {
		t.Error("IsActive = false with a kind and a character chosen")
	}

	filters.Clear()
	if filters.IsActive() || filters.Filter().Kinds != nil {
		t.Errorf("after Clear: active = %v, filter = %+v; want nothing on", filters.IsActive(), filters.Filter())
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
	if filters.characters[0].lineCount != 5 {
		t.Errorf("line count = %d, want the new count", filters.characters[0].lineCount)
	}
}

func TestLogFiltersFindText(t *testing.T) {
	filters := newLogFilters()
	filters.find.SetText("  COPS ")
	if !filters.Matches(chatlog.Record{Raw: "[09:57:05] Jane Roe whispers: the cops are around."}) {
		t.Error("find should ignore case and surrounding spaces")
	}
	if filters.Matches(chatlog.Record{Raw: "John Doe says: hi"}) {
		t.Error("a line without the text matched")
	}
	if !filters.IsActive() {
		t.Error("find text should count as an active filter")
	}
}

func TestTimestampsAreADisplayOption(t *testing.T) {
	filters := newLogFilters()
	if filters.ShowsTimestamps() {
		t.Fatal("timestamps should be hidden by default")
	}
	filters.timestamps.value.Value = true
	if filters.IsActive() {
		t.Error("showing timestamps counted as an active filter")
	}
}

func TestChatLinesWithoutTimestamps(t *testing.T) {
	records := []chatlog.Record{{Raw: "[09:57:03] * Jane Roe waves."}, {Raw: "(( Jane Roe: brb ))"}}
	want := "* Jane Roe waves.\n(( Jane Roe: brb ))"
	lines := newChatLines(records, false)
	if got := joinChatLines(lines); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if lines[0].time != "" {
		t.Errorf("time = %q, want it hidden", lines[0].time)
	}
}

func TestChatLinesWithTimestamps(t *testing.T) {
	records, err := chatlog.Parse(strings.NewReader("[09:57:03] * Jane Roe waves.\n(( Jane Roe: brb ))"))
	if err != nil {
		t.Fatal(err)
	}
	lines := newChatLines(records, true)
	if lines[0].time != "09:57:03" || lines[0].text != "* Jane Roe waves." {
		t.Errorf("line = %+v, want the timestamp split into its column", lines[0])
	}
	if lines[1].time != "" {
		t.Errorf("a line without a timestamp got time %q", lines[1].time)
	}
	if got, want := joinChatLines(lines), "[09:57:03] * Jane Roe waves.\n(( Jane Roe: brb ))"; got != want {
		t.Errorf("copied %q, want the stored lines %q", got, want)
	}
}
