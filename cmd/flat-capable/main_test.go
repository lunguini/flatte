package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

// A zero-enqueue Effects makes SetClipboard/ReadClipboard/Suspend/Exec
// safe no-ops, so Handle's state transitions are testable without a live
// terminal.
func noEffects() flat.Effects[State] {
	return flat.Effects[State]{}
}

func TestCopyKeySetsStatus(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'y'}, noEffects())

	if state.status != "copied to clipboard" {
		t.Fatalf("status = %q, want copied to clipboard", state.status)
	}
}

func TestPasteKeyRequestsRead(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'p'}, noEffects())

	if !strings.HasPrefix(state.status, "requested clipboard read") {
		t.Fatalf("status = %q, want a read-requested message", state.status)
	}
}

func TestCtrlZRequestsSuspend(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'z', Mod: flat.ModCtrl}, noEffects())

	if state.status != "suspended; resumed" {
		t.Fatalf("status = %q, want suspended status", state.status)
	}
}

func TestModifiedPlainCommandsAreIgnored(t *testing.T) {
	state := NewState()
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'z', Mod: flat.ModAlt}, fx)
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'y', Mod: flat.ModCtrl}, fx)
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q', Mod: flat.ModAlt}, fx)

	if state.status != "ready" {
		t.Fatalf("status = %q, want ready", state.status)
	}
	if quit {
		t.Fatal("modified q should not request quit")
	}
}

func TestClipboardEventStoresContent(t *testing.T) {
	state := NewState()

	Handle(state, flat.ClipboardEvent{Text: "pasted text"}, noEffects())

	if state.clipboard != "pasted text" {
		t.Fatalf("clipboard = %q, want pasted text", state.clipboard)
	}
	if state.status != "clipboard read" {
		t.Fatalf("status = %q, want clipboard read", state.status)
	}
}

func TestQuitKeyRequestsQuit(t *testing.T) {
	state := NewState()
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("q should request quit")
	}
}

func TestViewMatchesSnapshot(t *testing.T) {
	state := &State{
		status:     "clipboard read",
		clipboard:  "hello from the system clipboard",
		editorText: "edited line",
	}

	flatest.AssertGolden(t, "testdata/capable.golden", View(state, flat.RenderContext{Width: 72}).Content)
}
