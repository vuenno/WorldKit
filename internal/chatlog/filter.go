package chatlog

import (
	"slices"
	"strings"
)

// Filter selects records by name and kind. Its zero value keeps every record.
type Filter struct {
	// Name keeps lines spoken by or mentioning a character, ignoring case. Underscores
	// count as spaces, since names are typed as "John_Doe" in commands but shown as "John Doe".
	Name string
	// Kinds keeps only these kinds of lines; empty keeps all kinds.
	Kinds []Kind
}

// Apply returns the records that match the filter, in their original order.
func (f Filter) Apply(records []Record) []Record {
	name := normalizeName(f.Name)
	var matching []Record
	for _, record := range records {
		if f.hasKind(record.Kind) && mentionsName(record, name) {
			matching = append(matching, record)
		}
	}
	return matching
}

func (f Filter) hasKind(kind Kind) bool {
	return len(f.Kinds) == 0 || slices.Contains(f.Kinds, kind)
}

// mentionsName reports whether name (already normalized) is the speaker or appears in the text.
// An empty name matches every record.
func mentionsName(record Record, name string) bool {
	if name == "" {
		return true
	}
	return strings.Contains(normalizeName(record.Speaker), name) ||
		strings.Contains(normalizeName(record.Text), name)
}

func normalizeName(name string) string {
	return strings.ToLower(strings.TrimSpace(strings.ReplaceAll(name, "_", " ")))
}
