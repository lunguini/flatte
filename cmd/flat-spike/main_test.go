package main

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
)

func TestVimKeysMoveCursor(t *testing.T) {
	state := State{models: []string{"a", "b", "c"}}

	Handle(&state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'j'}, flat.Effects[State]{})
	if state.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 after j", state.cursor)
	}
	Handle(&state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'k'}, flat.Effects[State]{})
	if state.cursor != 0 {
		t.Fatalf("cursor = %d, want 0 after k", state.cursor)
	}
}

func TestHandleMovesCursorWithinBounds(t *testing.T) {
	state := State{models: []string{"haiku", "sonnet", "opus"}}

	Handle(&state, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})
	Handle(&state, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})
	Handle(&state, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})
	if state.cursor != 2 {
		t.Fatalf("cursor = %d, want 2", state.cursor)
	}

	Handle(&state, flat.KeyEvent{Key: flat.KeyUp}, flat.Effects[State]{})
	Handle(&state, flat.KeyEvent{Key: flat.KeyUp}, flat.Effects[State]{})
	Handle(&state, flat.KeyEvent{Key: flat.KeyUp}, flat.Effects[State]{})
	if state.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", state.cursor)
	}
}

func TestHandleEnterSelectsCursorModel(t *testing.T) {
	state := State{models: []string{"haiku", "sonnet", "opus"}, cursor: 1}

	Handle(&state, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

	if state.selectedModel != "sonnet" {
		t.Fatalf("selectedModel = %q, want %q", state.selectedModel, "sonnet")
	}
}

func TestQQuits(t *testing.T) {
	state := State{models: []string{"haiku"}}
	var quit bool
	fx := flat.NewEffects[State](context.Background(), nil, func() { quit = true })

	Handle(&state, flat.KeyEvent{Key: flat.KeyCharacter, Rune: 'q'}, fx)

	if !quit {
		t.Fatal("q should request quit")
	}
}

func TestClickSelectsModelOnItsRow(t *testing.T) {
	state := State{models: []string{"haiku", "sonnet", "opus", "freeform"}}

	// Card top border (row 1) + 3 lines (title, subtitle, blank) precede
	// the list, so model index 2 ("opus") renders on absolute row 6.
	Handle(&state, flat.MouseEvent{
		X: 5, Y: 6, Button: flat.MouseLeft, Action: flat.MousePress,
	}, flat.Effects[State]{})

	if state.cursor != 2 || state.selectedModel != "opus" {
		t.Fatalf("cursor=%d selected=%q, want 2/opus after clicking opus's row", state.cursor, state.selectedModel)
	}
}

func TestClickOutsideListIsIgnored(t *testing.T) {
	state := State{models: []string{"haiku", "sonnet"}, cursor: 1, selectedModel: "sonnet"}

	// Row 2 is the subtitle line, above the list — no model there.
	Handle(&state, flat.MouseEvent{
		X: 5, Y: 2, Button: flat.MouseLeft, Action: flat.MousePress,
	}, flat.Effects[State]{})
	// A row past the last model.
	Handle(&state, flat.MouseEvent{
		X: 5, Y: 20, Button: flat.MouseLeft, Action: flat.MousePress,
	}, flat.Effects[State]{})

	if state.cursor != 1 || state.selectedModel != "sonnet" {
		t.Fatalf("cursor=%d selected=%q, want unchanged 1/sonnet", state.cursor, state.selectedModel)
	}
}

func TestClickReleaseDoesNotSelect(t *testing.T) {
	state := State{models: []string{"haiku", "sonnet", "opus"}}

	Handle(&state, flat.MouseEvent{
		X: 5, Y: 5, Button: flat.MouseLeft, Action: flat.MouseRelease,
	}, flat.Effects[State]{})

	if state.selectedModel != "" {
		t.Fatalf("selectedModel = %q, want empty (release must not select)", state.selectedModel)
	}
}

func TestWheelMovesCursor(t *testing.T) {
	state := State{models: []string{"haiku", "sonnet", "opus"}}

	Handle(&state, flat.MouseEvent{Button: flat.MouseWheelDown, Action: flat.MousePress}, flat.Effects[State]{})
	Handle(&state, flat.MouseEvent{Button: flat.MouseWheelDown, Action: flat.MousePress}, flat.Effects[State]{})
	if state.cursor != 2 {
		t.Fatalf("cursor = %d, want 2 after two wheel-downs", state.cursor)
	}
	Handle(&state, flat.MouseEvent{Button: flat.MouseWheelUp, Action: flat.MousePress}, flat.Effects[State]{})
	if state.cursor != 1 {
		t.Fatalf("cursor = %d, want 1 after wheel-up", state.cursor)
	}
}

func TestViewRendersCurrentStateDeterministically(t *testing.T) {
	state := State{
		models:        []string{"haiku", "sonnet", "opus"},
		cursor:        1,
		selectedModel: "sonnet",
	}

	ctx := flat.RenderContext{Width: 72}
	first := View(&state, ctx).Content
	second := View(&state, ctx).Content
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
		for _, line := range strings.Split(View(&state, flat.RenderContext{Width: frameWidth}).Content, "\n") {
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

	frame := View(&state, flat.RenderContext{Width: 96}).Content
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

	flatest.AssertGolden(t, "testdata/loading.golden", View(&state, flat.RenderContext{Width: 72}).Content)
}

func TestViewMatchesLoadedSnapshot(t *testing.T) {
	state := State{
		models:        []string{"haiku", "sonnet", "opus", "freeform"},
		cursor:        2,
		selectedModel: "opus",
	}

	flatest.AssertGolden(t, "testdata/loaded.golden", View(&state, flat.RenderContext{Width: 72}).Content)
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
		done <- flat.Run(t.Context(), flat.App[State]{
			State:  state,
			Init:   loadModels,
			Handle: Handle,
			View:   View,
		}, flat.WithInput(reader), flat.WithOutput(&out))
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
