package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func driver() *flatest.Driver[State] {
	return flatest.Start(flat.App[State]{
		State:  NewState(),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}, 72)
}

func TestTimerAndStopwatchAdvanceFromFakeClock(t *testing.T) {
	d := driver()

	d.Advance(2 * tickInterval)
	if got := d.State().timer.Remaining(); got != 8*time.Second {
		t.Fatalf("timer remaining = %s, want 8s", got)
	}
	if got := d.State().stopwatch.Elapsed(); got != 0 {
		t.Fatalf("stopped stopwatch elapsed = %s, want 0", got)
	}

	d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: ' '})
	d.Advance(3 * tickInterval)
	if got := d.State().timer.Remaining(); got != 5*time.Second {
		t.Fatalf("timer remaining after more ticks = %s, want 5s", got)
	}
	if got := d.State().stopwatch.Elapsed(); got != 3*time.Second {
		t.Fatalf("running stopwatch elapsed = %s, want 3s", got)
	}
}

func TestTimerControlsResetAndRestart(t *testing.T) {
	d := driver()
	d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: ' '})
	d.Advance(4 * tickInterval)

	d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: 't'})
	if got := d.State().timer.Remaining(); got != countdownLength {
		t.Fatalf("timer remaining after restart = %s, want %s", got, countdownLength)
	}
	if got := d.State().stopwatch.Elapsed(); got != 4*time.Second {
		t.Fatalf("stopwatch after timer restart = %s, want 4s", got)
	}

	d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'r'})
	if got := d.State().timer.Remaining(); got != countdownLength {
		t.Fatalf("timer remaining after reset = %s, want %s", got, countdownLength)
	}
	if got := d.State().stopwatch.Elapsed(); got != 0 {
		t.Fatalf("stopwatch after reset = %s, want 0", got)
	}
	if d.State().stopwatch.Running() {
		t.Fatal("stopwatch running after reset")
	}
}

func TestTimerViewShowsState(t *testing.T) {
	frame := View(NewState(), flat.RenderContext{Width: 72}).Content
	for _, want := range []string{"Flat Timer", "timer:", "stopwatch:", "space start/stop"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("view missing %q:\n%s", want, frame)
		}
	}
}

func TestTimerQuit(t *testing.T) {
	for _, key := range []flat.KeyEvent{
		{Key: flat.KeyCharacter, Rune: 'q'},
		{Key: flat.KeyEscape},
	} {
		var quit bool
		fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })
		Handle(NewState(), key, fx)
		if !quit {
			t.Fatalf("%+v did not request quit", key)
		}
	}
}

func TestTimerSnapshot(t *testing.T) {
	d := driver()
	flatest.AssertGoldenFrame(t, "testdata/timer.golden", d.Frame())
}
