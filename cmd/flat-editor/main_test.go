package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

// settle sizes the textarea as a run does: compose the view at the frame
// size and deliver its rects.
func settle(s *State, width, height int) {
	_, comp := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: width}), width, height)
	Handle(s, flatte.LayoutEvent{Rects: comp.Rects}, flatte.Effects[State]{})
}

// composed is the frame View gives at width×height, as the host paints it.
func composed(s *State, width, height int) flatte.Frame {
	f, _ := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: width}), width, height)
	return f
}

func emptyState() *State {
	s := &State{}
	s.ta.SetValue("")
	s.ta.SetSoftWrap(true)
	settle(s, 80, 24)
	return s
}

// routed delivers ev as a host does: menu shortcuts become MenuEvents.
func routed(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	var m flatte.MenuHost
	app := flatte.App[State]{State: s, Menus: Menus, ContextMenu: ContextMenu}
	if ev, ok := flatte.RouteMenus(&m, app, ev, 80, 24); ok {
		Handle(s, ev, fx)
	}
}

func typeRunes(s *State, text string) {
	for _, r := range text {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}, flatte.Effects[State]{})
	}
}

func TestEditingInsertsAcrossNewlines(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hi")
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	typeRunes(s, "yo")
	if s.ta.Value() != "hi\nyo" {
		t.Fatalf("Value() = %q, want \"hi\\nyo\"", s.ta.Value())
	}
	if s.ta.Row() != 1 || s.ta.Col() != 2 {
		t.Fatalf("cursor = (%d,%d), want (1,2)", s.ta.Row(), s.ta.Col())
	}
}

func TestArrowsAndBackspaceEdit(t *testing.T) {
	s := emptyState()
	typeRunes(s, "abc")
	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft}, flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyBackspace}, flatte.Effects[State]{}) // remove 'b'
	if s.ta.Value() != "ac" {
		t.Fatalf("Value() = %q, want ac", s.ta.Value())
	}
}

func TestModifiedArrowsMoveByWord(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello world")

	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("alt-left cursor = (%d,%d), want start of world", s.ta.Row(), s.ta.Col())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != 0 {
		t.Fatalf("ctrl-left cursor = (%d,%d), want start", s.ta.Row(), s.ta.Col())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyRight, Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello") {
		t.Fatalf("ctrl-right cursor = (%d,%d), want end of hello", s.ta.Row(), s.ta.Col())
	}
}

func TestAltBFMoveByWord(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello world")

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'b', Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("alt-b cursor = (%d,%d), want start of world", s.ta.Row(), s.ta.Col())
	}
	if s.ta.Value() != "hello world" {
		t.Fatalf("alt-b inserted text: %q", s.ta.Value())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'f', Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello world") {
		t.Fatalf("alt-f cursor = (%d,%d), want end", s.ta.Row(), s.ta.Col())
	}
}

func TestHomeEndMoveToLineBoundaries(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello world")

	Handle(s, flatte.KeyEvent{Key: flatte.KeyHome}, flatte.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != 0 {
		t.Fatalf("home cursor = (%d,%d), want line start", s.ta.Row(), s.ta.Col())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnd}, flatte.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != len("hello world") {
		t.Fatalf("end cursor = (%d,%d), want line end", s.ta.Row(), s.ta.Col())
	}
}

func TestShiftHomeEndSelectLineRanges(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello world")

	Handle(s, flatte.KeyEvent{Key: flatte.KeyHome, Mod: flatte.ModShift}, flatte.Effects[State]{})
	if got := s.ta.SelectedText(); got != "hello world" {
		t.Fatalf("shift-home selection = %q, want full line", got)
	}

	typeRunes(s, "abc")
	Handle(s, flatte.KeyEvent{Key: flatte.KeyHome}, flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnd, Mod: flatte.ModShift}, flatte.Effects[State]{})
	if got := s.ta.SelectedText(); got != "abc" {
		t.Fatalf("shift-end selection = %q, want full line", got)
	}
}

func TestModifiedBackspaceAndDeleteRemoveWords(t *testing.T) {
	s := emptyState()
	s.ta.SetValue("hello world\nnext line")
	s.ta.SetSize(80, 4)
	Handle(s, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{})

	Handle(s, flatte.KeyEvent{Key: flatte.KeyBackspace, Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.ta.Value() != "hello next line" {
		t.Fatalf("ctrl-backspace value = %q, want %q", s.ta.Value(), "hello next line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("ctrl-backspace cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello "))
	}

	s.ta.SetValue("hello\nnext line")
	for range len("hello") {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyDelete, Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.ta.Value() != "hello line" {
		t.Fatalf("alt-delete value = %q, want %q", s.ta.Value(), "hello line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello") {
		t.Fatalf("alt-delete cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello"))
	}
}

func TestShiftArrowsSelectAndTypingReplacesSelection(t *testing.T) {
	s := emptyState()
	typeRunes(s, "abcd")

	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModShift}, flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModShift}, flatte.Effects[State]{})
	if got := s.ta.SelectedText(); got != "cd" {
		t.Fatalf("SelectedText() = %q, want %q", got, "cd")
	}

	typeRunes(s, "X")
	if s.ta.Value() != "abX" {
		t.Fatalf("Value after replacing selection = %q, want %q", s.ta.Value(), "abX")
	}
	if _, ok := s.ta.Selection(); ok {
		t.Fatal("selection still active after typing")
	}
}

func TestReadlineWordDeleteAliases(t *testing.T) {
	s := emptyState()
	s.ta.SetValue("hello world\nnext line")
	s.ta.SetSize(80, 4)
	Handle(s, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{})

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'w', Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.ta.Value() != "hello next line" {
		t.Fatalf("ctrl-w value = %q, want %q", s.ta.Value(), "hello next line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("ctrl-w cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello "))
	}

	s.ta.SetValue("hello\nnext line")
	for range len("hello") {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'd', Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if s.ta.Value() != "hello line" {
		t.Fatalf("alt-d value = %q, want %q", s.ta.Value(), "hello line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello") {
		t.Fatalf("alt-d cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello"))
	}
}

func TestCtrlAltHDWordDeleteAliases(t *testing.T) {
	s := emptyState()
	s.ta.SetValue("hello world\nnext line")
	s.ta.SetSize(80, 4)
	Handle(s, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{})

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'h', Mod: flatte.ModCtrl | flatte.ModAlt}, flatte.Effects[State]{})
	if s.ta.Value() != "hello next line" {
		t.Fatalf("ctrl-alt-h value = %q, want %q", s.ta.Value(), "hello next line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("ctrl-alt-h cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello "))
	}

	s.ta.SetValue("hello\nnext line")
	for range len("hello") {
		Handle(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.Effects[State]{})
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'd', Mod: flatte.ModCtrl | flatte.ModAlt}, flatte.Effects[State]{})
	if s.ta.Value() != "hello line" {
		t.Fatalf("ctrl-alt-d value = %q, want %q", s.ta.Value(), "hello line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello") {
		t.Fatalf("ctrl-alt-d cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello"))
	}
}

func TestCtrlHDeletesWordBackward(t *testing.T) {
	s := emptyState()
	s.ta.SetValue("hello world\nnext line")
	s.ta.SetSize(80, 4)
	Handle(s, flatte.KeyEvent{Key: flatte.KeyDown}, flatte.Effects[State]{})

	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'h', Mod: flatte.ModCtrl}, flatte.Effects[State]{})
	if s.ta.Value() != "hello next line" {
		t.Fatalf("ctrl-h value = %q, want %q", s.ta.Value(), "hello next line")
	}
	if s.ta.Row() != 0 || s.ta.Col() != len("hello ") {
		t.Fatalf("ctrl-h cursor = (%d,%d), want (0,%d)", s.ta.Row(), s.ta.Col(), len("hello "))
	}
}

func TestRawCtrlWDeletesWordThroughRun(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := emptyState()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- flatte.Run(context.Background(), flatte.App[State]{
			State:  state,
			Handle: Handle,
			View:   View,
		}, flatte.WithInput(reader), flatte.WithOutput(&out))
	}()

	_, _ = writer.Write([]byte("hello world\x17\x1b"))
	_ = writer.Close()

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not exit")
	}

	if state.ta.Value() != "hello " {
		t.Fatalf("raw ctrl-w value = %q, want %q", state.ta.Value(), "hello ")
	}
	if state.ta.Row() != 0 || state.ta.Col() != len("hello ") {
		t.Fatalf("raw ctrl-w cursor = (%d,%d), want (0,%d)", state.ta.Row(), state.ta.Col(), len("hello "))
	}
}

func TestEscQuits(t *testing.T) {
	s := emptyState()
	var quit bool
	fx := flatte.NewHarnessEffects[State](context.Background(), nil, func() { quit = true }, nil, nil)
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEscape}, fx)
	if !quit {
		t.Fatal("esc did not quit")
	}
}

func TestViewPlacesCursorAtOrigin(t *testing.T) {
	s := NewState()
	settle(s, 72, 24)
	frame := composed(s, 72, 24)
	if frame.Cursor == nil {
		t.Fatal("editor view has no cursor")
	}
	// card origin (3,1) + 3 pinned lines + cell (0,0) = (3,4)
	if frame.Cursor.X != 3 || frame.Cursor.Y != 4 {
		t.Fatalf("cursor = %+v, want (3,4)", *frame.Cursor)
	}
	if frame.Cursor.Style == nil || frame.Cursor.Style.Shape != flatte.CursorShapeBlock || !frame.Cursor.Style.Blink || frame.Cursor.Style.Color != nil {
		t.Fatalf("cursor style = %+v, want a blinking block in the terminal's color", frame.Cursor.Style)
	}
}

func TestViewSoftWrapsLongLineAndKeepsCursorInsideTextareaBody(t *testing.T) {
	s := emptyState()
	settle(s, 18, 10)
	typeRunes(s, "abcdefghijklmnopqrstuvwxyz")

	if got := s.ta.View(); got != "abcdefghijkl\nmnopqrstuvwx\nyz" {
		t.Fatalf("textarea view = %q, want wrapped rows", got)
	}
	frame := composed(s, 18, 10)
	if frame.Cursor == nil {
		t.Fatal("editor view has no cursor")
	}
	if frame.Cursor.X != 5 || frame.Cursor.Y != 6 {
		t.Fatalf("cursor = %+v, want (5,6)", *frame.Cursor)
	}
}

func TestDebugViewShowsLastDecodedKeyAndAction(t *testing.T) {
	s := emptyState()
	s.debugKeys = true
	settle(s, 72, 24)
	typeRunes(s, "hello world")
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'w', Mod: flatte.ModCtrl}, flatte.Effects[State]{})

	frame := composed(s, 72, 24)
	if !strings.Contains(frame.Content, "last: character 'w' ctrl -> delete-word-left") {
		t.Fatalf("debug footer missing last key/action:\n%s", frame.Content)
	}
}

func TestInitialSnapshot(t *testing.T) {
	s := NewState()
	settle(s, 72, 24)
	flatest.AssertGoldenFrame(t, "testdata/editor.golden", composed(s, 72, 24))
}

func TestMouseClickAndDragPlaceTheCaret(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello world")
	press := flatte.MouseEvent{Target: editorID, HasCaret: true, Caret: 6, Button: flatte.MouseLeft, Action: flatte.MousePress}
	Handle(s, press, flatte.Effects[State]{})
	if s.ta.Col() != 6 {
		t.Fatalf("click caret col %d, want 6", s.ta.Col())
	}
	drag := press
	drag.Action, drag.Caret = flatte.MouseMotion, 11
	Handle(s, drag, flatte.Effects[State]{})
	if got := s.ta.SelectedText(); got != "world" {
		t.Fatalf("drag selected %q, want world", got)
	}
	// Shift-click extends; clicks elsewhere or without a caret do nothing.
	shift := press
	shift.Mod, shift.Caret = flatte.ModShift, 0
	Handle(s, shift, flatte.Effects[State]{})
	if got := s.ta.SelectedText(); got != "hello " {
		t.Fatalf("shift-click selected %q", got)
	}
	Handle(s, flatte.MouseEvent{Target: "other", HasCaret: true, Caret: 3, Button: flatte.MouseLeft}, flatte.Effects[State]{})
	Handle(s, flatte.MouseEvent{Target: editorID, Button: flatte.MouseLeft}, flatte.Effects[State]{})
	if s.ta.Col() != 0 {
		t.Fatalf("stray clicks moved the caret to %d", s.ta.Col())
	}
}

func TestSuperChordsCopyCutSelectAllAndPaste(t *testing.T) {
	s := emptyState()
	typeRunes(s, "one two")
	var clip []string
	reads := 0
	fx := flatte.WithHostClipboard(flatte.Effects[State]{}, func(text string) { clip = append(clip, text) }, func() { reads++ })
	super := func(r rune) {
		routed(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r, Mod: flatte.ModSuper}, fx)
	}
	super('a')
	super('c')
	if len(clip) != 1 || clip[0] != "one two" || s.ta.Value() != "one two" {
		t.Fatalf("copy: clipboard %q, value %q", clip, s.ta.Value())
	}
	super('x')
	if len(clip) != 2 || clip[1] != "one two" || s.ta.Value() != "" {
		t.Fatalf("cut: clipboard %q, value %q", clip, s.ta.Value())
	}
	super('b') // unbound: not text
	if s.ta.Value() != "" {
		t.Fatalf("an unbound ⌘ chord typed %q", s.ta.Value())
	}
	Handle(s, flatte.PasteEvent{Text: "pasted\nlines"}, fx)
	if s.ta.Value() != "pasted\nlines" || s.ta.Row() != 1 {
		t.Fatalf("paste: %q at row %d", s.ta.Value(), s.ta.Row())
	}
	// ⌘V is the Paste command: it reads the clipboard, whose answer is
	// inserted.
	super('v')
	Handle(s, flatte.ClipboardEvent{Text: "!"}, fx)
	if reads != 1 || s.ta.Value() != "pasted\nlines!" {
		t.Fatalf("⌘V: %d reads, value %q", reads, s.ta.Value())
	}
	// Without a selection Cut and Copy are disabled: their keys do nothing.
	super('c')
	super('x')
	if len(clip) != 2 || s.ta.Value() != "pasted\nlines!" {
		t.Fatalf("disabled cut/copy acted: clip %q value %q", clip, s.ta.Value())
	}
}

func TestCopyOnSelectIsOptIn(t *testing.T) {
	for _, on := range []bool{false, true} {
		s := emptyState()
		s.copyOnSelect = on
		typeRunes(s, "hello")
		var clip []string
		fx := flatte.WithHostClipboard(flatte.Effects[State]{}, func(text string) { clip = append(clip, text) }, nil)
		drag := func(caret int, action flatte.MouseAction) {
			Handle(s, flatte.MouseEvent{Button: flatte.MouseLeft, Action: action, Target: editorID, Caret: caret, HasCaret: true}, fx)
		}
		drag(0, flatte.MousePress)
		drag(4, flatte.MouseMotion)
		if len(clip) != 0 {
			t.Fatalf("on=%v: copied mid-drag", on)
		}
		drag(4, flatte.MouseRelease)
		want := 0
		if on {
			want = 1
		}
		if len(clip) != want || on && clip[0] != "hell" {
			t.Fatalf("on=%v: clipboard %q after the drag", on, clip)
		}
		// A click without a selection copies nothing.
		drag(2, flatte.MousePress)
		drag(2, flatte.MouseRelease)
		if len(clip) != want {
			t.Fatalf("on=%v: a click copied %q", on, clip)
		}
	}
}

func TestMenus(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello")
	if ContextMenu(s, "elsewhere") != nil {
		t.Error("a context menu outside the editor")
	}
	items := ContextMenu(s, editorID)
	if len(items) != 5 || !items[0].Disabled || !items[1].Disabled || items[2].Disabled {
		t.Fatalf("context menu without a selection: %+v", items)
	}
	Handle(s, flatte.MenuEvent{ID: cmdSelectAll, Target: editorID}, flatte.Effects[State]{})
	if items := ContextMenu(s, editorID); items[0].Disabled || items[1].Disabled {
		t.Fatal("cut/copy disabled with a selection")
	}
	view := Menus(s)[1].Items
	if !view[0].Checked || view[1].Checked {
		t.Fatalf("view toggles %+v", view)
	}
	Handle(s, flatte.MenuEvent{ID: cmdWrap}, flatte.Effects[State]{})
	Handle(s, flatte.MenuEvent{ID: cmdKeys}, flatte.Effects[State]{})
	if s.ta.SoftWrap() || !s.debugKeys || Menus(s)[1].Items[0].Checked || !Menus(s)[1].Items[1].Checked {
		t.Fatal("view toggles did not toggle")
	}
}

// The terminal draws the context menu over the editor and delivers the
// choice.
func TestContextMenuInTheTerminal(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello world")
	d := flatest.Start(flatte.App[State]{State: s, Handle: Handle, View: View, Menus: Menus, ContextMenu: ContextMenu}, 72)
	d.Send(flatte.ResizeEvent{Width: 72, Height: 20})
	d.Send(flatte.MenuEvent{ID: cmdSelectAll})
	_, comp := flatte.ComposeFrame(d.Frame(), 72, 20)
	r := comp.Rects[editorID]
	f := d.Send(flatte.MouseEvent{X: r.X + 2, Y: r.Y, Button: flatte.MouseRight, Action: flatte.MousePress})
	flatest.AssertGoldenFrame(t, "testdata/context-menu.golden", f)
	d.Send(flatte.KeyEvent{Key: flatte.KeyDown}) // Copy
	d.Send(flatte.KeyEvent{Key: flatte.KeyDown}) // Paste
	d.Send(flatte.KeyEvent{Key: flatte.KeyUp})
	d.Send(flatte.KeyEvent{Key: flatte.KeyUp}) // Cut
	d.Send(flatte.KeyEvent{Key: flatte.KeyEnter})
	if s.ta.Value() != "" || s.lastAction != cmdCut {
		t.Fatalf("cut from the context menu: value %q action %q", s.ta.Value(), s.lastAction)
	}
}

func TestVerticalKeysUseTheHostsCaret(t *testing.T) {
	s := emptyState()
	typeRunes(s, "abc")
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	typeRunes(s, "defgh")
	// A window computed the row above as offset 1.
	Handle(s, flatte.KeyEvent{Key: flatte.KeyUp, HasCaret: true, Caret: 1}, flatte.Effects[State]{})
	if s.ta.Row() != 0 || s.ta.Col() != 1 {
		t.Fatalf("host up -> %d:%d, want 0:1", s.ta.Row(), s.ta.Col())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyDown, Mod: flatte.ModShift, HasCaret: true, Caret: 6}, flatte.Effects[State]{})
	if got := s.ta.SelectedText(); got != "bc\nde" {
		t.Fatalf("host shift-down selected %q", got)
	}
	// Without one, the textarea's own rows.
	Handle(s, flatte.KeyEvent{Key: flatte.KeyUp}, flatte.Effects[State]{})
	if s.ta.Row() != 0 {
		t.Fatalf("plain up -> row %d", s.ta.Row())
	}
}

func TestSuperArrowsMoveAndSelectLinesAndText(t *testing.T) {
	s := emptyState()
	typeRunes(s, "one two")
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	typeRunes(s, "three")
	super := func(k flatte.Key, mod flatte.Mod) {
		Handle(s, flatte.KeyEvent{Key: k, Mod: flatte.ModSuper | mod}, flatte.Effects[State]{})
	}
	super(flatte.KeyLeft, flatte.ModShift)
	if got := s.ta.SelectedText(); got != "three" {
		t.Fatalf("super+shift+left selected %q, want the line", got)
	}
	super(flatte.KeyRight, 0)
	if s.ta.Col() != 5 {
		t.Fatalf("super+right col %d, want the line end", s.ta.Col())
	}
	super(flatte.KeyUp, flatte.ModShift)
	if got := s.ta.SelectedText(); got != "one two\nthree" {
		t.Fatalf("super+shift+up selected %q, want everything", got)
	}
	super(flatte.KeyDown, 0)
	if s.ta.Row() != 1 || s.ta.Col() != 5 {
		t.Fatalf("super+down -> %d:%d, want the end", s.ta.Row(), s.ta.Col())
	}
	super(flatte.KeyBackspace, 0)
	if s.ta.Value() != "one two\n" {
		t.Fatalf("super+backspace left %q", s.ta.Value())
	}
}

func TestCtrlKeysCopyCutQuitAndMoveLines(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello")
	var clip []string
	quit := false
	fx := flatte.WithHostClipboard(flatte.NewHarnessEffects[State](context.Background(), nil, func() { quit = true }, nil, nil),
		func(text string) { clip = append(clip, text) }, nil)
	ctrl := func(r rune) { routed(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r, Mod: flatte.ModCtrl}, fx) }
	ctrl('a')
	if s.ta.Col() != 0 {
		t.Fatalf("ctrl+a col %d", s.ta.Col())
	}
	ctrl('e')
	if s.ta.Col() != 5 {
		t.Fatalf("ctrl+e col %d", s.ta.Col())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModShift}, fx)
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCtrlC}, fx)
	if quit || len(clip) != 1 || clip[0] != "o" {
		t.Fatalf("ctrl+c with a selection: quit %v clip %q", quit, clip)
	}
	ctrl('x')
	if s.ta.Value() != "hell" || len(clip) != 2 {
		t.Fatalf("ctrl+x: value %q clip %q", s.ta.Value(), clip)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCtrlC}, fx)
	if !quit {
		t.Fatal("ctrl+c without a selection did not quit")
	}
}

func TestCursorIsABarWhileSelecting(t *testing.T) {
	s := emptyState()
	typeRunes(s, "hello")
	if c := composed(s, 72, 24).Cursor; c == nil || c.Style.Shape != flatte.CursorShapeBlock {
		t.Fatalf("cursor without a selection = %+v, want a block", c)
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModShift}, flatte.Effects[State]{})
	if c := composed(s, 72, 24).Cursor; c == nil || c.Style.Shape != flatte.CursorShapeBar || !c.Style.Blink {
		t.Fatalf("cursor while selecting = %+v, want a blinking bar", c)
	}
}
