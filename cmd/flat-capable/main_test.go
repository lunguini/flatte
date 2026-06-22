package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

// A zero-enqueue Effects makes SetClipboard/ReadClipboard/Suspend/Exec
// safe no-ops, so Handle's state transitions are testable without a live
// terminal.
func noEffects() flatte.Effects[State] {
	return flatte.Effects[State]{}
}

func TestCopyKeySetsStatus(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'y'}, noEffects())

	if state.status != "copied to clipboard" {
		t.Fatalf("status = %q, want copied to clipboard", state.status)
	}
}

func TestPasteKeyRequestsRead(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'p'}, noEffects())

	if !strings.HasPrefix(state.status, "requested clipboard read") {
		t.Fatalf("status = %q, want a read-requested message", state.status)
	}
}

func TestCtrlZRequestsSuspend(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'z', Mod: flatte.ModCtrl}, noEffects())

	if state.status != "suspended; resumed" {
		t.Fatalf("status = %q, want suspended status", state.status)
	}
}

func TestModifiedPlainCommandsAreIgnored(t *testing.T) {
	state := NewState()
	var quit bool
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'z', Mod: flatte.ModAlt}, fx)
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'y', Mod: flatte.ModCtrl}, fx)
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q', Mod: flatte.ModAlt}, fx)

	if state.status != "ready" {
		t.Fatalf("status = %q, want ready", state.status)
	}
	if quit {
		t.Fatal("modified q should not request quit")
	}
}

func TestClipboardEventStoresContent(t *testing.T) {
	state := NewState()

	Handle(state, flatte.ClipboardEvent{Text: "pasted text"}, noEffects())

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
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("q should request quit")
	}
}

func TestRunOptionsEnableInlineFromEnvironment(t *testing.T) {
	t.Setenv("FLAT_CAPABLE_INLINE", "1")

	if got := len(runOptions()); got != 1 {
		t.Fatalf("runOptions() len = %d, want WithInline option", got)
	}
}

func TestViewMatchesSnapshot(t *testing.T) {
	state := &State{
		status:     "clipboard read",
		clipboard:  "hello from the system clipboard",
		editorText: "edited line",
	}

	flatest.AssertGolden(t, "testdata/capable.golden", View(state, flatte.RenderContext{Width: 72}).Content)
}
