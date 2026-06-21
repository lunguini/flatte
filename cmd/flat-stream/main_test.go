package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func TestApplyStreamEventAppendsAndCompletes(t *testing.T) {
	state := State{}

	applyStreamEvent(&state, StreamEvent{Level: "info", Message: "queued"})
	applyStreamEvent(&state, StreamEvent{Done: true})

	if len(state.events) != 1 {
		t.Fatalf("events = %d, want 1", len(state.events))
	}
	if state.events[0].Message != "queued" {
		t.Fatalf("message = %q, want queued", state.events[0].Message)
	}
	if !state.done {
		t.Fatal("done = false, want true")
	}
}

func TestStreamSourceStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var got []StreamEvent
	streamSource([]StreamEvent{{Message: "hidden"}}, 0)(ctx, func(ev StreamEvent) {
		got = append(got, ev)
	})

	if len(got) != 0 {
		t.Fatalf("events = %#v, want none after cancel", got)
	}
}

func TestInitStreamsEventsThroughDriverSettle(t *testing.T) {
	t.Setenv("FLAT_STREAM_INTERVAL", "0s")

	d := flatest.Start(flat.App[State]{
		State:  NewState(),
		Init:   Init,
		Handle: Handle,
		View:   View,
	}, 72)

	d.Settle()

	if got := len(d.State().events); got != len(defaultEvents) {
		t.Fatalf("events = %d, want %d", got, len(defaultEvents))
	}
	if !d.State().done {
		t.Fatal("done = false after source completes")
	}
}

func TestHandleClearsAndQuits(t *testing.T) {
	state := State{events: []StreamEvent{{Message: "old"}}, done: true}

	Handle(&state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'c'}, flat.Effects[State]{})
	if len(state.events) != 0 || state.done {
		t.Fatalf("state after clear = %#v, want empty streaming state", state)
	}

	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(&state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestStreamIntervalEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_STREAM_INTERVAL", "25ms")

	if got := streamInterval(); got != 25*time.Millisecond {
		t.Fatalf("streamInterval() = %s, want 25ms", got)
	}
}

func TestViewRendersStreamState(t *testing.T) {
	state := State{
		events: []StreamEvent{{Level: "info", Message: "queued"}},
		done:   true,
	}

	frame := View(&state, flat.RenderContext{Width: 72}).Content

	for _, want := range []string{"Flat Stream", "status: complete", "[info] queued"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewMatchesSnapshot(t *testing.T) {
	state := State{
		events: []StreamEvent{
			{Level: "info", Message: "queued"},
			{Level: "ok", Message: "deployed"},
		},
		done: true,
	}

	flatest.AssertGolden(t, "testdata/stream.golden", View(&state, flat.RenderContext{Width: 72}).Content)
}
