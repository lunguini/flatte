package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func TestStyleViewUsesStyledLipglossComposition(t *testing.T) {
	state := NewState()
	state.layout(80, 24)

	frame := View(state, flatte.RenderContext{Width: 80}).Content

	if !strings.Contains(frame, "\x1b[") {
		t.Fatalf("View() has no ANSI styling:\n%s", frame)
	}
	clean := flatest.CleanFrame(frame)
	for _, want := range []string{"Flat Style", "Palette", "Delivery", "deploy-api", "70%"} {
		if !strings.Contains(clean, want) {
			t.Fatalf("View() missing %q:\n%s", want, clean)
		}
	}
}

func TestStyleViewFitsRequestedWidth(t *testing.T) {
	state := NewState()
	state.layout(64, 18)

	frame := flatest.CleanFrame(View(state, flatte.RenderContext{Width: 64}).Content)

	for _, line := range strings.Split(frame, "\n") {
		if width := lipgloss.Width(line); width > 64 {
			t.Fatalf("line width = %d > 64:\n%s", width, frame)
		}
	}
}

func TestStyleHandleMovesSelectionAndProgress(t *testing.T) {
	state := NewState()

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'j'}, flatte.Effects[State]{})
	if got, want := state.list.Cursor(), 1; got != want {
		t.Fatalf("cursor after j = %d, want %d", got, want)
	}

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'l'}, flatte.Effects[State]{})
	if got, want := state.progress.Percent(), 80.0; got != want {
		t.Fatalf("percent after l = %.1f, want %.1f", got, want)
	}

	Handle(state, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'h'}, flatte.Effects[State]{})
	if got, want := state.progress.Percent(), 70.0; got != want {
		t.Fatalf("percent after h = %.1f, want %.1f", got, want)
	}
}

func TestStyleViewMatchesSnapshot(t *testing.T) {
	state := NewState()
	state.layout(80, 24)

	flatest.AssertGolden(t, "testdata/style.golden", View(state, flatte.RenderContext{Width: 80}).Content)
}
