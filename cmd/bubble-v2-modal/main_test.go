package main

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestEnterOpensModalAndStartsWaiting(t *testing.T) {
	model := NewModel()

	next, _ := model.Update(key(tea.KeyEnter, ""))
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
	model = updateModel(t, model, key(tea.KeyEnter, ""))
	model = updateModel(t, model, key('A', "A"))
	model = updateModel(t, model, key('d', "d"))
	model = updateModel(t, model, key('a', "a"))

	if model.modalInput.Value != "Ada" {
		t.Fatalf("modal input = %q, want Ada", model.modalInput.Value)
	}

	model = updateModel(t, model, key(tea.KeyEnter, ""))

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

func TestBackgroundTickContinuesWhileModalIsOpen(t *testing.T) {
	model := NewModel()
	model = updateModel(t, model, key(tea.KeyEnter, ""))

	next, cmd := model.Update(tickMsg{})
	model = next.(Model)
	if cmd == nil {
		t.Fatal("expected tick update to return next tick command")
	}

	if model.ticks != 1 {
		t.Fatalf("ticks = %d, want 1", model.ticks)
	}
	if model.spinner != 1 {
		t.Fatalf("spinner = %d, want 1", model.spinner)
	}
	if !model.modalOpen {
		t.Fatal("modal should remain open after background tick")
	}
}

func TestClipboardCommandsAndMessagesAreExplicit(t *testing.T) {
	model := NewModel()
	model.modalResult = "accepted: Ada"

	next, cmd := model.Update(key('c', "c"))
	model = next.(Model)

	if cmd == nil {
		t.Fatal("expected c to return a clipboard write command")
	}
	if model.clipboardStatus != "copying: accepted: Ada" {
		t.Fatalf("clipboardStatus = %q, want copying accepted result", model.clipboardStatus)
	}

	model = updateModel(t, model, tea.ClipboardMsg{Content: "from clipboard"})
	if model.clipboardStatus != "clipboard: from clipboard" {
		t.Fatalf("clipboardStatus = %q, want clipboard content", model.clipboardStatus)
	}

	next, cmd = model.Update(key('p', "p"))
	model = next.(Model)
	if cmd == nil {
		t.Fatal("expected p to return a clipboard read command")
	}
	if model.clipboardStatus != "reading clipboard" {
		t.Fatalf("clipboardStatus = %q, want reading clipboard", model.clipboardStatus)
	}
}

func TestViewUsesBubbleTeaV2ViewMetadata(t *testing.T) {
	model := NewModel()
	model.ticks = 4
	model.spinner = 1
	model.waiting = true
	model.modalOpen = true
	model.modalInput.Value = "Ada"
	model.modalInput.Cursor = 1
	model.clipboardStatus = "clipboard: Ada"

	view := model.View()

	if !view.AltScreen {
		t.Fatal("expected v2 view to request alt screen")
	}
	if view.MouseMode != tea.MouseModeCellMotion {
		t.Fatalf("MouseMode = %v, want cell motion", view.MouseMode)
	}
	for _, want := range []string{"Bubble v2 Modal", "background ticks: 4", "waiting \\", "Confirm Work", "A▌da", "clipboard: Ada"} {
		if !strings.Contains(view.Content, want) {
			t.Fatalf("View() missing %q:\n%s", want, view.Content)
		}
	}
}

func TestTickIntervalEnvironmentOverride(t *testing.T) {
	t.Setenv("BUBBLE_V2_MODAL_INTERVAL", "25ms")

	if got := tickInterval(); got != 25*time.Millisecond {
		t.Fatalf("tickInterval() = %s, want 25ms", got)
	}
}

func key(code rune, text string) tea.KeyPressMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Text: text})
}

func updateModel(t *testing.T, model Model, msg tea.Msg) Model {
	t.Helper()

	next, _ := model.Update(msg)
	return next.(Model)
}
