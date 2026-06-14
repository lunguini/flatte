package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatest"
)

func ready() *State {
	s := NewState()
	s.layout(24)
	return s
}

func key(r rune) flatcore.KeyEvent {
	return flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: r}
}

func TestNavigationSelectsRows(t *testing.T) {
	s := ready()
	for range 3 {
		Handle(s, key('j'), flatcore.Effects[State]{})
	}
	if s.tb.Cursor() != 3 {
		t.Fatalf("Cursor() = %d, want 3", s.tb.Cursor())
	}
	if got := s.tb.SelectedRow(); len(got) < 2 || got[1] != "service-04" {
		t.Fatalf("SelectedRow() = %v, want service-04", got)
	}
}

func TestViewHasHeaderAndMarksSelection(t *testing.T) {
	s := ready()
	frame := View(s, flatcore.RenderContext{Width: 72}).Content
	for _, want := range []string{"Flat Table", "ID", "Name", "Status", "service-01", "[1/20]"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("view missing %q:\n%s", want, frame)
		}
	}
	// The selected row (ID 1) carries the "> " marker before its first column.
	if !strings.Contains(frame, "> 1") {
		t.Fatalf("selected-row marker missing:\n%s", frame)
	}
}

func TestGotoEndAndQuit(t *testing.T) {
	s := ready()
	Handle(s, key('G'), flatcore.Effects[State]{})
	if s.tb.Cursor() != 19 {
		t.Fatalf("G: Cursor() = %d, want 19", s.tb.Cursor())
	}
	var quit bool
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(s, key('q'), fx)
	if !quit {
		t.Fatal("q did not quit")
	}
}

func TestScrollSequenceSnapshot(t *testing.T) {
	d := flatest.Start(flatcore.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, 72)
	frames := flatest.Frames(d,
		func(d *flatest.Driver[State]) {},
		func(d *flatest.Driver[State]) {
			for range 12 {
				d.Send(key('j'))
			}
		},
		func(d *flatest.Driver[State]) { d.Send(key('G')) },
	)
	flatest.AssertFrames(t, "testdata/table-sequence.golden", frames)
}
