package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

// ready builds a state already laid out on an 80x24 terminal.
func ready() *State {
	s := NewState()
	settle(s, 80, 24)
	return s
}

// settle composes the view at width×height and delivers the solved rects as
// the runtime does before painting.
func settle(s *State, width, height int) {
	_, comp := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: width}), width, height)
	Handle(s, flatte.LayoutEvent{Rects: comp.Rects}, flatte.Effects[State]{})
}

func TestHandleScrollKeysMoveTheViewport(t *testing.T) {
	s := ready()
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'}, flatte.Effects[State]{})
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'}, flatte.Effects[State]{})
	if s.vp.Offset() != 2 {
		t.Fatalf("after jj Offset() = %d, want 2", s.vp.Offset())
	}
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'k'}, flatte.Effects[State]{})
	if s.vp.Offset() != 1 {
		t.Fatalf("after k Offset() = %d, want 1", s.vp.Offset())
	}
}

func TestHandleGotoBottomAndQuit(t *testing.T) {
	s := ready()
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'G'}, flatte.Effects[State]{})
	if !s.vp.AtBottom() {
		t.Fatal("G did not reach bottom")
	}
	var quit bool
	fx := flatte.NewHarnessEffects[State](context.Background(), nil, func() { quit = true }, nil, nil)
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestResizeShrinksViewportInsteadOfBreaking(t *testing.T) {
	s := ready()
	tall := s.vp.VisibleLines()
	settle(s, 80, 10) // shrink height
	short := s.vp.VisibleLines()
	if short >= tall {
		t.Fatalf("VisibleLines did not shrink: %d -> %d", tall, short)
	}
	if short < 1 {
		t.Fatalf("VisibleLines collapsed to %d", short)
	}
}

func TestMouseWheelScrollsTheViewport(t *testing.T) {
	s := ready()
	Handle(s, flatte.MouseEvent{Button: flatte.MouseWheelDown}, flatte.Effects[State]{})
	if s.vp.Offset() != 3 {
		t.Fatalf("wheel down Offset() = %d, want 3 (wheelLines)", s.vp.Offset())
	}
	Handle(s, flatte.MouseEvent{Button: flatte.MouseWheelUp}, flatte.Effects[State]{})
	if s.vp.Offset() != 0 {
		t.Fatalf("wheel up Offset() = %d, want 0", s.vp.Offset())
	}
}

func TestViewKeepsChromePinnedWhileScrolling(t *testing.T) {
	s := ready()
	Handle(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'G'}, flatte.Effects[State]{})
	frame := composed(View(s, flatte.RenderContext{Width: 80})).Content
	for _, want := range []string{"Flat Reader", "scrollable viewport sample", "q quit"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("scrolled view missing pinned %q:\n%s", want, frame)
		}
	}
	if !strings.Contains(frame, "100%") {
		t.Fatalf("footer percent not at bottom:\n%s", frame)
	}
}

func TestViewMatchesInitialSnapshot(t *testing.T) {
	s := NewState()
	settle(s, 72, 24)
	flatest.AssertGoldenFrame(t, "testdata/reader.golden", composed(View(s, flatte.RenderContext{Width: 72})))
}

func TestScrollThenShrinkSequenceSnapshot(t *testing.T) {
	d := flatest.Start(flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, 72) // Start lays out at 72x24; the LayoutEvent sizes the viewport

	frames := flatest.Frames(d,
		func(d *flatest.Driver[State]) {}, // initial frame at 72x24, offset 0
		func(d *flatest.Driver[State]) {
			for range 5 {
				d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'})
			}
		}, // scrolled down 5
		func(d *flatest.Driver[State]) {
			d.Send(flatte.ResizeEvent{Width: 72, Height: 10})
		}, // shrink: body clips to fewer rows, chrome still pinned
	)
	flatest.AssertFrames(t, "testdata/reader-sequence.golden", frames)
}

// composed is View's frame as the runtime paints it at 72×24.
func composed(f flatte.Frame) flatte.Frame {
	f, _ = flatte.ComposeFrame(f, 72, 24)
	return f
}
