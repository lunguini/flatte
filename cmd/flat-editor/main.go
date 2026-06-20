package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
)

type State struct {
	ta         flatui.Textarea
	debugKeys  bool
	lastKey    string
	lastAction string
}

func NewState() *State {
	s := &State{debugKeys: os.Getenv("FLAT_DEBUG_KEYS") != ""}
	s.ta.SetValue("Edit me.\nMultiple lines.\nGrapheme-correct.")
	return s
}

// layout sizes the textarea to the rows left after the pinned chrome (title,
// subtitle, blank, blank, footer = 5) and the card's top+bottom border (2).
func (s *State) layout(width, height int) {
	const pinnedRows = 5 // title, subtitle, blank, blank, footer
	extraRows := 0
	if s.debugKeys {
		extraRows = 1
	}
	s.ta.SetSize(
		max(flatui.CardBodyWidth(width), 1),
		max(flatui.CardBodyHeight(height, pinnedRows+extraRows), 1),
	)
	s.ta.SetSoftWrap(true)
}

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	switch e := ev.(type) {
	case flat.ResizeEvent:
		s.layout(e.Width, e.Height)
	case flat.KeyEvent:
		handleKey(s, e, fx)
	}
}

func handleKey(s *State, key flat.KeyEvent, fx flat.Effects[State]) {
	s.lastKey = describeKey(key)
	s.lastAction = "ignored"
	switch key.Key {
	case flat.KeyEscape:
		s.lastAction = "quit"
		fx.Quit()
	case flat.KeyEnter:
		s.lastAction = "newline"
		s.ta.InsertNewline()
	case flat.KeyBackspace:
		if wordMove(key.Mod) {
			s.lastAction = "delete-word-left"
			s.ta.DeleteWordLeft()
		} else {
			s.lastAction = "backspace"
			s.ta.Backspace()
		}
	case flat.KeyDelete:
		if wordMove(key.Mod) {
			s.lastAction = "delete-word-right"
			s.ta.DeleteWordRight()
		} else {
			s.lastAction = "delete"
			s.ta.Delete()
		}
	case flat.KeyLeft:
		if key.Mod.Contains(flat.ModShift) && wordMove(key.Mod) {
			s.lastAction = "select-word-left"
			s.ta.MoveWordLeftSelecting()
		} else if key.Mod.Contains(flat.ModShift) {
			s.lastAction = "select-left"
			s.ta.MoveLeftSelecting()
		} else if wordMove(key.Mod) {
			s.lastAction = "move-word-left"
			s.ta.MoveWordLeft()
		} else {
			s.lastAction = "move-left"
			s.ta.MoveLeft()
		}
	case flat.KeyRight:
		if key.Mod.Contains(flat.ModShift) && wordMove(key.Mod) {
			s.lastAction = "select-word-right"
			s.ta.MoveWordRightSelecting()
		} else if key.Mod.Contains(flat.ModShift) {
			s.lastAction = "select-right"
			s.ta.MoveRightSelecting()
		} else if wordMove(key.Mod) {
			s.lastAction = "move-word-right"
			s.ta.MoveWordRight()
		} else {
			s.lastAction = "move-right"
			s.ta.MoveRight()
		}
	case flat.KeyUp:
		if key.Mod.Contains(flat.ModShift) {
			s.lastAction = "select-up"
			s.ta.MoveUpSelecting()
		} else {
			s.lastAction = "move-up"
			s.ta.MoveUp()
		}
	case flat.KeyDown:
		if key.Mod.Contains(flat.ModShift) {
			s.lastAction = "select-down"
			s.ta.MoveDownSelecting()
		} else {
			s.lastAction = "move-down"
			s.ta.MoveDown()
		}
	case flat.KeyCharacter:
		if handleWordDeleteKey(key, s.ta.DeleteWordLeft, s.ta.DeleteWordRight) {
			if key.Mod.Contains(flat.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
				s.lastAction = "delete-word-right"
			} else {
				s.lastAction = "delete-word-left"
			}
			return
		}
		if handleAltWordKey(key, s.ta.MoveWordLeft, s.ta.MoveWordRight) {
			if key.Rune == 'f' || key.Rune == 'F' {
				s.lastAction = "move-word-right"
			} else {
				s.lastAction = "move-word-left"
			}
			return
		}
		s.lastAction = "insert"
		s.ta.Insert(key.Rune)
	}
}

func wordMove(mod flat.Mod) bool {
	return mod.Contains(flat.ModAlt) || mod.Contains(flat.ModCtrl)
}

func handleAltWordKey(key flat.KeyEvent, moveLeft, moveRight func()) bool {
	if !key.Mod.Contains(flat.ModAlt) {
		return false
	}
	switch key.Rune {
	case 'b', 'B':
		moveLeft()
		return true
	case 'f', 'F':
		moveRight()
		return true
	}
	return false
}

func handleWordDeleteKey(key flat.KeyEvent, deleteLeft, deleteRight func()) bool {
	if key.Mod.Contains(flat.ModCtrl) && (key.Rune == 'w' || key.Rune == 'W' || key.Rune == 'h' || key.Rune == 'H') {
		deleteLeft()
		return true
	}
	if key.Mod.Contains(flat.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
		deleteRight()
		return true
	}
	return false
}

func describeKey(key flat.KeyEvent) string {
	if key.Key == flat.KeyCharacter {
		return fmt.Sprintf("character %q %s", key.Rune, describeMod(key.Mod))
	}
	return fmt.Sprintf("%s %s", keyName(key.Key), describeMod(key.Mod))
}

func describeMod(mod flat.Mod) string {
	var parts []string
	if mod.Contains(flat.ModCtrl) {
		parts = append(parts, "ctrl")
	}
	if mod.Contains(flat.ModAlt) {
		parts = append(parts, "alt")
	}
	if mod.Contains(flat.ModShift) {
		parts = append(parts, "shift")
	}
	if len(parts) == 0 {
		return "plain"
	}
	return strings.Join(parts, "+")
}

func keyName(key flat.Key) string {
	switch key {
	case flat.KeyUp:
		return "up"
	case flat.KeyDown:
		return "down"
	case flat.KeyEnter:
		return "enter"
	case flat.KeyCtrlC:
		return "ctrl-c"
	case flat.KeyBackspace:
		return "backspace"
	case flat.KeyTab:
		return "tab"
	case flat.KeyEscape:
		return "escape"
	case flat.KeyLeft:
		return "left"
	case flat.KeyRight:
		return "right"
	case flat.KeyDelete:
		return "delete"
	default:
		return "unknown"
	}
}

func View(s *State, ctx flat.RenderContext) flat.Frame {
	lines := []string{
		flatui.Title("Flat Editor"),
		flatui.Subtle("multi-line textarea sample"),
		"",
	}
	lines = append(lines, strings.Split(s.ta.ViewWithSelection(renderSelection), "\n")...)
	lines = append(lines, "", flatui.Subtle("arrows move  enter newline  esc quit"))
	if s.debugKeys {
		lines = append(lines, flatui.Subtle(fmt.Sprintf("last: %s -> %s", s.lastKey, s.lastAction)))
	}

	frame := flat.Frame{Content: flatui.Card(lines, ctx.Width)}
	// Place the hardware cursor: card origin + the three pinned lines (title,
	// subtitle, blank) that precede the textarea body + the cell within it.
	ox, oy := flatui.CardOrigin()
	cx, cy := s.ta.CursorCell()
	frame.Cursor = &flat.Cursor{
		X: ox + cx,
		Y: oy + 3 + cy,
		Style: &flat.CursorStyle{
			Shape: flat.CursorShapeBar,
			Blink: false,
		},
	}
	return frame
}

func renderSelection(text string, selected bool) string {
	if !selected {
		return text
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("16")).
		Background(lipgloss.Color("229")).
		Render(text)
}

func main() {
	if err := flat.Run(context.Background(), flat.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
