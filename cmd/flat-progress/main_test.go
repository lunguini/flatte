package main

import (
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
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

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: ' '}, flatte.Effects[State]{})
	if !state.paused {
		t.Fatal("space should pause")
	}

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'r'}, flatte.Effects[State]{})
	if got := state.progress.Percent(); got != 0 {
		t.Fatalf("percent after reset = %.1f, want 0", got)
	}

	var quit bool
	fx := flatte.NewHarnessEffects[State](t.Context(), nil, func() { quit = true }, nil, nil)
	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q should request quit")
	}
}

func TestLayoutSizesProgressToItsRect(t *testing.T) {
	state := NewState()

	Handle(state, flatte.LayoutEvent{Rects: map[string]layout.Rect{barID: {X: 3, Y: 4, W: 40, H: 1}}}, flatte.Effects[State]{})

	if got := state.progress.Width(); got != 40-percentLabelWidth {
		t.Fatalf("progress width = %d, want %d", got, 40-percentLabelWidth)
	}
}

func TestBarFillsTheCardAt72Columns(t *testing.T) {
	d := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 72)
	// 72 columns less the card's border and padding (6) and the indent (2),
	// less the percent label: the same bar as before the frame tree.
	if got := d.State().progress.Width(); got != 72-6-2-percentLabelWidth {
		t.Fatalf("progress width = %d, want %d", got, 72-6-2-percentLabelWidth)
	}
}

func TestViewRendersProgressState(t *testing.T) {
	state := NewState()
	state.progress.SetPercent(30)

	frame := composed(View(state, flatte.RenderContext{Width: 72})).Content

	for _, want := range []string{"Flat Progress", "30%", "space pause", "r reset"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestProgressSequenceSnapshot(t *testing.T) {
	t.Setenv("FLAT_PROGRESS_INTERVAL", "10ms")

	d := flatest.Start(flatte.App[State]{
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
			d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: ' '})
			d.Advance(30 * time.Millisecond)
		}, // paused: still 40%
		func(d *flatest.Driver[State]) {
			d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'r'})
		}, // reset: 0%
	)

	flatest.AssertFrames(t, "testdata/progress-sequence.golden", frames)
}

// composed is View's frame as the runtime paints it at 72×24.
func composed(f flatte.Frame) flatte.Frame {
	f, _ = flatte.ComposeFrame(f, 72, 24)
	return f
}
