package chatlog

import (
	"slices"
	"strings"
	"testing"
)

func TestFilterApply(t *testing.T) {
	session := strings.Join([]string{
		"[DATE: 18/OCT/2026 | TIME: 09:57:01]",            // 1
		"[09:57:02] John Doe says: hi Jane",               // 2
		"[09:57:03] * Jane Roe waves.",                    // 3
		"[09:57:04] * The door is locked. (( John Doe ))", // 4
		"[09:57:05] (( Jane Roe: brb ))",                  // 5
		"[09:57:06] * John Doe looks at Jane Roe.",        // 6
	}, "\n")
	records, err := Parse(strings.NewReader(session))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		filter    Filter
		wantLines []int
	}{
		{"zero filter keeps everything", Filter{}, []int{1, 2, 3, 4, 5, 6}},
		{"name matches speaker and mentions", Filter{Name: "jane roe"}, []int{3, 5, 6}},
		{"underscores match spaces", Filter{Name: " Jane_Roe "}, []int{3, 5, 6}},
		{"partial name", Filter{Name: "jane"}, []int{2, 3, 5, 6}},
		{"one kind", Filter{Kinds: []Kind{KindMe}}, []int{3, 6}},
		{"several kinds", Filter{Kinds: []Kind{KindDo, KindOOC}}, []int{4, 5}},
		{"name and kind", Filter{Name: "John Doe", Kinds: []Kind{KindMe, KindDo}}, []int{4, 6}},
		{"no match", Filter{Name: "nobody"}, nil},
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
