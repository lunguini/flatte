package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatest"
)

// ready builds a state sized to a 24-row terminal (list height 17).
func ready() *State {
	s := NewState()
	s.layout(24)
	return s
}

func key(r rune) flatcore.KeyEvent {
	return flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: r}
}

func TestNavigationScrollsToKeepCursorVisible(t *testing.T) {
	s := ready() // height 17
	for range 20 {
		Handle(s, key('j'), flatcore.Effects[State]{})
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
	Handle(s, key('j'), flatcore.Effects[State]{})
	Handle(s, key('j'), flatcore.Effects[State]{})
	Handle(s, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})
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
	Handle(s, flatcore.MouseEvent{Button: flatcore.MouseLeft, Action: flatcore.MousePress, Y: 4}, flatcore.Effects[State]{})
	if s.list.Cursor() != 4 {
		t.Fatalf("click top visible row selected %d, want 4 (offset+0)", s.list.Cursor())
	}
}

func TestGotoEndAndQuit(t *testing.T) {
	s := ready()
	Handle(s, key('G'), flatcore.Effects[State]{})
	if s.list.Cursor() != 29 {
		t.Fatalf("G: Cursor() = %d, want 29", s.list.Cursor())
	}
	var quit bool
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(s, key('q'), fx)
	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestViewKeepsChromePinned(t *testing.T) {
	s := ready()
	Handle(s, key('G'), flatcore.Effects[State]{}) // scroll to bottom
	frame := View(s, flatcore.RenderContext{Width: 72}).Content
	for _, want := range []string{"Flat List", "scrollable selectable list", "q quit", "[30/30]"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("view missing pinned %q:\n%s", want, frame)
		}
	}
}

func TestViewInitialSnapshot(t *testing.T) {
	s := NewState()
	s.layout(24)
	flatest.AssertGoldenFrame(t, "testdata/list.golden", View(s, flatcore.RenderContext{Width: 72}))
}

func TestScrollSequenceSnapshot(t *testing.T) {
	d := flatest.Start(flatcore.App[State]{
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
