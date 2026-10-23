package flatui

import (
	"unicode"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/rivo/uniseg"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// TextField is a single-line editable string. Cursor is a byte offset into
// Value; the movement and edit methods keep it on a grapheme-cluster boundary
// so multi-rune clusters (combining marks, ZWJ emoji, regional-indicator flags)
// are treated as indivisible units and never split. SetCursor and the edit
// methods preserve the byte-offset API while normalizing offsets to grapheme
// boundaries; clampCursor additionally guards against a manually-set offset
// landing inside a UTF-8 rune.
type TextField struct {
	Value           string
	Cursor          int
	selectionActive bool
	selectionAnchor int
}

func (f *TextField) Insert(r rune) {
	f.clampCursor()
	f.deleteSelection()
	text := string(r)
	f.Value = f.Value[:f.Cursor] + text + f.Value[f.Cursor:]
	f.Cursor += len(text)
}

func (f *TextField) Backspace() {
	f.clampCursor()
	if f.deleteSelection() {
		return
	}
	if f.Cursor == 0 {
		return
	}
	start := prevGraphemeBoundary(f.Value, f.Cursor)
	f.Value = f.Value[:start] + f.Value[f.Cursor:]
	f.Cursor = start
}

func (f *TextField) Delete() {
	f.clampCursor()
	if f.deleteSelection() {
		return
	}
	if f.Cursor >= len(f.Value) {
		return
	}
	end := nextGraphemeBoundary(f.Value, f.Cursor)
	f.Value = f.Value[:f.Cursor] + f.Value[end:]
}

func (f *TextField) MoveLeft() {
	f.clampCursor()
	f.Cursor = prevGraphemeBoundary(f.Value, f.Cursor)
	f.ClearSelection()
}

func (f *TextField) MoveRight() {
	f.clampCursor()
	f.Cursor = nextGraphemeBoundary(f.Value, f.Cursor)
	f.ClearSelection()
}

func (f *TextField) MoveLeftSelecting() {
	f.startSelection()
	f.clampCursor()
	f.Cursor = prevGraphemeBoundary(f.Value, f.Cursor)
}

func (f *TextField) MoveRightSelecting() {
	f.startSelection()
	f.clampCursor()
	f.Cursor = nextGraphemeBoundary(f.Value, f.Cursor)
}

func (f *TextField) MoveWordLeft() {
	f.clampCursor()
	f.Cursor = prevWordBoundary(f.Value, f.Cursor)
	f.ClearSelection()
}

func (f *TextField) MoveWordRight() {
	f.clampCursor()
	f.Cursor = nextWordBoundary(f.Value, f.Cursor)
	f.ClearSelection()
}

func (f *TextField) MoveWordLeftSelecting() {
	f.startSelection()
	f.clampCursor()
	f.Cursor = prevWordBoundary(f.Value, f.Cursor)
}

func (f *TextField) MoveWordRightSelecting() {
	f.startSelection()
	f.clampCursor()
	f.Cursor = nextWordBoundary(f.Value, f.Cursor)
}

func (f *TextField) DeleteWordLeft() {
	f.clampCursor()
	if f.deleteSelection() {
		return
	}
	if f.Cursor == 0 {
		return
	}
	start := prevWordBoundary(f.Value, f.Cursor)
	f.Value = f.Value[:start] + f.Value[f.Cursor:]
	f.Cursor = start
}

func (f *TextField) DeleteWordRight() {
	f.clampCursor()
	if f.deleteSelection() {
		return
	}
	if f.Cursor >= len(f.Value) {
		return
	}
	end := nextWordBoundary(f.Value, f.Cursor)
	f.Value = f.Value[:f.Cursor] + f.Value[end:]
}

// MoveStart and MoveEnd put the cursor at the start or end of the value;
// the Selecting forms extend the selection there.
func (f *TextField) MoveStart() { f.SetCursor(0) }
func (f *TextField) MoveEnd()   { f.SetCursor(len(f.Value)) }

func (f *TextField) MoveStartSelecting() {
	f.startSelection()
	f.Cursor = 0
}

func (f *TextField) MoveEndSelecting() {
	f.startSelection()
	f.Cursor = len(f.Value)
}

// SelectAll selects the whole value, the cursor at its end.
func (f *TextField) SelectAll() {
	f.selectionAnchor, f.selectionActive = 0, true
	f.Cursor = len(f.Value)
}

// DeleteToStart and DeleteToEnd delete from the cursor to the start or end
// (or the selection, when there is one).
func (f *TextField) DeleteToStart() {
	f.clampCursor()
	if f.deleteSelection() {
		return
	}
	f.Value, f.Cursor = f.Value[f.Cursor:], 0
}

func (f *TextField) DeleteToEnd() {
	f.clampCursor()
	if f.deleteSelection() {
		return
	}
	f.Value = f.Value[:f.Cursor]
}

// Edit applies key as a native one-line text field does, and reports
// whether it was an editing key. It handles typing (replacing the
// selection), ←/→ (Shift selects, ⌥ or Ctrl by word, ⌘ to the start or
// end), Home/End (a window's ⌘←/⌘→), ⌫ and ⌦ (⌥ or Ctrl a word, ⌘ to the
// start or end), ⌘A, and the terminal's Ctrl+A/E/U/K/W. Everything else is
// the app's — Enter, Esc, Tab, ↑/↓, and copy, cut, and paste, which need
// the clipboard (SelectedText gives what to copy).
func (f *TextField) Edit(key flatte.KeyEvent) bool {
	shift := key.Mod.Contains(flatte.ModShift)
	super := key.Mod.Contains(flatte.ModSuper)
	word := key.Mod.Contains(flatte.ModAlt) || key.Mod.Contains(flatte.ModCtrl)
	switch key.Key {
	case flatte.KeyCharacter:
		switch {
		case key.Mod&^flatte.ModShift == 0:
			f.Insert(key.Rune)
		case super && (key.Rune == 'a' || key.Rune == 'A') && key.Mod&^(flatte.ModSuper|flatte.ModShift) == 0:
			f.SelectAll()
		case key.Mod == flatte.ModCtrl:
			switch key.Rune {
			case 'a':
				f.MoveStart()
			case 'e':
				f.MoveEnd()
			case 'u':
				f.DeleteToStart()
			case 'k':
				f.DeleteToEnd()
			case 'w':
				f.DeleteWordLeft()
			default:
				return false
			}
		default:
			return false
		}
	case flatte.KeyLeft, flatte.KeyRight:
		left := key.Key == flatte.KeyLeft
		switch {
		case super && shift && left:
			f.MoveStartSelecting()
		case super && shift:
			f.MoveEndSelecting()
		case super && left:
			f.MoveStart()
		case super:
			f.MoveEnd()
		case word && shift && left:
			f.MoveWordLeftSelecting()
		case word && shift:
			f.MoveWordRightSelecting()
		case word && left:
			f.MoveWordLeft()
		case word:
			f.MoveWordRight()
		case shift && left:
			f.MoveLeftSelecting()
		case shift:
			f.MoveRightSelecting()
		default:
			// A selection collapses to the edge the arrow points at.
			if start, end, ok := f.Selection(); ok && left {
				f.SetCursor(start)
			} else if ok {
				f.SetCursor(end)
			} else if left {
				f.MoveLeft()
			} else {
				f.MoveRight()
			}
		}
	case flatte.KeyHome, flatte.KeyEnd:
		home := key.Key == flatte.KeyHome
		switch {
		case shift && home:
			f.MoveStartSelecting()
		case shift:
			f.MoveEndSelecting()
		case home:
			f.MoveStart()
		default:
			f.MoveEnd()
		}
	case flatte.KeyBackspace:
		switch {
		case super:
			f.DeleteToStart()
		case word:
			f.DeleteWordLeft()
		default:
			f.Backspace()
		}
	case flatte.KeyDelete:
		switch {
		case super:
			f.DeleteToEnd()
		case word:
			f.DeleteWordRight()
		default:
			f.Delete()
		}
	default:
		return false
	}
	return true
}

func (f *TextField) SetCursor(cursor int) {
	f.Cursor = cursor
	f.clampCursor()
	f.ClearSelection()
}

// Selection returns the selected byte range, normalized as [start,end). The
// range is false when no non-empty selection is active.
func (f TextField) Selection() (start, end int, ok bool) {
	if !f.selectionActive {
		return 0, 0, false
	}
	cursor := f.clampedOffset(f.Cursor)
	anchor := f.clampedOffset(f.selectionAnchor)
	if cursor == anchor {
		return 0, 0, false
	}
	if anchor < cursor {
		return anchor, cursor, true
	}
	return cursor, anchor, true
}

func (f TextField) SelectedText() string {
	start, end, ok := f.Selection()
	if !ok {
		return ""
	}
	return f.Value[start:end]
}

func (f *TextField) ClearSelection() {
	f.selectionActive = false
	f.selectionAnchor = 0
}

// CursorColumn returns the cursor offset in display cells within the
// rendered value (wide runes count their terminal width, not their byte
// or rune count).
func (f TextField) CursorColumn() int {
	f.clampCursor()
	return lipgloss.Width(f.Value[:f.Cursor])
}

// Layout returns the field as a layout.Input node with the given ID, showing
// the value, caret, and selection. focused is the app's focus decision: the
// focused Input's caret places the frame cursor.
func (f TextField) Layout(id string, focused bool) layout.Input {
	f.clampCursor()
	in := layout.Input{NodeBase: layout.NodeBase{ID: id}, Value: f.Value, Caret: f.Cursor, Focused: focused}
	if start, end, ok := f.Selection(); ok {
		in.SelStart, in.SelEnd = start, end
	}
	return in
}

func (f *TextField) startSelection() {
	f.clampCursor()
	if !f.selectionActive {
		f.selectionAnchor = f.Cursor
		f.selectionActive = true
	}
}

func (f *TextField) deleteSelection() bool {
	start, end, ok := f.Selection()
	if !ok {
		return false
	}
	f.Value = f.Value[:start] + f.Value[end:]
	f.Cursor = start
	f.ClearSelection()
	return true
}

func (f TextField) clampedOffset(offset int) int {
	f.Cursor = offset
	f.clampCursor()
	return f.Cursor
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
	f.Cursor = graphemeBoundaryAtOrBefore(f.Value, f.Cursor)
}

// graphemeBoundaryAtOrBefore returns pos when it is a grapheme boundary, or
// the start of the containing cluster when pos falls inside one. The result
// is always a byte offset into s and is clamped to [0,len(s)].
func graphemeBoundaryAtOrBefore(s string, pos int) int {
	if pos <= 0 {
		return 0
	}
	if pos >= len(s) {
		return len(s)
	}
	// nextGraphemeBoundary is strict (it returns the end of the cluster that
	// contains pos), so asking about the preceding byte distinguishes an exact
	// boundary from an offset inside the preceding cluster.
	if nextGraphemeBoundary(s, pos-1) == pos {
		return pos
	}
	return prevGraphemeBoundary(s, pos)
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
