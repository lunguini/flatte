package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

// ready builds a state laid out on a 72x24 terminal (list height 17).
func ready() *State {
	s := NewState()
	settle(s, 72, 24)
	return s
}

// settle composes the view at width×height and delivers the solved rects as
// the runtime does before painting.
func settle(s *State, width, height int) {
	_, comp := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: width}), width, height)
	Handle(s, flatte.LayoutEvent{Rects: comp.Rects}, flatte.Effects[State]{})
}

// composed is View's frame as the runtime paints it at 72×24.
func composed(f flatte.Frame) flatte.Frame {
	f, _ = flatte.ComposeFrame(f, 72, 24)
	return f
}

func key(r rune) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r}
}

func TestNavigationScrollsToKeepCursorVisible(t *testing.T) {
	s := ready() // height 17
	for range 20 {
		Handle(s, key('j'), flatte.Effects[State]{})
	}
	if s.list.Cursor() != 20 {
		t.Fatalf("Cursor() = %d, want 20", s.list.Cursor())
	}
	if s.list.Offset() != 4 { // 20 - 17 + 1
		t.Fatalf("Offset() = %d, want 4 (kept cursor visible)", s.list.Offset())
	}
}

func TestEnterChoosesCurrentItem(t *testing.T) {
	s := ready()
	Handle(s, key('j'), flatte.Effects[State]{})
	Handle(s, key('j'), flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyEnter}, flatte.Effects[State]{})
	if s.chosen != 2 {
		t.Fatalf("chosen = %d, want 2", s.chosen)
	}
}

func TestClickSelectsRowAccountingForScroll(t *testing.T) {
	s := ready()
	s.list.Select(20) // scroll down; offset 4 (20-17+1)
	if s.list.Offset() != 4 {
		t.Fatalf("setup Offset() = %d, want 4", s.list.Offset())
	}
	// The top visible row is item 4's node.
	Handle(s, flatte.MouseEvent{Button: flatte.MouseLeft, Action: flatte.MousePress, Target: "items:4"}, flatte.Effects[State]{})
	if s.list.Cursor() != 4 {
		t.Fatalf("click top visible row selected %d, want 4 (offset+0)", s.list.Cursor())
	}
}

func TestClickHitsTheRowUnderThePointer(t *testing.T) {
	d := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 72)
	// Row 6 of the card: border, title, subtitle, blank, then items 0, 1, 2.
	d.Send(flatte.MouseEvent{X: 10, Y: 6, Button: flatte.MouseLeft, Action: flatte.MousePress})
	if got := d.State().list.Cursor(); got != 2 {
		t.Fatalf("click at row 6 selected %d, want 2", got)
	}
	// Outside the list: no change.
	d.Send(flatte.MouseEvent{X: 10, Y: 1, Button: flatte.MouseLeft, Action: flatte.MousePress})
	if got := d.State().list.Cursor(); got != 2 {
		t.Fatalf("click on the title moved the selection to %d", got)
	}
}

func TestGotoEndAndQuit(t *testing.T) {
	s := ready()
	Handle(s, key('G'), flatte.Effects[State]{})
	if s.list.Cursor() != 29 {
		t.Fatalf("G: Cursor() = %d, want 29", s.list.Cursor())
	}
	var quit bool
	fx := flatte.NewHarnessEffects[State](context.Background(), nil, func() { quit = true }, nil, nil)
	Handle(s, key('q'), fx)
	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestViewKeepsChromePinned(t *testing.T) {
	s := ready()
	Handle(s, key('G'), flatte.Effects[State]{}) // scroll to bottom
	frame := composed(View(s, flatte.RenderContext{Width: 72})).Content
	for _, want := range []string{"Flat List", "scrollable selectable list", "q quit", "[30/30]"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("view missing pinned %q:\n%s", want, frame)
		}
	}
}

func TestViewInitialSnapshot(t *testing.T) {
	s := ready()
	flatest.AssertGoldenFrame(t, "testdata/list.golden", composed(View(s, flatte.RenderContext{Width: 72})))
}

func TestScrollSequenceSnapshot(t *testing.T) {
	d := flatest.Start(flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, 72) // laid out at 72x24 -> list height 17

	frames := flatest.Frames(d,
		func(d *flatest.Driver[State]) {}, // top: items 1-17
		func(d *flatest.Driver[State]) {
			for range 20 {
				d.Send(key('j'))
			}
		}, // mid-scroll: offset 4
		func(d *flatest.Driver[State]) {
			d.Send(key('G'))
		}, // bottom: offset 13, [30/30]
	)
	flatest.AssertFrames(t, "testdata/list-sequence.golden", frames)
}
