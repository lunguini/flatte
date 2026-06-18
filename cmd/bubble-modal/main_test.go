package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lunguini/flat/flatest"
)

func TestEnterOpensModalAndStartsWaiting(t *testing.T) {
	model := NewModel()

	next, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = next.(Model)

	if !model.modalOpen {
		t.Fatal("expected modal to open")
	}
	if !model.waiting {
		t.Fatal("expected background to wait for modal result")
	}
}

func TestModalCapturesInputAndConfirmCompletesWaiting(t *testing.T) {
	model := NewModel()
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})

	if model.modalInput.Value != "Ada" {
		t.Fatalf("modal input = %q, want Ada", model.modalInput.Value)
	}

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	if model.modalOpen {
		t.Fatal("expected modal to close after confirm")
	}
	if model.waiting {
		t.Fatal("expected waiting to stop after confirm")
	}
	if model.modalResult != "accepted: Ada" {
		t.Fatalf("modalResult = %q, want accepted: Ada", model.modalResult)
	}
}

func TestModalCapturesQInsteadOfQuitting(t *testing.T) {
	model := NewModel()
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if model.quit {
		t.Fatal("q should not quit while modal is open")
	}
	if model.modalInput.Value != "q" {
		t.Fatalf("modal input = %q, want q", model.modalInput.Value)
	}
}

func TestEscapeCancelsModal(t *testing.T) {
	model := NewModel()
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEsc})

	if model.modalOpen {
		t.Fatal("expected modal to close after escape")
	}
	if model.waiting {
		t.Fatal("expected waiting to stop after cancel")
	}
	if model.modalResult != "cancelled" {
		t.Fatalf("modalResult = %q, want cancelled", model.modalResult)
	}
}

func TestBackgroundTickContinuesWhileModalIsOpen(t *testing.T) {
	model := NewModel()
	model = updateModel(t, model, tea.KeyMsg{Type: tea.KeyEnter})

	next, cmd := model.Update(tickMsg{})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("expected tick update to return next tick command")
	}
	next, _ = model.Update(tickMsg{})
	model = next.(Model)

	if model.ticks != 2 {
		t.Fatalf("ticks = %d, want 2", model.ticks)
	}
	if model.spinner != 2 {
		t.Fatalf("spinner = %d, want 2", model.spinner)
	}
	if !model.modalOpen {
		t.Fatal("modal should remain open after background ticks")
	}
}

func TestQQuitsOnlyWhenModalIsClosed(t *testing.T) {
	model := NewModel()

	next, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	model = next.(Model)

	if !model.quit {
		t.Fatal("q should quit when modal is closed")
	}
	if cmd == nil {
		t.Fatal("expected q to return tea.Quit command")
	}
}

func TestInitReturnsTickCommand(t *testing.T) {
	model := NewModel()

	if cmd := model.Init(); cmd == nil {
		t.Fatal("expected Init to return tick command")
	}
}

func TestViewRendersMainAndModalState(t *testing.T) {
	model := NewModel()
	model.ticks = 4
	model.spinner = 1
	model.waiting = true
	model.modalOpen = true
	model.modalInput.Value = "Ada"
	model.modalInput.Cursor = 1

	frame := model.View()

	for _, want := range []string{"Bubble Modal", "background ticks: 4", "waiting \\", "Confirm Work", "A▌da"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("View() missing %q:\n%s", want, frame)
		}
	}
}

func TestViewMatchesModalSnapshot(t *testing.T) {
	model := NewModel()
	model.ticks = 4
	model.spinner = 1
	model.waiting = true
	model.modalOpen = true
	model.modalInput.Value = "Ada"
	model.modalInput.Cursor = 1

	flatest.AssertGolden(t, "testdata/modal-open.golden", model.View())
}

func TestTickIntervalEnvironmentOverride(t *testing.T) {
	t.Setenv("BUBBLE_MODAL_INTERVAL", "25ms")

	if got := tickInterval(); got != 25*time.Millisecond {
		t.Fatalf("tickInterval() = %s, want 25ms", got)
	}
}

func updateModel(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()

	next, _ := model.Update(msg)
	return next.(Model)
}
