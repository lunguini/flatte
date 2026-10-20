package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func TestEnterOpensModalAndStartsWaiting(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})

	if !state.modalOpen {
		t.Fatal("expected modal to open")
	}
	if !state.waiting {
		t.Fatal("expected background to wait for modal result")
	}
}

func TestModalCapturesInputAndConfirmCompletesWaiting(t *testing.T) {
	state := NewState()
	Handle(state, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'A'}, flatte.Effects[State]{})
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'd'}, flatte.Effects[State]{})
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'a'}, flatte.Effects[State]{})

	if state.modalInput.Value != "Ada" {
		t.Fatalf("modal input = %q, want Ada", state.modalInput.Value)
	}

	Handle(state, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})

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
	fx := flatte.NewHarnessEffects[State](context.Background(), nil, func() { quit = true }, nil, nil)
	Handle(state, flatte.KeyEvent{Key: flatte.KeyEnter}, fx)
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)

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

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'b', Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if state.modalInput.Cursor != len("hello ") {
		t.Fatalf("alt-b cursor = %d, want start of world", state.modalInput.Cursor)
	}
	if state.modalInput.Value != "hello world" {
		t.Fatalf("alt-b inserted text: %q", state.modalInput.Value)
	}
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'f', Mod: flatte.ModAlt}, flatte.Effects[State]{})
	if state.modalInput.Cursor != len("hello world") {
		t.Fatalf("alt-f cursor = %d, want end", state.modalInput.Cursor)
	}
}

func TestEscapeCancelsModal(t *testing.T) {
	state := NewState()
	Handle(state, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})

	Handle(state, flatte.KeyEvent{Key: flatte.KeyEscape}, flatte.Effects[State]{})

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
	Handle(state, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})

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
	fx := flatte.NewHarnessEffects[State](context.Background(), nil, func() { quit = true }, nil, nil)
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("q should quit when modal is closed")
	}
}

// openModal returns a driver at 72×24 with the modal open on "Ada", the
// caret after "A", and four background ticks applied.
func openModal() *flatest.Driver[State] {
	state := NewState()
	state.ticks = 4
	state.spinner = 1
	state.waiting = true
	state.modalOpen = true
	state.modalInput.Value = "Ada"
	state.modalInput.Cursor = 1
	return flatest.Start(flatte.App[State]{State: state, Handle: Handle, View: View}, 72)
}

func TestViewRendersMainAndModalState(t *testing.T) {
	frame := flatest.CleanFrame(openModal().Frame().Content)
	for _, want := range []string{"Flat Modal", "background ticks: 4", "waiting \\", "Confirm Work", "Ada"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewMatchesModalSnapshot(t *testing.T) {
	flatest.AssertGoldenFrame(t, "testdata/modal-open.golden", openModal().Frame())
	closed := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 72)
	flatest.AssertGoldenFrame(t, "testdata/modal-closed.golden", closed.Frame())
}

// TestViewPlacesCursorInsideModal reads the painted frame: the cursor must
// sit on the name field, between the typed "A" and "da".
func TestViewPlacesCursorInsideModal(t *testing.T) {
	closed := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 72)
	if c := closed.Frame().Cursor; c != nil {
		t.Fatalf("closed-modal view has a cursor: %+v", *c)
	}
	frame := openModal().Frame()
	if frame.Cursor == nil {
		t.Fatal("open-modal view has no cursor")
	}
	lines := strings.Split(flatest.CleanFrame(frame.Content), "\n")
	row := []rune(lines[frame.Cursor.Y])
	if got := string(row[frame.Cursor.X-len("name: A") : frame.Cursor.X+2]); got != "name: Ada" {
		t.Fatalf("cursor (%d,%d) is not after \"name: A\": row %q", frame.Cursor.X, frame.Cursor.Y, string(row))
	}
	if frame.Cursor.X < 20 || frame.Cursor.Y < 5 {
		t.Fatalf("cursor (%d,%d) not inside a centered modal", frame.Cursor.X, frame.Cursor.Y)
	}
}

// TestTypingMovesCursor: the cursor follows the field as the user types,
// through the runtime path (no View-side arithmetic to keep in sync).
func TestTypingMovesCursor(t *testing.T) {
	d := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 72)
	start := d.Send(flatte.KeyEvent{Key: flatte.KeyEnter}).Cursor
	if start == nil {
		t.Fatal("no cursor after opening the modal")
	}
	after := d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '日'}).Cursor
	if after == nil || after.X != start.X+2 || after.Y != start.Y {
		t.Fatalf("cursor after a wide rune = %+v, want two cells right of %+v", after, *start)
	}
	if c := d.Send(flatte.KeyEvent{Key: flatte.KeyEscape}).Cursor; c != nil {
		t.Fatalf("cursor after closing the modal = %+v", *c)
	}
}

func TestTickIntervalEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_MODAL_INTERVAL", "25ms")

	if got := tickInterval(); got != 25*time.Millisecond {
		t.Fatalf("tickInterval() = %s, want 25ms", got)
	}
}
