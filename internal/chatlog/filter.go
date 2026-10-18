package chatlog

import (
	"cmp"
	"slices"
	"strings"
)

// Filter selects records by character and kind. Its zero value keeps every record.
type Filter struct {
	// Names keeps lines spoken by or mentioning any of these characters, ignoring case.
	// Underscores count as spaces, since names are typed as "John_Doe" in commands but shown
	// as "John Doe". Empty keeps every character.
	Names []string
	// Kinds keeps only these kinds of lines; empty keeps all kinds.
	Kinds []Kind
}

// Apply returns the records that match the filter, in their original order.
func (f Filter) Apply(records []Record) []Record {
	names := make([]string, 0, len(f.Names))
	for _, name := range f.Names {
		if name := normalizeName(name); name != "" {
			names = append(names, name)
		}
	}
	var matching []Record
	for _, record := range records {
		if f.hasKind(record.Kind) && mentionsAnyName(record, names) {
			matching = append(matching, record)
		}
	}
	return matching
}

func (f Filter) hasKind(kind Kind) bool {
	return len(f.Kinds) == 0 || slices.Contains(f.Kinds, kind)
}

// mentionsAnyName reports whether one of names (already normalized) is the speaker or appears
// in the text. No names matches every record.
func mentionsAnyName(record Record, names []string) bool {
	if len(names) == 0 {
		return true
	}
	speaker := normalizeName(record.Speaker)
	text := normalizeName(record.Text)
	for _, name := range names {
		if strings.Contains(speaker, name) || strings.Contains(text, name) {
			return true
		}
	}
	return false
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(name, "_", " ")))
}

// Character is someone who speaks in a session.
type Character struct {
	Name      string // as first written in the session
	LineCount int    // lines they speak (says, emotes, OOC, PMs…)
}

// Characters returns everyone who speaks in records, most active first; ties are alphabetical.
// Names differing only in case or "_" vs " " count as one character.
func Characters(records []Record) []Character {
	indexByName := map[string]int{}
	var characters []Character
	for _, record := range records {
		key := normalizeName(record.Speaker)
		if key == "" {
			continue
		}
		index, ok := indexByName[key]
		if !ok {
			index = len(characters)
			indexByName[key] = index
			characters = append(characters, Character{Name: record.Speaker})
		}
		characters[index].LineCount++
	}
	slices.SortStableFunc(characters, func(a, b Character) int {
		if byCount := cmp.Compare(b.LineCount, a.LineCount); byCount != 0 {
			return byCount
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return characters
}
