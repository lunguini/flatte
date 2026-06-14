package flatui

import (
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/rivo/uniseg"
)

// TextField is a single-line editable string. Cursor is a byte offset into
// Value; the movement and edit methods keep it on a grapheme-cluster boundary
// so multi-rune clusters (combining marks, ZWJ emoji, regional-indicator flags)
// are treated as indivisible units and never split. clampCursor additionally
// guards against a manually-set offset landing inside a UTF-8 rune.
type TextField struct {
	Value  string
	Cursor int
}

func (f *TextField) Insert(r rune) {
	f.clampCursor()
	text := string(r)
	f.Value = f.Value[:f.Cursor] + text + f.Value[f.Cursor:]
	f.Cursor += len(text)
}

func (f *TextField) Backspace() {
	f.clampCursor()
	if f.Cursor == 0 {
		return
	}
	start := prevGraphemeBoundary(f.Value, f.Cursor)
	f.Value = f.Value[:start] + f.Value[f.Cursor:]
	f.Cursor = start
}

func (f *TextField) Delete() {
	f.clampCursor()
	if f.Cursor >= len(f.Value) {
		return
	}
	end := nextGraphemeBoundary(f.Value, f.Cursor)
	f.Value = f.Value[:f.Cursor] + f.Value[end:]
}

func (f *TextField) MoveLeft() {
	f.clampCursor()
	f.Cursor = prevGraphemeBoundary(f.Value, f.Cursor)
}

func (f *TextField) MoveRight() {
	f.clampCursor()
	f.Cursor = nextGraphemeBoundary(f.Value, f.Cursor)
}

func (f *TextField) MoveWordLeft() {
	f.clampCursor()
	f.Cursor = prevWordBoundary(f.Value, f.Cursor)
}

func (f *TextField) MoveWordRight() {
	f.clampCursor()
	f.Cursor = nextWordBoundary(f.Value, f.Cursor)
}

func (f *TextField) SetCursor(cursor int) {
	f.Cursor = cursor
	f.clampCursor()
}

// CursorColumn returns the cursor offset in display cells within the
// rendered value (wide runes count their terminal width, not their byte
// or rune count).
func (f TextField) CursorColumn() int {
	f.clampCursor()
	return lipgloss.Width(f.Value[:f.Cursor])
}

func (f *TextField) clampCursor() {
	if f.Cursor < 0 {
		f.Cursor = 0
	}
	if f.Cursor > len(f.Value) {
		f.Cursor = len(f.Value)
	}
	for f.Cursor > 0 && f.Cursor < len(f.Value) && !utf8.RuneStart(f.Value[f.Cursor]) {
		f.Cursor--
	}
}

// prevGraphemeBoundary returns the byte offset of the grapheme-cluster boundary
// immediately before pos (0 when pos is at or before the start).
func prevGraphemeBoundary(s string, pos int) int {
	prev := 0
	state := -1
	rest := s
	at := 0
	for len(rest) > 0 && at < pos {
		var cluster string
		cluster, rest, _, state = uniseg.StepString(rest, state)
		next := at + len(cluster)
		if next >= pos {
			return at
		}
		at = next
		prev = at
	}
	return prev
}

// nextGraphemeBoundary returns the byte offset of the grapheme-cluster boundary
// immediately after pos (len(s) when pos is at or past the end).
func nextGraphemeBoundary(s string, pos int) int {
	state := -1
	rest := s
	at := 0
	for len(rest) > 0 {
		var cluster string
		cluster, rest, _, state = uniseg.StepString(rest, state)
		next := at + len(cluster)
		if next > pos {
			return next
		}
		at = next
	}
	return len(s)
}

type textCluster struct {
	start int
	end   int
	word  bool
}

func wordClusters(s string) []textCluster {
	var clusters []textCluster
	state := -1
	rest := s
	at := 0
	for len(rest) > 0 {
		cluster, next, _, nextState := uniseg.StepString(rest, state)
		end := at + len(cluster)
		clusters = append(clusters, textCluster{
			start: at,
			end:   end,
			word:  isWordCluster(cluster),
		})
		rest, state, at = next, nextState, end
	}
	return clusters
}

func isWordCluster(cluster string) bool {
	for _, r := range cluster {
		if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
			return true
		}
	}
	return false
}

func prevWordBoundary(s string, pos int) int {
	if pos <= 0 {
		return 0
	}
	if pos > len(s) {
		pos = len(s)
	}
	clusters := wordClusters(s)
	i := len(clusters) - 1
	for i >= 0 && clusters[i].start >= pos {
		i--
	}
	for i >= 0 && !clusters[i].word {
		i--
	}
	if i < 0 {
		return 0
	}
	for i >= 0 && clusters[i].word {
		i--
	}
	return clusters[i+1].start
}

func nextWordBoundary(s string, pos int) int {
	if pos >= len(s) {
		return len(s)
	}
	if pos < 0 {
		pos = 0
	}
	clusters := wordClusters(s)
	i := 0
	for i < len(clusters) && clusters[i].end <= pos {
		i++
	}
	for i < len(clusters) && !clusters[i].word {
		i++
	}
	if i >= len(clusters) {
		return len(s)
	}
	for i < len(clusters) && clusters[i].word {
		i++
	}
	return clusters[i-1].end
}
