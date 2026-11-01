package flatui

import (
	"testing"

	"github.com/lunguini/flatte"
)

func k(key flatte.Key, mod flatte.Mod) flatte.KeyEvent { return flatte.KeyEvent{Key: key, Mod: mod} }
func ch(r rune, mod flatte.Mod) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r, Mod: mod}
}

func TestTextFieldEdit(t *testing.T) {
	type step struct {
		key              flatte.KeyEvent
		value, selection string
		cursor           int
	}
	for _, c := range []struct {
		name  string
		start string
		steps []step
	}{
		{"type and select all", "", []step{
			{ch('h', 0), "h", "", 1},
			{ch('I', flatte.ModShift), "hI", "", 2},
			{ch('a', flatte.ModSuper), "hI", "hI", 2},
			{ch('x', 0), "x", "", 1}, // typing replaces the selection
		}},
		{"⌘ arrows and Home/End", "hello world", []step{
			{k(flatte.KeyLeft, flatte.ModSuper), "hello world", "", 0},
			{k(flatte.KeyRight, flatte.ModSuper|flatte.ModShift), "hello world", "hello world", 11},
			{k(flatte.KeyLeft, 0), "hello world", "", 0}, // collapses to the start
			{k(flatte.KeyEnd, 0), "hello world", "", 11},
			{k(flatte.KeyHome, flatte.ModShift), "hello world", "hello world", 0},
			{k(flatte.KeyRight, 0), "hello world", "", 11}, // collapses to the end
		}},
		{"words", "hello big world", []step{
			{k(flatte.KeyLeft, flatte.ModAlt), "hello big world", "", 10},
			{k(flatte.KeyLeft, flatte.ModAlt|flatte.ModShift), "hello big world", "big ", 6},
			{k(flatte.KeyBackspace, 0), "hello world", "", 6},
			{k(flatte.KeyBackspace, flatte.ModAlt), "world", "", 0},
		}},
		{"⌘⌫ and ⌘⌦", "abc def", []step{
			{k(flatte.KeyLeft, 0), "abc def", "", 6},
			{k(flatte.KeyBackspace, flatte.ModSuper), "f", "", 0},
			{k(flatte.KeyDelete, flatte.ModSuper), "", "", 0},
		}},
		{"terminal Ctrl keys", "abc def", []step{
			{ch('a', flatte.ModCtrl), "abc def", "", 0},
			{ch('e', flatte.ModCtrl), "abc def", "", 7},
			{ch('w', flatte.ModCtrl), "abc ", "", 4},
			{ch('u', flatte.ModCtrl), "", "", 0},
		}},
	} {
		f := TextField{Value: c.start, Cursor: len(c.start)}
		for i, st := range c.steps {
			if !f.Edit(st.key) {
				t.Fatalf("%s step %d: %+v not handled", c.name, i, st.key)
			}
			if f.Value != st.value || f.SelectedText() != st.selection || f.Cursor != st.cursor {
				t.Fatalf("%s step %d: value %q selection %q cursor %d, want %q %q %d", c.name, i, f.Value, f.SelectedText(), f.Cursor, st.value, st.selection, st.cursor)
			}
		}
	}
	// The app's keys stay the app's.
	f := TextField{Value: "x", Cursor: 1}
	for _, key := range []flatte.KeyEvent{k(flatte.KeyEnter, 0), k(flatte.KeyEscape, 0), k(flatte.KeyTab, 0), k(flatte.KeyUp, 0), ch('c', flatte.ModSuper), ch('v', flatte.ModSuper), ch('q', flatte.ModCtrl)} {
		if f.Edit(key) || f.Value != "x" {
			t.Errorf("%+v was taken (value %q)", key, f.Value)
		}
	}
	// The selection reaches the Input.
	f.SelectAll()
	if in := f.Layout("q", true); in.SelStart != 0 || in.SelEnd != 1 {
		t.Errorf("layout selection %d..%d", in.SelStart, in.SelEnd)
	}
}
