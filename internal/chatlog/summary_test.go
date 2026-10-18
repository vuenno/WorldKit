package chatlog

import (
	"strings"
	"testing"
	"time"
)

func TestSummarize(t *testing.T) {
	text := `[DATE: 18/OCT/2026 | TIME: 09:57:01]
[09:57:02] John Doe says: Hey.
[09:57:03] * John Doe waves at Jane Roe.
[09:57:04] * The door is locked. (( Jane Roe ))
[09:57:05] (( John Doe: brb ))
[10:12:06] (( PM from (169) Jane Roe: hi ))`
	records, err := Parse(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	got := Summarize(records)
	if got.Lines != 5 || got.Says != 1 || got.Emotes != 2 || got.OOC != 1 || got.PMs != 1 {
		t.Errorf("counts = %+v, want 5 lines: 1 say, 2 emotes, 1 OOC, 1 PM", got)
	}
	if len(got.Characters) != 2 || got.Characters[0].Name != "John Doe" {
		t.Errorf("characters = %+v, want John Doe first of 2", got.Characters)
	}
	if want := 15*time.Minute + 5*time.Second; got.Duration() != want {
		t.Errorf("duration = %v, want %v", got.Duration(), want)
	}
}

func TestSummarizeEmpty(t *testing.T) {
	got := Summarize(nil)
	if got.Lines != 0 || got.Duration() != 0 || len(got.Characters) != 0 {
		t.Errorf("got %+v, want an empty summary", got)
	}
}
