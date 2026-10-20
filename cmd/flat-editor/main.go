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
	case flatte.PasteEvent:
		s.lastAction = "paste"
		s.ta.InsertText(e.Text)
	case flatte.MouseEvent:
		handleMouse(s, e)
	}
}

// handleMouse places the caret where the host says a click or drag in the
// editor points (shift-click extends the selection), so it lands on the
// character under the pointer in any font.
func handleMouse(s *State, m flatte.MouseEvent) {
	if m.Target != editorID || !m.HasCaret || m.Button != flatte.MouseLeft {
		return
	}
	switch {
	case m.Action == flatte.MousePress && !m.Mod.Contains(flatte.ModShift):
		s.ta.MoveTo(m.Caret)
	case m.Action == flatte.MousePress, m.Action == flatte.MouseMotion:
		s.ta.MoveToSelecting(m.Caret)
	}
}

// handleSuper is the ⌘ (Super) chords: copy, cut, select all, and the
// platform's line and document moves (⌘←/→ line start/end, ⌘↑/↓ the start
// or end of the text, with Shift to select; ⌘⌫ deletes to the line start).
// A window delivers ⌘←/→ as Home/End already; a terminal that reports
// Super delivers them as arrows with ModSuper. It reports whether key was
// one.
func handleSuper(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) bool {
	if !key.Mod.Contains(flatte.ModSuper) {
		return false
	}
	selecting := key.Mod.Contains(flatte.ModShift)
	switch key.Key {
	case flatte.KeyLeft:
		s.lastAction = "move-line-start"
		if selecting {
			s.ta.MoveLineStartSelecting()
		} else {
			s.ta.MoveLineStart()
		}
		return true
	case flatte.KeyRight:
		s.lastAction = "move-line-end"
		if selecting {
			s.ta.MoveLineEndSelecting()
		} else {
			s.ta.MoveLineEnd()
		}
		return true
	case flatte.KeyUp, flatte.KeyDown:
		to := 0
		s.lastAction = "move-text-start"
		if key.Key == flatte.KeyDown {
			to, s.lastAction = len(s.ta.Value()), "move-text-end"
		}
		if selecting {
			s.ta.MoveToSelecting(to)
		} else {
			s.ta.MoveTo(to)
		}
		return true
	case flatte.KeyBackspace:
		s.lastAction = "delete-line-start"
		if _, sel := s.ta.Selection(); !sel {
			s.ta.MoveLineStartSelecting()
		}
		s.ta.Backspace()
		return true
	case flatte.KeyCharacter:
	default:
		return false
	}
	switch key.Rune {
	case 'c':
		s.lastAction = "copy"
		copySelection(s, fx)
	case 'x':
		s.lastAction = "cut"
		cutSelection(s, fx)
	case 'a':
		s.lastAction = "select-all"
		s.ta.SelectAll()
	default:
		return false
	}
	return true
}

// copySelection puts the selection on the clipboard, reporting whether
// there was one.
func copySelection(s *State, fx flatte.Effects[State]) bool {
	text := s.ta.SelectedText()
	if text != "" {
		fx.SetClipboard(text)
	}
	return text != ""
}

func cutSelection(s *State, fx flatte.Effects[State]) {
	if copySelection(s, fx) {
		s.ta.Backspace() // deletes the selection
	}
}

// vertical moves the cursor a row up (dir < 0) or down: to the caret the
// host computed in its own layout when it gave one (a window wraps by
// words), else by the textarea's rows.
func vertical(s *State, key flatte.KeyEvent, dir int) {
	selecting := key.Mod.Contains(flatte.ModShift)
	switch {
	case key.HasCaret && selecting:
		s.ta.MoveToSelecting(key.Caret)
	case key.HasCaret:
		s.ta.MoveTo(key.Caret)
	case dir < 0 && selecting:
		s.ta.MoveUpSelecting()
	case dir < 0:
		s.ta.MoveUp()
	case selecting:
		s.ta.MoveDownSelecting()
	default:
		s.ta.MoveDown()
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	s.lastKey = describeKey(key)
	s.lastAction = "ignored"
	if handleSuper(s, key, fx) || (key.Key == flatte.KeyCharacter && key.Mod.Contains(flatte.ModSuper)) {
		return // other ⌘ chords are not text
	}
	switch key.Key {
	case flatte.KeyEscape:
		s.lastAction = "quit"
		fx.Quit()
	case flatte.KeyCtrlC:
		// A terminal keeps ⌘C for itself: ctrl+c copies a selection, and
		// quits when there is none.
		if copySelection(s, fx) {
			s.lastAction = "copy"
		} else {
			s.lastAction = "quit"
			fx.Quit()
		}
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
		s.lastAction = "move-up"
		if key.Mod.Contains(flatte.ModShift) {
			s.lastAction = "select-up"
		}
		vertical(s, key, -1)
	case flatte.KeyDown:
		s.lastAction = "move-down"
		if key.Mod.Contains(flatte.ModShift) {
			s.lastAction = "select-down"
		}
		vertical(s, key, 1)
	case flatte.KeyCharacter:
		if handleWordDeleteKey(key, s.ta.DeleteWordLeft, s.ta.DeleteWordRight) {
			if key.Mod.Contains(flatte.ModAlt) && (key.Rune == 'd' || key.Rune == 'D') {
				s.lastAction = "delete-word-right"
			} else {
				s.lastAction = "delete-word-left"
			}
			return
		}
		if handleCtrlKey(s, key, fx) {
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

// handleCtrlKey is the Control letter chords: ctrl+x cuts, and ctrl+a /
// ctrl+e move to the line start / end, as in readline (some terminals send
// them for ⌘← / ⌘→). It reports whether key was one.
func handleCtrlKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) bool {
	if !key.Mod.Contains(flatte.ModCtrl) {
		return false
	}
	switch key.Rune {
	case 'x', 'X':
		s.lastAction = "cut"
		cutSelection(s, fx)
	case 'a', 'A':
		s.lastAction = "move-line-start"
		s.ta.MoveLineStart()
	case 'e', 'E':
		s.lastAction = "move-line-end"
		s.ta.MoveLineEnd()
	default:
		return false
	}
	return true
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
	if mod.Contains(flatte.ModSuper) {
		parts = append(parts, "super")
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
		layout.Label{Text: "arrows move  click/drag select  ctrl+c copy  esc quit", Style: subtle},
	}
	if s.debugKeys {
		rows = append(rows, layout.Label{Text: fmt.Sprintf("last: %s -> %s", s.lastKey, s.lastAction), Style: subtle})
	}
	return flatte.Frame{
		Root:        flatui.CardLayout(rows...),
		CursorStyle: cursorStyle(s),
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
		// Ctrl+C is the editor's (copy, or quit with nothing selected).
		err = flatte.Run(context.Background(), app, flatte.WithMouse(flatte.MouseModeCellMotion), flatte.WithoutDefaultQuit())
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// cursorStyle is a blinking block, which stays visible whatever the
// terminal's cursor color — or, while text is selected, a bar, so the
// cursor marks the selection's moving end without covering its last cell.
func cursorStyle(s *State) *flatte.CursorStyle {
	shape := flatte.CursorShapeBlock
	if _, selected := s.ta.Selection(); selected {
		shape = flatte.CursorShapeBar
	}
	return &flatte.CursorStyle{Shape: shape, Blink: true}
}
