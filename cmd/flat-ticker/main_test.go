package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func TestTickUpdateIncrementsOnlyWhenRunning(t *testing.T) {
	state := State{}

	applyTick(&state, time.Time{})
	if state.ticks != 1 {
		t.Fatalf("ticks = %d, want 1", state.ticks)
	}

	state.paused = true
	applyTick(&state, time.Time{})
	if state.ticks != 1 {
		t.Fatalf("paused ticks = %d, want unchanged", state.ticks)
	}
}

func TestHandleTogglesPauseAndResets(t *testing.T) {
	state := State{ticks: 5}

	Handle(&state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'p'}, flat.Effects[State]{})
	if !state.paused {
		t.Fatal("expected paused state after p")
	}

	Handle(&state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'r'}, flat.Effects[State]{})
	if state.ticks != 0 {
		t.Fatalf("ticks = %d, want reset to 0", state.ticks)
	}
}

func TestViewRendersTickerState(t *testing.T) {
	state := State{ticks: 3, paused: true}

	frame := View(&state, flat.RenderContext{Width: 72}).Content

	for _, want := range []string{"Flat Ticker", "ticks: 3", "state: paused"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewMatchesPausedSnapshot(t *testing.T) {
	state := State{ticks: 3, paused: true}

	flatest.AssertGolden(t, "testdata/paused.golden", View(&state, flat.RenderContext{Width: 72}).Content)
}

func TestTickIntervalEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_TICKER_INTERVAL", "25ms")

	if got := tickInterval(); got != 25*time.Millisecond {
		t.Fatalf("tickInterval() = %s, want 25ms", got)
	}
}

func TestTicksAreDeterministicUnderFakeClock(t *testing.T) {
	t.Setenv("FLAT_TICKER_INTERVAL", "10ms")

	d := flatest.Start(flat.App[State]{
		State:  &State{},
		Init:   Init,
		Handle: Handle,
		View:   View,
	}, 72)

	frames := flatest.Frames(d,
		func(d *flatest.Driver[State]) {},                                   // ticks: 0
		func(d *flatest.Driver[State]) { d.Advance(10 * time.Millisecond) }, // ticks: 1
		func(d *flatest.Driver[State]) {
			d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'p'})
			d.Advance(30 * time.Millisecond)
		}, // paused: still 1
		func(d *flatest.Driver[State]) {
			d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'p'})
			d.Advance(20 * time.Millisecond)
		}, // resumed: 3
	)

	flatest.AssertFrames(t, "testdata/ticks-sequence.golden", frames)
}
