package main

import (
	"context"
	"strings"
	"testing"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

// ready builds a state already sized to an 80x24 terminal.
func ready() *State {
	s := NewState()
	s.layout(80, 24)
	return s
}

func TestHandleScrollKeysMoveTheViewport(t *testing.T) {
	s := ready()
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'j'}, flat.Effects[State]{})
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'j'}, flat.Effects[State]{})
	if s.vp.Offset() != 2 {
		t.Fatalf("after jj Offset() = %d, want 2", s.vp.Offset())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'k'}, flat.Effects[State]{})
	if s.vp.Offset() != 1 {
		t.Fatalf("after k Offset() = %d, want 1", s.vp.Offset())
	}
}

func TestHandleGotoBottomAndQuit(t *testing.T) {
	s := ready()
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'G'}, flat.Effects[State]{})
	if !s.vp.AtBottom() {
		t.Fatal("G did not reach bottom")
	}
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)
	if !quit {
		t.Fatal("q did not request quit")
	}
}

func TestResizeShrinksViewportInsteadOfBreaking(t *testing.T) {
	s := ready()
	tall := s.vp.VisibleLines()
	s.layout(80, 10) // shrink height
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
	Handle(s, flat.MouseEvent{Button: flat.MouseWheelDown}, flat.Effects[State]{})
	if s.vp.Offset() != 3 {
		t.Fatalf("wheel down Offset() = %d, want 3 (wheelLines)", s.vp.Offset())
	}
	Handle(s, flat.MouseEvent{Button: flat.MouseWheelUp}, flat.Effects[State]{})
	if s.vp.Offset() != 0 {
		t.Fatalf("wheel up Offset() = %d, want 0", s.vp.Offset())
	}
}

func TestViewKeepsChromePinnedWhileScrolling(t *testing.T) {
	s := ready()
	Handle(s, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'G'}, flat.Effects[State]{})
	frame := View(s, flat.RenderContext{Width: 80}).Content
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
	s.layout(72, 24)
	flatest.AssertGoldenFrame(t, "testdata/reader.golden", View(s, flat.RenderContext{Width: 72}))
}

func TestScrollThenShrinkSequenceSnapshot(t *testing.T) {
	d := flatest.Start(flat.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, 72) // Start delivers ResizeEvent{72,24}; layout sizes the viewport

	frames := flatest.Frames(d,
		func(d *flatest.Driver[State]) {}, // initial frame at 72x24, offset 0
		func(d *flatest.Driver[State]) {
			for range 5 {
				d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'j'})
			}
		}, // scrolled down 5
		func(d *flatest.Driver[State]) {
			d.Send(flat.ResizeEvent{Width: 72, Height: 10})
		}, // shrink: body clips to fewer rows, chrome still pinned
	)
	flatest.AssertFrames(t, "testdata/reader-sequence.golden", frames)
}
