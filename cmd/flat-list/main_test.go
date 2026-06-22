package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

// ready builds a state sized to a 24-row terminal (list height 17).
func ready() *State {
	s := NewState()
	s.layout(24)
	return s
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
	// Click the top visible row: cardTop(1) + listTopLine(3) = y 4.
	Handle(s, flatte.MouseEvent{Button: flatte.MouseLeft, Action: flatte.MousePress, Y: 4}, flatte.Effects[State]{})
	if s.list.Cursor() != 4 {
		t.Fatalf("click top visible row selected %d, want 4 (offset+0)", s.list.Cursor())
	}
}

func TestGotoEndAndQuit(t *testing.T) {
	s := ready()
	Handle(s, key('G'), flatte.Effects[State]{})
	if s.list.Cursor() != 29 {
		t.Fatalf("G: Cursor() = %d, want 29", s.list.Cursor())
	}
	var quit bool
	fx := flatte.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(s, key('q'), fx)
	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestViewKeepsChromePinned(t *testing.T) {
	s := ready()
	Handle(s, key('G'), flatte.Effects[State]{}) // scroll to bottom
	frame := View(s, flatte.RenderContext{Width: 72}).Content
	for _, want := range []string{"Flat List", "scrollable selectable list", "q quit", "[30/30]"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("view missing pinned %q:\n%s", want, frame)
		}
	}
}

func TestViewInitialSnapshot(t *testing.T) {
	s := NewState()
	s.layout(24)
	flatest.AssertGoldenFrame(t, "testdata/list.golden", View(s, flatte.RenderContext{Width: 72}))
}

func TestScrollSequenceSnapshot(t *testing.T) {
	d := flatest.Start(flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, 72) // ResizeEvent{72,24} -> list height 17

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
