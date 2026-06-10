package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatuitest"
)

func TestEnterOpensModalAndStartsWaiting(t *testing.T) {
	state := NewState()

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})

	if !state.modalOpen {
		t.Fatal("expected modal to open")
	}
	if !state.waiting {
		t.Fatal("expected background to wait for modal result")
	}
}

func TestModalCapturesInputAndConfirmCompletesWaiting(t *testing.T) {
	state := NewState()
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'A'}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'd'}, flatcore.Effects[State]{})
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'a'}, flatcore.Effects[State]{})

	if state.modalInput.Value != "Ada" {
		t.Fatalf("modal input = %q, want Ada", state.modalInput.Value)
	}

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})

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
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, fx)
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'q'}, fx)

	if quit {
		t.Fatal("q should not quit while modal is open")
	}
	if state.modalInput.Value != "q" {
		t.Fatalf("modal input = %q, want q", state.modalInput.Value)
	}
}

func TestEscapeCancelsModal(t *testing.T) {
	state := NewState()
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEscape}, flatcore.Effects[State]{})

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
	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})

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
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'q'}, fx)

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

	frame := View(state, flatcore.RenderContext{Width: 72})

	for _, want := range []string{"Flat Modal", "background ticks: 4", "waiting \\", "Confirm Work", "A▌da"} {
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

	flatuitest.AssertGolden(t, "testdata/modal-open.golden", View(state, flatcore.RenderContext{Width: 72}))
}

func TestTickIntervalEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_MODAL_INTERVAL", "25ms")

	if got := tickInterval(); got != 25*time.Millisecond {
		t.Fatalf("tickInterval() = %s, want 25ms", got)
	}
}
