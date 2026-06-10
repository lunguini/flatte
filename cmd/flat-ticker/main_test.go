package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatuitest"
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

	Handle(&state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'p'}, flatcore.Effects[State]{})
	if !state.paused {
		t.Fatal("expected paused state after p")
	}

	Handle(&state, flatcore.Event{Key: flatcore.KeyCharacter, Rune: 'r'}, flatcore.Effects[State]{})
	if state.ticks != 0 {
		t.Fatalf("ticks = %d, want reset to 0", state.ticks)
	}
}

func TestViewRendersTickerState(t *testing.T) {
	state := State{ticks: 3, paused: true}

	frame := View(&state, flatcore.RenderContext{Width: 72})

	for _, want := range []string{"Flat Ticker", "ticks: 3", "state: paused"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewMatchesPausedSnapshot(t *testing.T) {
	state := State{ticks: 3, paused: true}

	flatuitest.AssertGolden(t, "testdata/paused.golden", View(&state, flatcore.RenderContext{Width: 72}))
}

func TestTickIntervalEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_TICKER_INTERVAL", "25ms")

	if got := tickInterval(); got != 25*time.Millisecond {
		t.Fatalf("tickInterval() = %s, want 25ms", got)
	}
}
