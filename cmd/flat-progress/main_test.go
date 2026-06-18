package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
	"github.com/lunguini/flat/flatui"
)

func TestProgressTickAdvancesUntilComplete(t *testing.T) {
	state := NewState()

	applyTick(state, time.Time{})
	if got, want := state.progress.Percent(), 10.0; got != want {
		t.Fatalf("percent = %.1f, want %.1f", got, want)
	}

	state.paused = true
	applyTick(state, time.Time{})
	if got, want := state.progress.Percent(), 10.0; got != want {
		t.Fatalf("paused percent = %.1f, want unchanged %.1f", got, want)
	}

	state.paused = false
	for range 20 {
		applyTick(state, time.Time{})
	}
	if got, want := state.progress.Percent(), 100.0; got != want {
		t.Fatalf("complete percent = %.1f, want %.1f", got, want)
	}
}

func TestHandleTogglesPauseResetsAndQuits(t *testing.T) {
	state := NewState()
	state.progress.SetPercent(40)

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: ' '}, flat.Effects[State]{})
	if !state.paused {
		t.Fatal("space should pause")
	}

	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'r'}, flat.Effects[State]{})
	if got := state.progress.Percent(); got != 0 {
		t.Fatalf("percent after reset = %.1f, want 0", got)
	}

	var quit bool
	fx := flat.NewEffects[State](t.Context(), nil, func() { quit = true })
	Handle(state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q should request quit")
	}
}

func TestResizeSetsProgressWidth(t *testing.T) {
	state := NewState()

	Handle(state, flat.ResizeEvent{Width: 72, Height: 24}, flat.Effects[State]{})

	if got, want := state.progress.Width(), flatui.CardBodyWidth(72)-8; got != want {
		t.Fatalf("progress width = %d, want %d", got, want)
	}
}

func TestViewRendersProgressState(t *testing.T) {
	state := NewState()
	state.progress.SetPercent(30)

	frame := View(state, flat.RenderContext{Width: 72}).Content

	for _, want := range []string{"Flat Progress", "30%", "space pause", "r reset"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestProgressSequenceSnapshot(t *testing.T) {
	t.Setenv("FLAT_PROGRESS_INTERVAL", "10ms")

	d := flatest.Start(flat.App[State]{
		State:  NewState(),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}, 72)

	frames := flatest.Frames(d,
		func(d *flatest.Driver[State]) {},                                   // 0%
		func(d *flatest.Driver[State]) { d.Advance(10 * time.Millisecond) }, // 10%
		func(d *flatest.Driver[State]) { d.Advance(30 * time.Millisecond) }, // 40%
		func(d *flatest.Driver[State]) {
			d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: ' '})
			d.Advance(30 * time.Millisecond)
		}, // paused: still 40%
		func(d *flatest.Driver[State]) {
			d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'r'})
		}, // reset: 0%
	)

	flatest.AssertFrames(t, "testdata/progress-sequence.golden", frames)
}
