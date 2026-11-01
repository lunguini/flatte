package flatui

import (
	"testing"

	"github.com/lunguini/flatte"
)

func TestTextareaEditMovementSelectionAndTyping(t *testing.T) {
	area := &Textarea{}
	area.SetValue("hello world")
	if !area.Edit(k(flatte.KeyEnd, 0)) {
		t.Fatal("End was not handled")
	}
	if area.Row() != 0 || area.Col() != len("hello world") {
		t.Fatalf("End cursor = %d:%d", area.Row(), area.Col())
	}
	if !area.Edit(k(flatte.KeyLeft, flatte.ModSuper)) {
		t.Fatal("Cmd+Left was not handled")
	}
	if area.Row() != 0 || area.Col() != 0 {
		t.Fatalf("Cmd+Left cursor = %d:%d, want 0:0", area.Row(), area.Col())
	}
	if !area.Edit(k(flatte.KeyRight, flatte.ModAlt)) {
		t.Fatal("Alt+Right was not handled")
	}
	if area.Row() != 0 || area.Col() != len("hello") {
		t.Fatalf("Alt+Right cursor = %d:%d, want 0:%d", area.Row(), area.Col(), len("hello"))
	}
	if !area.Edit(k(flatte.KeyHome, flatte.ModShift)) || area.SelectedText() != "hello" {
		t.Fatalf("Shift+Home selection = %q", area.SelectedText())
	}
	if !area.Edit(ch('H', 0)) || area.Value() != "H world" || area.Col() != 1 {
		t.Fatalf("typing over selection = %q at %d", area.Value(), area.Col())
	}
	if !area.Edit(ch('a', flatte.ModSuper)) || area.SelectedText() != "H world" {
		t.Fatalf("Cmd+A selection = %q", area.SelectedText())
	}
	if !area.Edit(ch('x', 0)) || area.Value() != "x" || area.Col() != 1 {
		t.Fatalf("typing over Cmd+A selection = %q at %d", area.Value(), area.Col())
	}
}

func TestTextareaEditArrowsCollapseSelectionToEdge(t *testing.T) {
	area := &Textarea{}
	area.SetValue("hello")
	area.Edit(k(flatte.KeyRight, flatte.ModShift))
	area.Edit(k(flatte.KeyRight, flatte.ModShift))
	if got := area.SelectedText(); got != "he" {
		t.Fatalf("forward selection = %q, want he", got)
	}
	area.Edit(k(flatte.KeyLeft, 0))
	if area.Col() != 0 {
		t.Fatalf("Left collapsed forward selection to %d, want 0", area.Col())
	}

	area.MoveTo(len("hello"))
	area.Edit(k(flatte.KeyLeft, flatte.ModShift))
	area.Edit(k(flatte.KeyLeft, flatte.ModShift))
	if got := area.SelectedText(); got != "lo" {
		t.Fatalf("backward selection = %q, want lo", got)
	}
	area.Edit(k(flatte.KeyRight, 0))
	if area.Col() != len("hello") {
		t.Fatalf("Right collapsed backward selection to %d, want %d", area.Col(), len("hello"))
	}
}

func TestTextareaEditReadlineKeys(t *testing.T) {
	tests := []struct {
		name   string
		value  string
		cursor int
		key    flatte.KeyEvent
		want   string
		row    int
		col    int
	}{
		{"Ctrl+A", "hello world", len("hello world"), ch('a', flatte.ModCtrl), "hello world", 0, 0},
		{"Ctrl+E", "hello world", 0, ch('e', flatte.ModCtrl), "hello world", 0, len("hello world")},
		{"Ctrl+U", "hello world", len("hello"), ch('u', flatte.ModCtrl), " world", 0, 0},
		{"Ctrl+U at line start", "hello\nnext", 0, ch('u', flatte.ModCtrl), "hello\nnext", 0, 0},
		{"Ctrl+K", "hello world\nnext", 0, ch('k', flatte.ModCtrl), "\nnext", 0, 0},
		{"Ctrl+K at line end", "hello\nnext", len("hello"), ch('k', flatte.ModCtrl), "hello\nnext", 0, len("hello")},
		{"Ctrl+W", "hello big world", len("hello big world"), ch('w', flatte.ModCtrl), "hello big ", 0, len("hello big ")},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			area := &Textarea{}
			area.SetValue(tc.value)
			area.MoveTo(tc.cursor)
			if !area.Edit(tc.key) {
				t.Fatal("key was not handled")
			}
			if area.Value() != tc.want || area.Row() != tc.row || area.Col() != tc.col {
				t.Fatalf("Edit = %q at %d:%d, want %q at %d:%d", area.Value(), area.Row(), area.Col(), tc.want, tc.row, tc.col)
			}
		})
	}
}

func TestTextareaEditLeavesAppKeysAlone(t *testing.T) {
	area := &Textarea{}
	area.SetValue("hello")
	for _, key := range []flatte.KeyEvent{
		k(flatte.KeyEnter, 0),
		k(flatte.KeyEscape, 0),
		k(flatte.KeyTab, 0),
		k(flatte.KeyUp, 0),
		k(flatte.KeyDown, 0),
		ch('c', flatte.ModSuper),
		ch('v', flatte.ModSuper),
		ch('x', flatte.ModCtrl),
	} {
		if area.Edit(key) {
			t.Errorf("%+v was handled", key)
		}
	}
	if area.Value() != "hello" {
		t.Fatalf("unhandled keys changed value to %q", area.Value())
	}
}
