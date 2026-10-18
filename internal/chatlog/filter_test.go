package chatlog

import (
	"slices"
	"strings"
	"testing"
)

const filterSession = `[DATE: 18/OCT/2026 | TIME: 09:57:01]
[09:57:02] John Doe says: hi Jane
[09:57:03] * Jane Roe waves.
[09:57:04] * The door is locked. (( John Doe ))
[09:57:05] (( Jane Roe: brb ))
[09:57:06] * John Doe looks at Jane Roe.
[09:57:07] Mark Lee shouts: hey!
[09:57:08] john_doe whispers: psst`

func parseFilterSession(t *testing.T) []Record {
	t.Helper()
	records, err := Parse(strings.NewReader(filterSession))
	if err != nil {
		t.Fatal(err)
	}
	return records
}

func TestFilterApply(t *testing.T) {
	records := parseFilterSession(t)
	tests := []struct {
		name      string
		filter    Filter
		wantLines []int
	}{
		{"zero filter keeps everything", Filter{}, []int{1, 2, 3, 4, 5, 6, 7, 8}},
		{"name matches speaker and mentions", Filter{Names: []string{"jane roe"}}, []int{3, 5, 6}},
		{"underscores match spaces", Filter{Names: []string{" Jane_Roe "}}, []int{3, 5, 6}},
		{"partial name", Filter{Names: []string{"jane"}}, []int{2, 3, 5, 6}},
		{"several names match any", Filter{Names: []string{"Mark Lee", "Jane Roe"}}, []int{3, 5, 6, 7}},
		{"blank names are ignored", Filter{Names: []string{" "}}, []int{1, 2, 3, 4, 5, 6, 7, 8}},
		{"one kind", Filter{Kinds: []Kind{KindMe}}, []int{3, 6}},
		{"several kinds", Filter{Kinds: []Kind{KindDo, KindOOC}}, []int{4, 5}},
		{"name and kind", Filter{Names: []string{"John Doe"}, Kinds: []Kind{KindMe, KindDo}}, []int{4, 6}},
		{"no match", Filter{Names: []string{"nobody"}}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var gotLines []int
			for _, record := range test.filter.Apply(records) {
				gotLines = append(gotLines, record.Line)
			}
			if !slices.Equal(gotLines, test.wantLines) {
				t.Errorf("got lines %v, want %v", gotLines, test.wantLines)
			}
		})
	}
}

func TestCharacters(t *testing.T) {
	got := Characters(parseFilterSession(t))
	want := []Character{
		{Name: "John Doe", LineCount: 4},
		{Name: "Jane Roe", LineCount: 2},
		{Name: "Mark Lee", LineCount: 1},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
