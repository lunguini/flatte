package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatuitest"
)

func TestVimKeysMoveCursor(t *testing.T) {
	state := State{models: []string{"a", "b", "c"}}

	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'j'}, flatcore.Effects[State]{})
	if state.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 after j", state.cursor)
	}
	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'k'}, flatcore.Effects[State]{})
	if state.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after k", state.cursor)
	}
}

func TestHandleMovesCursorWithinBounds(t *testing.T) {
	state := State{models: []string{"haiku", "sonnet", "opus"}}

	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyDown}, flatcore.Effects[State]{})
	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyDown}, flatcore.Effects[State]{})
	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyDown}, flatcore.Effects[State]{})
	if state.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", state.cursor)
	}

	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyUp}, flatcore.Effects[State]{})
	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyUp}, flatcore.Effects[State]{})
	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyUp}, flatcore.Effects[State]{})
	if state.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", state.cursor)
	}
}

func TestHandleEnterSelectsCursorModel(t *testing.T) {
	state := State{models: []string{"haiku", "sonnet", "opus"}, cursor: 1}

	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyEnter}, flatcore.Effects[State]{})

	if state.selectedModel != "sonnet" {
		t.Fatalf("selectedModel = %q, want %q", state.selectedModel, "sonnet")
	}
}

func TestQQuits(t *testing.T) {
	state := State{models: []string{"haiku"}}
	var quit bool
	fx := flatcore.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(&state, flatcore.KeyEvent{Key: flatcore.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("q should request quit")
	}
}

func TestViewRendersCurrentStateDeterministically(t *testing.T) {
	state := State{
		models:        []string{"haiku", "sonnet", "opus"},
		cursor:        1,
		selectedModel: "sonnet",
	}

	ctx := flatcore.RenderContext{Width: 72}
	first := View(&state, ctx)
	second := View(&state, ctx)
	if first != second {
		t.Fatal("View output changed without a state change")
	}

	for _, want := range []string{"Flatte", "> sonnet", "selected: sonnet"} {
		if !strings.Contains(first, want) {
			t.Fatalf("View() missing %q:\n%s", want, first)
		}
	}
}

func TestViewAdaptsRenderedLinesToContextWidth(t *testing.T) {
	state := State{
		models: []string{"haiku", "sonnet", "opus"},
	}

	for _, frameWidth := range []int{40, 72, 96} {
		for _, line := range strings.Split(View(&state, flatcore.RenderContext{Width: frameWidth}), "\n") {
			if width := lipgloss.Width(line); width > frameWidth {
				t.Fatalf("line width = %d, want <= %d:\n%q", width, frameWidth, line)
			}
		}
	}
}

func TestViewUsesCompactContentWidthWhenThereIsRoom(t *testing.T) {
	state := State{
		models: []string{"haiku", "sonnet", "opus"},
	}

	frame := View(&state, flatcore.RenderContext{Width: 96})
	firstLine := strings.Split(frame, "\n")[0]
	width := lipgloss.Width(firstLine)
	if width >= 96 {
		t.Fatalf("frame width = %d, want compact width below terminal width", width)
	}
	if width > 48 {
		t.Fatalf("frame width = %d, want a compact default frame", width)
	}
}

func TestViewMatchesLoadingSnapshot(t *testing.T) {
	state := State{loading: true}

	flatuitest.AssertGolden(t, "testdata/loading.golden", View(&state, flatcore.RenderContext{Width: 72}))
}

func TestViewMatchesLoadedSnapshot(t *testing.T) {
	state := State{
		models:        []string{"haiku", "sonnet", "opus", "freeform"},
		cursor:        2,
		selectedModel: "opus",
	}

	flatuitest.AssertGolden(t, "testdata/loaded.golden", View(&state, flatcore.RenderContext{Width: 72}))
}

func TestRunAppliesStartupAsyncUpdateBeforeLaterInput(t *testing.T) {
	t.Setenv("FLAT_SPIKE_LOAD_DELAY", "50ms")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()

	state := NewState()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- flatcore.Run(t.Context(), flatcore.App[State]{
			State:  state,
			Init:   loadModels,
			Handle: Handle,
			View:   View,
		}, flatcore.WithInput(reader), flatcore.WithOutput(&out))
	}()

	time.Sleep(350 * time.Millisecond)
	if _, err := writer.Write([]byte("q")); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for Run to exit")
	}

	if state.loading {
		t.Fatal("state is still loading after async update should have applied")
	}
	if got := strings.Join(state.models, ","); got != "haiku,sonnet,opus,freeform" {
		t.Fatalf("models = %q, want loaded models", got)
	}
}

func TestLoadDelayDefaultsToShortSpikeDelay(t *testing.T) {
	t.Setenv("FLAT_SPIKE_LOAD_DELAY", "")

	if got := loadDelay(); got != 250*time.Millisecond {
		t.Fatalf("loadDelay() = %s, want 250ms", got)
	}
}

func TestLoadDelayUsesEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_SPIKE_LOAD_DELAY", "2s")

	if got := loadDelay(); got != 2*time.Second {
		t.Fatalf("loadDelay() = %s, want 2s", got)
	}
}

func TestLoadDelayIgnoresInvalidEnvironmentOverride(t *testing.T) {
	t.Setenv("FLAT_SPIKE_LOAD_DELAY", "slow")

	if got := loadDelay(); got != 250*time.Millisecond {
		t.Fatalf("loadDelay() = %s, want 250ms", got)
	}
}
