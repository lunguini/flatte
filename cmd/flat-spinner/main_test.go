package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func driver() *flatest.Driver[State] {
	return flatest.Start(flatte.App[State]{
		State:  NewState(),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}, 72)
}

func TestSpinnerFrameChangesOnTick(t *testing.T) {
	d := driver()
	f0 := d.Frame().Content
	d.Advance(interval)
	f1 := d.Frame().Content
	if f0 == f1 {
		t.Fatalf("spinner frame did not change after one tick:\n%s", f0)
	}
}

func TestViewShowsLabel(t *testing.T) {
	frame := View(NewState(), flatte.RenderContext{Width: 72}).Content
	for _, want := range []string{"Flat Spinner", "working...", "q quit"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("view missing %q:\n%s", want, frame)
		}
	}
}

func TestQuitOnQ(t *testing.T) {
	var quit bool
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(NewState(), flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestSpinnerSequenceSnapshot(t *testing.T) {
	d := driver()
	frames := flatest.Frames(d,
		func(d *flatest.Driver[State]) {},                          // frame 0
		func(d *flatest.Driver[State]) { d.Advance(interval) },     // +1 tick
		func(d *flatest.Driver[State]) { d.Advance(interval) },     // +1 tick
		func(d *flatest.Driver[State]) { d.Advance(2 * interval) }, // +2 ticks
	)
	flatest.AssertFrames(t, "testdata/spinner-sequence.golden", frames)
}
