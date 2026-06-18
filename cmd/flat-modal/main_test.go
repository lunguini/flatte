package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
	"github.com/lunguini/flat/flatui"
)

func TestEnterOpensModalAndStartsWaiting(t *testing.T) {
	state := NewState()

	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

	if !state.modalOpen {
		t.Fatal("expected modal to open")
	}
	if !state.waiting {
		t.Fatal("expected background to wait for modal result")
	}
}

func TestModalCapturesInputAndConfirmCompletesWaiting(t *testing.T) {
	state := NewState()
	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'A'}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'd'}, flat.Effects[State]{})
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'a'}, flat.Effects[State]{})

	if state.modalInput.Value != "Ada" {
		t.Fatalf("modal input = %q, want Ada", state.modalInput.Value)
	}

	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

	if state.modalOpen {
		t.Fatal("expected modal to close after confirm")
	}
	if state.waiting {
		t.Fatal("expected waiting to stop after confirm")
	}
	if state.modalResult != "accepted: Ada" {
		t.Fatalf("modalResult = %q, want accepted: Ada", state.modalResult)
	}
}

func TestModalCapturesQInsteadOfQuitting(t *testing.T) {
	state := NewState()
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, fx)
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)

	if quit {
		t.Fatal("q should not quit while modal is open")
	}
	if state.modalInput.Value != "q" {
		t.Fatalf("modal input = %q, want q", state.modalInput.Value)
	}
}

func TestModalUsesAltBFForWordMovement(t *testing.T) {
	state := NewState()
	state.modalOpen = true
	state.modalInput.Value = "hello world"
	state.modalInput.Cursor = len("hello world")

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'b', Mod: flat.ModAlt}, flat.Effects[State]{})
	if state.modalInput.Cursor != len("hello ") {
		t.Fatalf("alt-b cursor = %d, want start of world", state.modalInput.Cursor)
	}
	if state.modalInput.Value != "hello world" {
		t.Fatalf("alt-b inserted text: %q", state.modalInput.Value)
	}
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'f', Mod: flat.ModAlt}, flat.Effects[State]{})
	if state.modalInput.Cursor != len("hello world") {
		t.Fatalf("alt-f cursor = %d, want end", state.modalInput.Cursor)
	}
}

func TestEscapeCancelsModal(t *testing.T) {
	state := NewState()
	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

	Handle(state, flat.KeyEvent{Key: flat.KeyEscape}, flat.Effects[State]{})

	if state.modalOpen {
		t.Fatal("expected modal to close after escape")
	}
	if state.waiting {
		t.Fatal("expected waiting to stop after cancel")
	}
	if state.modalResult != "cancelled" {
		t.Fatalf("modalResult = %q, want cancelled", state.modalResult)
	}
}

func TestBackgroundTickContinuesWhileModalIsOpen(t *testing.T) {
	state := NewState()
	Handle(state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

	applyTick(state, time.Time{})
	applyTick(state, time.Time{})

	if state.ticks != 2 {
		t.Fatalf("ticks = %d, want 2", state.ticks)
	}
	if state.spinner != 2 {
		t.Fatalf("spinner = %d, want 2", state.spinner)
	}
	if !state.modalOpen {
		t.Fatal("modal should remain open after background ticks")
	}
}

func TestQQuitsOnlyWhenModalIsClosed(t *testing.T) {
	state := NewState()
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("q should quit when modal is closed")
	}
}

func TestViewRendersMainAndModalState(t *testing.T) {
	state := NewState()
	state.ticks = 4
	state.spinner = 1
	state.waiting = true
	state.modalOpen = true
	state.modalInput.Value = "Ada"
	state.modalInput.Cursor = 1

	frame := View(state, flat.RenderContext{Width: 72}).Content

	for _, want := range []string{"Flat Modal", "background ticks: 4", "waiting \\", "Confirm Work", "Ada"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewMatchesModalSnapshot(t *testing.T) {
	state := NewState()
	state.ticks = 4
	state.spinner = 1
	state.waiting = true
	state.modalOpen = true
	state.modalInput.Value = "Ada"
	state.modalInput.Cursor = 1

	flatest.AssertGoldenFrame(t, "testdata/modal-open.golden", View(state, flat.RenderContext{Width: 72}))
}

func TestViewPlacesCursorInsideModal(t *testing.T) {
	state := NewState()
	if closed := View(state, flat.RenderContext{Width: 72}); closed.Cursor != nil {
		t.Fatalf("closed-modal view has a cursor: %+v", *closed.Cursor)
	}

	state.modalOpen = true
	state.modalInput.Value = "Ada"
	state.modalInput.Cursor = 1
	ctx := flat.RenderContext{Width: 72}
	frame := View(state, ctx)
	if frame.Cursor == nil {
		t.Fatal("open-modal view has no cursor")
	}
	// The expected position derives from the same helpers the app uses:
	// overlay placement + card origin + label width + typed cells.
	base := viewMain(state, ctx)
	modal := viewModal(state, ctx)
	overlayX, overlayY := flatui.OverlayOrigin(base, modal)
	cardX, cardY := flatui.CardOrigin()
	wantX := overlayX + cardX + len("  name: ") + 1
	wantY := overlayY + cardY + 3
	if frame.Cursor.X != wantX || frame.Cursor.Y != wantY {
		t.Fatalf("cursor = %+v, want (%d,%d)", *frame.Cursor, wantX, wantY)
	}
	if overlayX == 0 || overlayY == 0 {
		t.Fatalf("modal not centered (overlay origin %d,%d) - expected offsets inside a 72-col frame", overlayX, overlayY)
	}
}

func TestTickIntervalEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_MODAL_INTERVAL", "25ms")

	if got := tickInterval(); got != 25*time.Millisecond {
		t.Fatalf("tickInterval() = %s, want 25ms", got)
	}
}
