package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
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
}

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	switch e := ev.(type) {
	case flatcore.ResizeEvent:
		s.layout(e.Width, e.Height)
	case flatcore.KeyEvent:
		handleKey(s, e, fx)
	}
}

func handleKey(s *State, key flatcore.KeyEvent, fx flatcore.Effects[State]) {
	s.lastKey = describeKey(key)
	s.lastAction = "ignored"
	switch key.Key {
	case flatcore.KeyEscape:
		s.lastAction = "quit"
		fx.Quit()
	case flatcore.KeyEnter:
		s.lastAction = "newline"
		s.ta.InsertNewline()
	case flatcore.KeyBackspace:
		if wordMove(key.Mod) {
			s.lastAction = "delete-word-left"
			s.ta.DeleteWordLeft()
		} else {
			s.lastAction = "backspace"
			s.ta.Backspace()
		}
	case flatcore.KeyDelete:
		if wordMove(key.Mod) {
			s.lastAction = "delete-word-right"
			s.ta.DeleteWordRight()
		} else {
			s.lastAction = "delete"
			s.ta.Delete()
		}
	case flatcore.KeyLeft:
		if wordMove(key.Mod) {
			s.lastAction = "move-word-left"
			s.ta.MoveWordLeft()
		} else {
			s.lastAction = "move-left"
			s.ta.MoveLeft()
		}
	case flatcore.KeyRight:
		if wordMove(key.Mod) {
			s.lastAction = "move-word-right"
			s.ta.MoveWordRight()
		} else {
			s.lastAction = "move-right"
			s.ta.MoveRight()
		}
	case flatcore.KeyUp:
		s.lastAction = "move-up"
		s.ta.MoveUp()
	case flatcore.KeyDown:
		s.lastAction = "move-down"
		s.ta.MoveDown()
	case flatcore.KeyCharacter:
		if handleWordDeleteKey(key, s.ta.DeleteWordLeft, s.ta.DeleteWordRight) {
			if key.Mod.Contains(flatcore.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
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

func wordMove(mod flatcore.Mod) bool {
	return mod.Contains(flatcore.ModAlt) || mod.Contains(flatcore.ModCtrl)
}

func handleAltWordKey(key flatcore.KeyEvent, moveLeft, moveRight func()) bool {
	if !key.Mod.Contains(flatcore.ModAlt) {
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

func handleWordDeleteKey(key flatcore.KeyEvent, deleteLeft, deleteRight func()) bool {
	if key.Mod.Contains(flatcore.ModCtrl) && (key.Rune == 'w' || key.Rune == 'W' || key.Rune == 'h' || key.Rune == 'H') {
		deleteLeft()
		return true
	}
	if key.Mod.Contains(flatcore.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
		deleteRight()
		return true
	}
	return false
}

func describeKey(key flatcore.KeyEvent) string {
	if key.Key == flatcore.KeyCharacter {
		return fmt.Sprintf("character %q %s", key.Rune, describeMod(key.Mod))
	}
	return fmt.Sprintf("%s %s", keyName(key.Key), describeMod(key.Mod))
}

func describeMod(mod flatcore.Mod) string {
	var parts []string
	if mod.Contains(flatcore.ModCtrl) {
		parts = append(parts, "ctrl")
	}
	if mod.Contains(flatcore.ModAlt) {
		parts = append(parts, "alt")
	}
	if mod.Contains(flatcore.ModShift) {
		parts = append(parts, "shift")
	}
	if len(parts) == 0 {
		return "plain"
	}
	return strings.Join(parts, "+")
}

func keyName(key flatcore.Key) string {
	switch key {
	case flatcore.KeyUp:
		return "up"
	case flatcore.KeyDown:
		return "down"
	case flatcore.KeyEnter:
		return "enter"
	case flatcore.KeyCtrlC:
		return "ctrl-c"
	case flatcore.KeyBackspace:
		return "backspace"
	case flatcore.KeyTab:
		return "tab"
	case flatcore.KeyEscape:
		return "escape"
	case flatcore.KeyLeft:
		return "left"
	case flatcore.KeyRight:
		return "right"
	case flatcore.KeyDelete:
		return "delete"
	default:
		return "unknown"
	}
}

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
	lines := []string{
		flatui.Title("Flat Editor"),
		flatui.Subtle("multi-line textarea sample"),
		"",
	}
	lines = append(lines, strings.Split(s.ta.View(), "\n")...)
	lines = append(lines, "", flatui.Subtle("arrows move  enter newline  esc quit"))
	if s.debugKeys {
		lines = append(lines, flatui.Subtle(fmt.Sprintf("last: %s -> %s", s.lastKey, s.lastAction)))
	}

	frame := flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
	// Place the hardware cursor: card origin + the three pinned lines (title,
	// subtitle, blank) that precede the textarea body + the cell within it.
	ox, oy := flatui.CardOrigin()
	cx, cy := s.ta.CursorCell()
	frame.Cursor = &flatcore.Cursor{X: ox + cx, Y: oy + 3 + cy}
	return frame
}

func main() {
	if err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
