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
	spans := wrapSpans(text, width, func(a, b int) float64 { return measure(text[a:b]) })
	lines := make([]string, len(spans))
	for i, sp := range spans {
		lines[i] = text[sp.start:sp.end]
	}
	return lines
}

// span is the bytes start..end of a text.
type span struct{ start, end int }

// wrapSpans is wrapWords by byte offsets, measuring text[a:b] as
// measure(a, b), for text whose width is not one font's (rich text).
func wrapSpans(text string, width float64, measure func(a, b int) float64) []span {
	var lines []span
	for at := 0; ; {
		end := strings.IndexByte(text[at:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += at
		}
		lines = wrapPara(lines, text, at, end, width, measure)
		if end == len(text) {
			return lines
		}
		at = end + 1
	}
}

// wrapPara appends the lines of the paragraph text[at:end].
func wrapPara(lines []span, text string, at, end int, width float64, measure func(a, b int) float64) []span {
	if width <= 0 {
		return append(lines, span{at, end})
	}
	line := span{at, at}
	for w := at; ; {
		we := end
		if sp := strings.IndexByte(text[w:end], ' '); sp >= 0 {
			we = w + sp
		}
		// The line so far, the space, and this word (the word alone when
		// it is the first: then line.start is w).
		if measure(line.start, we) <= width {
			line.end = we
		} else {
			if w > at {
				lines = append(lines, line)
			}
			// The word starts a line; break it while it is too wide.
			for measure(w, we) > width {
				cut := fit(text, w, we, width, measure)
				lines = append(lines, span{w, cut})
				w = cut
			}
			line = span{w, we}
		}
		if we == end {
			return append(lines, line)
		}
		w = we + 1
	}
}

// fit is the end of the longest run of whole runes of text[a:b] from a (at
// least one rune) no wider than width.
func fit(text string, a, b int, width float64, measure func(a, b int) float64) int {
	_, first := utf8.DecodeRuneInString(text[a:b])
	n := a + first
	for n < b {
		_, size := utf8.DecodeRuneInString(text[n:b])
		if measure(a, n+size) > width {
			break
		}
		n += size
	}
	return n
}
