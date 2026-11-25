package flatui

import (
	"testing"

	"github.com/lunguini/flatte"
)

var _ Clipboard = flatte.Effects[int]{}

type fakeClipboard struct {
	text  string
	reads int
}

func (c *fakeClipboard) SetClipboard(s string) { c.text = s }
func (c *fakeClipboard) ReadClipboard()        { c.reads++ }

func chord(r rune, mod flatte.Mod) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r, Mod: mod}
}

type testInput interface {
	input
	Handle(string, flatte.Event, Clipboard) bool
	Do(string, Clipboard) bool
	EditMenu() []flatte.MenuItem
	CanUndo() bool
	CanRedo() bool
}

func inputs() map[string]func() testInput {
	return map[string]func() testInput{
		"field": func() testInput { return &TextField{} },
		"area":  func() testInput { return &Textarea{} },
	}
}

func typeIn(in testInput, clip Clipboard, s string) {
	for _, r := range s {
		in.Handle("x", chord(r, 0), clip)
	}
}

func TestInputClipboard(t *testing.T) {
	for name, mk := range inputs() {
		t.Run(name, func(t *testing.T) {
			in, clip := mk(), &fakeClipboard{}
			typeIn(in, clip, "a.go")
			// Nothing selected: copy and cut are taken and do nothing.
			if !in.Handle("x", chord('c', flatte.ModSuper), clip) || !in.Handle("x", chord('x', flatte.ModSuper), clip) || clip.text != "" || in.state().value != "a.go" {
				t.Fatalf("copied %q of nothing", clip.text)
			}
			in.Handle("x", chord('a', flatte.ModSuper), clip)
			in.Handle("x", chord('c', flatte.ModSuper), clip)
			if clip.text != "a.go" || in.SelectedText() != "a.go" {
				t.Fatalf("copied %q, left %q selected", clip.text, in.SelectedText())
			}
			clip.text = ""
			in.Handle("x", chord('X', flatte.ModCtrl), clip)
			if clip.text != "a.go" || in.state().value != "" {
				t.Fatalf("cut %q, left %q", clip.text, in.state().value)
			}
			in.Handle("x", chord('v', flatte.ModSuper), clip)
			if clip.reads != 1 {
				t.Fatal("paste did not ask for the clipboard")
			}
			in.Handle("x", flatte.ClipboardEvent{Text: "b"}, clip)
			in.Handle("x", flatte.PasteEvent{Text: "c"}, clip)
			if in.state().value != "bc" {
				t.Fatalf("pasted %q", in.state().value)
			}
		})
	}
}

func TestInputUndo(t *testing.T) {
	for name, mk := range inputs() {
		t.Run(name, func(t *testing.T) {
			in, clip := mk(), &fakeClipboard{}
			in.InsertText("old ") // the app's: not the user's to undo
			if in.CanUndo() {
				t.Fatal("undo of what the app put there")
			}
			typeIn(in, clip, "one two")
			in.Handle("x", flatte.KeyEvent{Key: flatte.KeyBackspace}, clip)
			want := func(s string) {
				t.Helper()
				if got := in.state().value; got != s {
					t.Fatalf("%q, want %q", got, s)
				}
			}
			undo, redo := chord('z', flatte.ModSuper), chord('z', flatte.ModSuper|flatte.ModShift)
			in.Handle("x", undo, clip)
			want("old one two")
			in.Handle("x", undo, clip)
			want("old one")
			in.Handle("x", undo, clip)
			want("old ")
			in.Handle("x", undo, clip) // nothing left
			want("old ")
			in.Handle("x", redo, clip)
			in.Handle("x", redo, clip)
			want("old one two")
			typeIn(in, clip, "!")
			if in.CanRedo() {
				t.Fatal("redo after a new edit")
			}
			in.Do(EditSelectAll, clip)
			typeIn(in, clip, "z") // replaces the selection
			in.Do(EditUndo, clip)
			want("old one two!")
			if in.SelectedText() != "old one two!" {
				t.Fatalf("undo left %q selected", in.SelectedText())
			}
			in.Do(EditDelete, clip)
			want("")
		})
	}
}

func TestInputUndoIsBounded(t *testing.T) {
	f, clip := &TextField{}, &fakeClipboard{}
	for range 5 * maxUndo {
		f.Handle("x", flatte.PasteEvent{Text: "a"}, clip)
	}
	n := 0
	for ; f.CanUndo(); n++ {
		f.Do(EditUndo, clip)
	}
	if n < maxUndo || n > 2*maxUndo {
		t.Fatalf("%d undos", n)
	}
}

// A copied input keeps its own history.
func TestInputCopiesUndoApart(t *testing.T) {
	a, clip := TextField{}, &fakeClipboard{}
	a.Handle("x", flatte.PasteEvent{Text: "1"}, clip)
	b := a
	a.Handle("x", flatte.PasteEvent{Text: "2"}, clip)
	b.Handle("x", flatte.PasteEvent{Text: "3"}, clip)
	a.Do(EditUndo, clip)
	b.Do(EditUndo, clip)
	if a.Value != "1" || b.Value != "1" {
		t.Fatalf("%q %q", a.Value, b.Value)
	}
}

func TestInputMenu(t *testing.T) {
	for name, mk := range inputs() {
		t.Run(name, func(t *testing.T) {
			in, clip := mk(), &fakeClipboard{}
			enabled := func() (ids string) {
				for _, it := range in.EditMenu() {
					if !it.Separator && !it.Disabled {
						ids += it.Label + ","
					}
				}
				return ids
			}
			if got := enabled(); got != "Paste," {
				t.Fatal(got)
			}
			typeIn(in, clip, "a")
			if got := enabled(); got != "Undo,Paste,Select All," {
				t.Fatal(got)
			}
			if in.Handle("x", flatte.MenuEvent{ID: EditSelectAll, Target: "y"}, clip) {
				t.Fatal("took another node's menu")
			}
			if in.Handle("x", flatte.MenuEvent{ID: "app.thing", Target: "x"}, clip) {
				t.Fatal("took the app's command")
			}
			in.Handle("x", flatte.MenuEvent{ID: EditSelectAll, Target: "x"}, clip)
			in.Handle("x", flatte.MenuEvent{ID: EditCut, Target: "x"}, clip)
			if clip.text != "a" || in.state().value != "" {
				t.Fatalf("cut %q", clip.text)
			}
			if got := enabled(); got != "Undo,Paste," {
				t.Fatal(got)
			}
		})
	}
}

// Ctrl+A and Ctrl+K keep their terminal meaning, and an app's chords pass.
func TestInputLeavesOtherChords(t *testing.T) {
	f, clip := &TextField{}, &fakeClipboard{}
	typeIn(f, clip, "ab")
	f.Handle("x", chord('a', flatte.ModCtrl), clip)
	if f.Cursor != 0 || f.SelectedText() != "" {
		t.Fatal("Ctrl+A is not the line's start")
	}
	for _, k := range []flatte.KeyEvent{chord('p', flatte.ModSuper), chord('c', flatte.ModSuper|flatte.ModShift), chord('z', flatte.ModSuper|flatte.ModAlt), {Key: flatte.KeyEnter}} {
		if f.Handle("x", k, clip) {
			t.Errorf("took %+v", k)
		}
	}
}

func TestTextareaMouse(t *testing.T) {
	a, clip := &Textarea{}, &fakeClipboard{}
	a.SetValue("one two\nthree")
	press := func(action flatte.MouseAction, at, clicks int) bool {
		return a.Handle("x", flatte.MouseEvent{Target: "x", Button: flatte.MouseLeft, Action: action, HasCaret: true, Caret: at, Clicks: clicks}, clip)
	}
	press(flatte.MousePress, 1, 1)
	press(flatte.MouseMotion, 9, 0)
	press(flatte.MouseRelease, 9, 0)
	if got := a.SelectedText(); got != "ne two\nt" {
		t.Fatalf("drag selected %q", got)
	}
	if press(flatte.MouseMotion, 2, 0) {
		t.Fatal("a motion without a press")
	}
	press(flatte.MousePress, 5, 2)
	if got := a.SelectedText(); got != "two" {
		t.Fatalf("double click selected %q", got)
	}
	press(flatte.MousePress, 10, 3)
	if got := a.SelectedText(); got != "three" {
		t.Fatalf("triple click selected %q", got)
	}
}
