package window

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// runeWidth measures one point per rune.
func runeWidth(s string) float64 { return float64(utf8.RuneCountInString(s)) }

func TestWrapWords(t *testing.T) {
	for _, c := range []struct {
		text  string
		width float64
		want  string
	}{
		{"the quick brown fox", 9, "the quick|brown fox"},
		{"the quick brown fox", 100, "the quick brown fox"},
		{"abcdefghij kl", 4, "abcd|efgh|ij|kl"},   // a word wider than the line breaks
		{"one\n\ntwo three", 5, "one||two|three"}, // newlines and empty lines kept
		{"a  b", 4, "a  b"},                       // runs of spaces stay inside a line that fits
		{"", 5, ""},
		{"word", 0, "word"}, // no width: unwrapped
	} {
		got := strings.Join(wrapWords(c.text, c.width, runeWidth), "|")
		if got != c.want {
			t.Errorf("wrapWords(%q, %v) = %q, want %q", c.text, c.width, got, c.want)
		}
	}
}

func TestWrapWordsNeverExceedsTheWidth(t *testing.T) {
	text := "lorem ipsum dolor sit amet consectetur adipiscing elit sed do eiusmod tempor incididunt"
	for w := 1.0; w < 30; w++ {
		for _, line := range wrapWords(text, w, runeWidth) {
			if runeWidth(line) > w {
				t.Fatalf("width %v: line %q is %v wide", w, line, runeWidth(line))
			}
		}
	}
}
