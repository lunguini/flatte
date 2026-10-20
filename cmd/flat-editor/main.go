package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/brand"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/window"
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
	s.ta.SetSoftWrap(true)
	return s
}

// editorID names the textarea's node: the card's space left after the
// pinned rows, as the frame solved it.
const editorID = "editor"

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.LayoutEvent:
		if r, ok := e.Rects[editorID]; ok {
			s.ta.SetSize(max(r.W, 1), max(r.H, 1))
		}
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	s.lastKey = describeKey(key)
	s.lastAction = "ignored"
	switch key.Key {
	case flatte.KeyEscape:
		s.lastAction = "quit"
		fx.Quit()
	case flatte.KeyEnter:
		s.lastAction = "newline"
		s.ta.InsertNewline()
	case flatte.KeyBackspace:
		if wordMove(key.Mod) {
			s.lastAction = "delete-word-left"
			s.ta.DeleteWordLeft()
		} else {
			s.lastAction = "backspace"
			s.ta.Backspace()
		}
	case flatte.KeyDelete:
		if wordMove(key.Mod) {
			s.lastAction = "delete-word-right"
			s.ta.DeleteWordRight()
		} else {
			s.lastAction = "delete"
			s.ta.Delete()
		}
	case flatte.KeyLeft:
		if key.Mod.Contains(flatte.ModShift) && wordMove(key.Mod) {
			s.lastAction = "select-word-left"
			s.ta.MoveWordLeftSelecting()
		} else if key.Mod.Contains(flatte.ModShift) {
			s.lastAction = "select-left"
			s.ta.MoveLeftSelecting()
		} else if wordMove(key.Mod) {
			s.lastAction = "move-word-left"
			s.ta.MoveWordLeft()
		} else {
			s.lastAction = "move-left"
			s.ta.MoveLeft()
		}
	case flatte.KeyRight:
		if key.Mod.Contains(flatte.ModShift) && wordMove(key.Mod) {
			s.lastAction = "select-word-right"
			s.ta.MoveWordRightSelecting()
		} else if key.Mod.Contains(flatte.ModShift) {
			s.lastAction = "select-right"
			s.ta.MoveRightSelecting()
		} else if wordMove(key.Mod) {
			s.lastAction = "move-word-right"
			s.ta.MoveWordRight()
		} else {
			s.lastAction = "move-right"
			s.ta.MoveRight()
		}
	case flatte.KeyHome:
		if key.Mod.Contains(flatte.ModShift) {
			s.lastAction = "select-line-start"
			s.ta.MoveLineStartSelecting()
		} else {
			s.lastAction = "move-line-start"
			s.ta.MoveLineStart()
		}
	case flatte.KeyEnd:
		if key.Mod.Contains(flatte.ModShift) {
			s.lastAction = "select-line-end"
			s.ta.MoveLineEndSelecting()
		} else {
			s.lastAction = "move-line-end"
			s.ta.MoveLineEnd()
		}
	case flatte.KeyUp:
		if key.Mod.Contains(flatte.ModShift) {
			s.lastAction = "select-up"
			s.ta.MoveUpSelecting()
		} else {
			s.lastAction = "move-up"
			s.ta.MoveUp()
		}
	case flatte.KeyDown:
		if key.Mod.Contains(flatte.ModShift) {
			s.lastAction = "select-down"
			s.ta.MoveDownSelecting()
		} else {
			s.lastAction = "move-down"
			s.ta.MoveDown()
		}
	case flatte.KeyCharacter:
		if handleWordDeleteKey(key, s.ta.DeleteWordLeft, s.ta.DeleteWordRight) {
			if key.Mod.Contains(flatte.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
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

func wordMove(mod flatte.Mod) bool {
	return mod.Contains(flatte.ModAlt) || mod.Contains(flatte.ModCtrl)
}

func handleAltWordKey(key flatte.KeyEvent, moveLeft, moveRight func()) bool {
	if !key.Mod.Contains(flatte.ModAlt) {
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

func handleWordDeleteKey(key flatte.KeyEvent, deleteLeft, deleteRight func()) bool {
	if key.Mod.Contains(flatte.ModCtrl) && (key.Rune == 'w' || key.Rune == 'W' || key.Rune == 'h' || key.Rune == 'H') {
		deleteLeft()
		return true
	}
	if key.Mod.Contains(flatte.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
		deleteRight()
		return true
	}
	return false
}

func describeKey(key flatte.KeyEvent) string {
	if key.Key == flatte.KeyCharacter {
		return fmt.Sprintf("character %q %s", key.Rune, describeMod(key.Mod))
	}
	return fmt.Sprintf("%s %s", keyName(key.Key), describeMod(key.Mod))
}

func describeMod(mod flatte.Mod) string {
	var parts []string
	if mod.Contains(flatte.ModCtrl) {
		parts = append(parts, "ctrl")
	}
	if mod.Contains(flatte.ModAlt) {
		parts = append(parts, "alt")
	}
	if mod.Contains(flatte.ModShift) {
		parts = append(parts, "shift")
	}
	if len(parts) == 0 {
		return "plain"
	}
	return strings.Join(parts, "+")
}

func keyName(key flatte.Key) string {
	switch key {
	case flatte.KeyUp:
		return "up"
	case flatte.KeyDown:
		return "down"
	case flatte.KeyEnter:
		return "enter"
	case flatte.KeyCtrlC:
		return "ctrl-c"
	case flatte.KeyBackspace:
		return "backspace"
	case flatte.KeyTab:
		return "tab"
	case flatte.KeyEscape:
		return "escape"
	case flatte.KeyLeft:
		return "left"
	case flatte.KeyRight:
		return "right"
	case flatte.KeyDelete:
		return "delete"
	case flatte.KeyHome:
		return "home"
	case flatte.KeyEnd:
		return "end"
	default:
		return "unknown"
	}
}

// View shows the textarea as Textarea.Layout, a multi-line Input: the
// terminal draws its cells and places the cursor at its caret; a window
// draws a native text area.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	editor := s.ta.Layout(editorID, true)
	editor.SelectionStyle = layout.TextStyle{Fg: lipgloss.Color("16"), Bg: brand.Teal}
	subtle := layout.TextStyle{Role: layout.TextSubtle}
	rows := []layout.Node{
		layout.Label{Text: "Flat Editor", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Label{Text: "multi-line textarea sample", Style: subtle},
		layout.Label{},
		editor,
		layout.Label{},
		layout.Label{Text: "arrows move  enter newline  esc quit", Style: subtle},
	}
	if s.debugKeys {
		rows = append(rows, layout.Label{Text: fmt.Sprintf("last: %s -> %s", s.lastKey, s.lastAction), Style: subtle})
	}
	return flatte.Frame{
		Root:        flatui.CardLayout(rows...),
		CursorStyle: &flatte.CursorStyle{Shape: flatte.CursorShapeBar},
	}
}

func main() {
	gui := flag.Bool("gui", false, "run in a native window instead of the terminal")
	flag.Parse()
	app := flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}
	var err error
	if *gui {
		err = window.Run(context.Background(), app, window.Options{Title: "Flat Editor", Width: 72, Height: 20})
	} else {
		err = flatte.Run(context.Background(), app)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
