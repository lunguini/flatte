package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func ready() *State {
	s := NewState()
	settle(s, 72, 24)
	return s
}

func key(r rune) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}
}

func TestNavigationSelectsRows(t *testing.T) {
	s := ready()
	for range 3 {
		Handle(s, key('j'), flatte.Effects[State]{})
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
	frame := composed(View(s, flatte.RenderContext{Width: 72})).Content
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
	Handle(s, key('G'), flatte.Effects[State]{})
	if s.tb.Cursor() != 19 {
		t.Fatalf("G: Cursor() = %d, want 19", s.tb.Cursor())
	}
	var quit bool
	fx := flatte.NewHarnessEffects[State](context.Background(), nil, func() { quit = true }, nil, nil)
	Handle(s, key('q'), fx)
	if !quit {
		t.Fatal("q did not quit")
	}
}

func TestScrollSequenceSnapshot(t *testing.T) {
	d := flatest.Start(flatte.App[State]{
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

// settle composes the view at width×height and delivers the solved rects as
// the runtime does before painting.
func settle(s *State, width, height int) {
	_, comp := flatte.ComposeFrame(composed(View(s, flatte.RenderContext{Width: width})), width, height)
	Handle(s, flatte.LayoutEvent{Rects: comp.Rects}, flatte.Effects[State]{})
}

func TestClickSelectsTheRowUnderThePointer(t *testing.T) {
	d := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 72)
	// Border, title, subtitle, blank, header, then rows 0, 1, 2: row 7.
	d.Send(flatte.MouseEvent{X: 10, Y: 7, Button: flatte.MouseLeft, Action: flatte.MousePress})
	if got := d.State().tb.Cursor(); got != 2 {
		t.Fatalf("click at row 7 selected %d, want 2", got)
	}
}

// composed is View's frame as the runtime paints it at 72×24.
func composed(f flatte.Frame) flatte.Frame {
	f, _ = flatte.ComposeFrame(f, 72, 24)
	return f
}
