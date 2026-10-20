package window

import (
	"strings"
	"unicode/utf8"
)

// wrapWords breaks text into lines no wider than width under measure:
// greedily by words at spaces, a word wider than the width broken by runes,
// and "\n" always a break. width <= 0 only splits at "\n". MeasureText and
// the painter both use it, so measured and drawn lines cannot differ.
func wrapWords(text string, width float64, measure func(string) float64) []string {
	paras := strings.Split(text, "\n")
	if width <= 0 {
		return paras
	}
	var lines []string
	for _, para := range paras {
		line := ""
		for i, word := range strings.Split(para, " ") {
			candidate := word
			if i > 0 {
				candidate = line + " " + word
			}
			if measure(candidate) <= width {
				line = candidate
				continue
			}
			if i > 0 {
				lines = append(lines, line)
			}
			// The word starts a line; break it while it is too wide.
			for measure(word) > width {
				cut := fit(word, width, measure)
				lines = append(lines, word[:cut])
				word = word[cut:]
			}
			line = word
		}
		lines = append(lines, line)
	}
	return lines
}

// fit is the byte length of the longest prefix of word (at least one rune)
// no wider than width.
func fit(word string, width float64, measure func(string) float64) int {
	_, first := utf8.DecodeRuneInString(word)
	n := first
	for n < len(word) {
		_, size := utf8.DecodeRuneInString(word[n:])
		if measure(word[:n+size]) > width {
			break
		}
		n += size
	}
	return n
}
