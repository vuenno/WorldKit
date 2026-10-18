package screenshot

import (
	"image/color"
	"slices"
	"testing"
)

var (
	say   = color.NRGBA{R: 0xF1, G: 0xF1, B: 0xF1, A: 0xFF}
	emote = color.NRGBA{R: 0xC2, G: 0xA3, B: 0xDA, A: 0xFF}
	red   = color.NRGBA{R: 0xF0, A: 0xFF}
	black = color.NRGBA{A: 0xFF}
)

// sayLine is a plain line in the say color.
func sayLine(text string) Line {
	return NewLine(text, say, emote)
}

func TestParseMarkup(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []Span
	}{
		{"plain", "John Doe says: hi.", []Span{{"John Doe says: hi.", say, false}}},
		{"italic word", "I /really/ mean it", []Span{{"I ", say, false}, {"really", say, true}, {" mean it", say, false}}},
		{"italic one letter", "/x/", []Span{{"x", say, true}}},
		{"italic phrase", "/two words/.", []Span{{"two words", say, true}, {".", say, false}}},
		{"slash inside words stays", "and/or 1/2 w/o", []Span{{"and/or 1/2 w/o", say, false}}},
		{"unclosed slash stays", "use /me to act", []Span{{"use /me to act", say, false}}},
		{"slash with spaces stays", "a / b / c", []Span{{"a / b / c", say, false}}},
		{
			"emote inside speech drops asterisks",
			"John Doe says: Fine. *gives up on his friend and runs*",
			[]Span{{"John Doe says: Fine. ", say, false}, {"gives up on his friend and runs", emote, false}},
		},
		{"emote one word", "Hello *waves*", []Span{{"Hello ", say, false}, {"waves", emote, false}}},
		{"me line asterisk stays", "* John Doe waves.", []Span{{"* John Doe waves.", say, false}}},
		{"radio prefix stays", "** [S: 1 | CH: 2] hi", []Span{{"** [S: 1 | CH: 2] hi", say, false}}},
		{"multiplication stays", "2 * 3 * 4", []Span{{"2 * 3 * 4", say, false}}},
		{
			"italic inside emote",
			"*sighs /loudly/*",
			[]Span{{"sighs ", emote, false}, {"loudly", emote, true}},
		},
		{"empty", "", nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := ParseMarkup(test.text, say, emote)
			if !slices.Equal(got, test.want) {
				t.Errorf("ParseMarkup(%q) =\n %v\nwant\n %v", test.text, got, test.want)
			}
		})
	}
}
