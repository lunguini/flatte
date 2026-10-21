package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/brand"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/window"
)

type State struct {
	ta        flatui.Textarea
	debugKeys bool
	// copyOnSelect puts a mouse selection on the clipboard as the drag
	// ends, as some terminals do for their own selection (-copy-on-select).
	copyOnSelect bool
	// path is the open file ("" untitled); saved is the text as last
	// opened or saved; status reports the last file operation.
	path, saved, status string
	// stats shows the statistics window; focus is the focused window ("" the
	// editor's), which keys and pastes belong to.
	stats      bool
	focus      string
	lastKey    string
	lastAction string
}

func NewState() *State {
	s := &State{debugKeys: os.Getenv("FLAT_DEBUG_KEYS") != ""}
	s.ta.SetValue("Edit me.\nMultiple lines.\nGrapheme-correct.")
	s.ta.SetSoftWrap(true)
	s.saved = s.ta.Value()
	return s
}

// dirty reports unsaved changes.
func (s *State) dirty() bool { return s.ta.Value() != s.saved }

// editorID names the textarea's node: the card's space left after the
// pinned rows, as the frame solved it.
const editorID = "editor"

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.LayoutEvent:
		if r, ok := e.Rects[editorID]; ok && e.Window == "" {
			s.ta.SetSize(max(r.W, 1), max(r.H, 1))
		}
	case flatte.WindowEvent:
		switch {
		case e.Kind == flatte.WindowFocused:
			s.focus = e.Window
		case e.Window == statsID: // its close button
			s.stats = false
		}
	case flatte.KeyEvent:
		if s.focus == statsID {
			// The statistics window takes no text; Esc closes it.
			if e.Key == flatte.KeyEscape {
				s.stats = false
			}
			return
		}
		handleKey(s, e, fx)
	case flatte.PasteEvent:
		if s.focus == statsID {
			return
		}
		s.lastAction = "paste"
		s.ta.InsertText(e.Text)
	case flatte.ClipboardEvent: // the answer to Paste's ReadClipboard
		s.lastAction = "paste"
		s.ta.InsertText(e.Text)
	case flatte.MenuEvent:
		command(s, e.ID, fx)
	case flatte.DropEvent: // a file dropped on the window opens
		if len(e.Paths) > 0 {
			path := e.Paths[0]
			discardThen(s, fx, func(s *State) { open(s, fx, path) })
		}
	case flatte.MouseEvent:
		handleMouse(s, e)
		if s.copyOnSelect && e.Button == flatte.MouseLeft && e.Action == flatte.MouseRelease {
			copySelection(s, fx)
		}
	}
}

// Menu commands.
const (
	cmdNew       = "new"
	cmdOpen      = "open"
	cmdSave      = "save"
	cmdSaveAs    = "save-as"
	cmdCut       = "cut"
	cmdCopy      = "copy"
	cmdPaste     = "paste"
	cmdSelectAll = "select-all"
	cmdWrap      = "wrap"
	cmdKeys      = "show-keys"
	cmdStats     = "stats"
)

func super(r rune) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r, Mod: flatte.ModSuper}
}

// editItems are the editing commands, in the Edit menu and the editor's
// context menu: ⌘ shortcuts for a window (and terminals that report ⌘);
// Ctrl+X too, which a terminal sends. Cut and Copy need a selection.
func editItems(s *State) []flatte.MenuItem {
	_, selected := s.ta.Selection()
	return []flatte.MenuItem{
		{ID: cmdCut, Label: "Cut", Keys: []flatte.KeyEvent{super('x'), {Key: flatte.KeyCharacter, Rune: 'x', Mod: flatte.ModCtrl}}, Disabled: !selected},
		{ID: cmdCopy, Label: "Copy", Keys: []flatte.KeyEvent{super('c')}, Disabled: !selected},
		{ID: cmdPaste, Label: "Paste", Keys: []flatte.KeyEvent{super('v')}},
		{Separator: true},
		{ID: cmdSelectAll, Label: "Select All", Keys: []flatte.KeyEvent{super('a')}},
	}
}

func ctrl(r rune) flatte.KeyEvent {
	return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r, Mod: flatte.ModCtrl}
}

// Menus is the menu bar: File, Edit, and View's toggles.
func Menus(s *State) []flatte.Menu {
	saveAs := super('s')
	saveAs.Mod |= flatte.ModShift
	return []flatte.Menu{
		{Title: "File", Items: []flatte.MenuItem{
			{ID: cmdNew, Label: "New", Keys: []flatte.KeyEvent{super('n'), ctrl('n')}},
			{ID: cmdOpen, Label: "Open…", Keys: []flatte.KeyEvent{super('o'), ctrl('o')}},
			{Separator: true},
			{ID: cmdSave, Label: "Save", Keys: []flatte.KeyEvent{super('s'), ctrl('s')}},
			{ID: cmdSaveAs, Label: "Save As…", Keys: []flatte.KeyEvent{saveAs}},
		}},
		{Title: "Edit", Items: editItems(s)},
		{Title: "View", Items: []flatte.MenuItem{
			{ID: cmdWrap, Label: "Wrap Lines", Checked: s.ta.SoftWrap()},
			{ID: cmdKeys, Label: "Show Keys", Checked: s.debugKeys},
			{Separator: true},
			{ID: cmdStats, Label: "Statistics", Checked: s.stats, Keys: []flatte.KeyEvent{statsKey, ctrl('t')}},
		}},
	}
}

// ContextMenu is the editing commands on the editor.
func ContextMenu(s *State, target string) []flatte.MenuItem {
	if target != editorID {
		return nil
	}
	return editItems(s)
}

// command runs a menu command.
func command(s *State, id string, fx flatte.Effects[State]) {
	s.lastKey, s.lastAction = "menu", id
	switch id {
	case cmdNew:
		discardThen(s, fx, func(s *State) {
			s.ta.SetValue("")
			s.path, s.saved, s.status = "", "", "New file."
		})
	case cmdOpen:
		discardThen(s, fx, func(s *State) {
			flatte.OpenFile(fx, "open-dialog", flatte.OpenDialog{Title: "Open", Directory: dirOf(s.path)}, func(s *State, paths []string, err error) {
				switch {
				case err != nil:
					s.status = err.Error()
				case len(paths) > 0:
					open(s, fx, paths[0])
				}
			})
		})
	case cmdSave:
		if s.path != "" {
			save(s, fx, s.path)
			break
		}
		saveAs(s, fx)
	case cmdSaveAs:
		saveAs(s, fx)
	case cmdCut:
		cutSelection(s, fx)
	case cmdCopy:
		copySelection(s, fx)
	case cmdPaste:
		fx.ReadClipboard()
	case cmdSelectAll:
		s.ta.SelectAll()
	case cmdWrap:
		s.ta.SetSoftWrap(!s.ta.SoftWrap())
	case cmdKeys:
		s.debugKeys = !s.debugKeys
	case cmdStats:
		s.stats = !s.stats
	}
}

// statsID is the statistics window's ID.
const statsID = "stats"

// statsKey is ⇧⌘I, the window's Statistics shortcut. A terminal gets
// Ctrl+T: it cannot send Ctrl+Shift+I (Ctrl+I is Tab, and Shift is lost on
// Ctrl+letter).
var statsKey = flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'i', Mod: flatte.ModSuper | flatte.ModShift}

// Windows declares the statistics window while it is shown.
func Windows(s *State) []flatte.Window {
	if !s.stats {
		return nil
	}
	return []flatte.Window{{ID: statsID, Width: 30, Height: 7, MinWidth: 24, MinHeight: 6, Autosave: "flat-editor.statistics"}}
}

// statsView is the statistics window: live counts of the text.
func statsView(s *State) flatte.Frame {
	text := s.ta.Value()
	selected := len([]rune(s.ta.SelectedText()))
	row := func(name string, n int) layout.Node {
		return layout.Row{Children: []layout.Node{
			layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(12)}, Text: name, Style: layout.TextStyle{Role: layout.TextSubtle}},
			layout.Label{NodeBase: layout.NodeBase{ID: "stat:" + name}, Text: strconv.Itoa(n), Style: layout.TextStyle{Bold: true}},
		}}
	}
	return flatte.Frame{Title: "Statistics — " + nameOr(s.path, "untitled"), Root: layout.Col{
		NodeBase: layout.NodeBase{Pad: 1},
		Children: []layout.Node{
			row("Words", len(strings.Fields(text))),
			row("Lines", strings.Count(text, "\n")+1),
			row("Characters", len([]rune(text))),
			row("Selected", selected),
			layout.NewSpacer(),
			layout.Label{Text: "esc closes", Style: layout.TextStyle{Role: layout.TextSubtle}},
		},
	}}
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

// The file commands. Their folds capture fx to chain the next step — a
// dialog's answer starting the read or write, which runs off the loop.

// discardThen runs then when there are no unsaved changes, or after the
// user agrees to discard them.
func discardThen(s *State, fx flatte.Effects[State], then func(*State)) {
	if !s.dirty() {
		then(s)
		return
	}
	flatte.Alert(fx, "discard-changes", flatte.AlertDialog{
		Title:   "Discard unsaved changes?",
		Message: "Your changes to " + displayName(s.path) + " will be lost.",
		Buttons: []string{"Discard", "Cancel"},
	}, func(s *State, button int) {
		if button == 0 {
			then(s)
		}
	})
}

func saveAs(s *State, fx flatte.Effects[State]) {
	flatte.SaveFile(fx, "save-dialog", flatte.SaveDialog{Title: "Save As", Directory: dirOf(s.path), Name: nameOr(s.path, "untitled.txt")}, func(s *State, path string, err error) {
		switch {
		case err != nil:
			s.status = err.Error()
		case path != "":
			save(s, fx, path)
		}
	})
}

// open reads path off the loop and shows it.
func open(s *State, fx flatte.Effects[State], path string) {
	s.status = "Opening " + filepath.Base(path) + "…"
	flatte.Go(fx, "read-file", func(context.Context) ([]byte, error) { return os.ReadFile(path) }, func(s *State, data []byte, err error) {
		if err != nil {
			s.status = err.Error()
			return
		}
		s.ta.SetValue(string(data))
		s.ta.MoveTo(0)
		s.path, s.saved, s.status = path, s.ta.Value(), "Opened "+filepath.Base(path)+"."
	})
}

// save writes the text to path off the loop.
func save(s *State, fx flatte.Effects[State], path string) {
	text := s.ta.Value()
	flatte.Go(fx, "write-file", func(context.Context) (struct{}, error) { return struct{}{}, os.WriteFile(path, []byte(text), 0o644) }, func(s *State, _ struct{}, err error) {
		if err != nil {
			s.status = err.Error()
			return
		}
		s.path, s.saved, s.status = path, text, "Saved "+filepath.Base(path)+"."
	})
}

func dirOf(path string) string {
	if path == "" {
		return ""
	}
	return filepath.Dir(path)
}

func nameOr(path, fallback string) string {
	if path == "" {
		return fallback
	}
	return filepath.Base(path)
}

func displayName(path string) string { return nameOr(path, "this untitled file") }

// handleSuper is the ⌘ (Super) chords that are not menu commands (those
// arrive as MenuEvents): the platform's line and document moves (⌘←/→ line start/end, ⌘↑/↓ the start
// or end of the text, with Shift to select; ⌘⌫ deletes to the line start).
// A window delivers ⌘←/→ as Home/End already; a terminal that reports
// Super delivers them as arrows with ModSuper. It reports whether key was
// one.
func handleSuper(s *State, key flatte.KeyEvent) bool {
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
	}
	return false
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
	if handleSuper(s, key) || (key.Key == flatte.KeyCharacter && key.Mod.Contains(flatte.ModSuper)) {
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
		if handleCtrlKey(s, key) {
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

// handleCtrlKey is the Control letter chords: ctrl+a / ctrl+e move to the
// line start / end, as in readline (some terminals send them for ⌘← / ⌘→);
// ctrl+x is the Cut command. It reports whether key was one.
func handleCtrlKey(s *State, key flatte.KeyEvent) bool {
	if !key.Mod.Contains(flatte.ModCtrl) {
		return false
	}
	switch key.Rune {
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
func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	if ctx.Window == statsID {
		return statsView(s)
	}
	editor := s.ta.Layout(editorID, true)
	editor.SelectionStyle = layout.TextStyle{Fg: lipgloss.Color("16"), Bg: brand.Teal}
	subtle := layout.TextStyle{Role: layout.TextSubtle}
	rows := []layout.Node{
		layout.Label{Text: "Flat Editor", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Label{Text: fileLine(s), Style: subtle},
		layout.Label{},
		editor,
		layout.Label{},
		layout.Label{Text: "ctrl+n new  ctrl+o open  ctrl+s save  ctrl+t stats  ⌘ in windows", Style: subtle},
		layout.Label{Text: "arrows  drag select  right-click menu  ⌘C/ctrl+c copy  esc quit", Style: subtle},
	}
	if s.debugKeys {
		rows = append(rows, layout.Label{Text: fmt.Sprintf("last: %s -> %s", s.lastKey, s.lastAction), Style: subtle})
	}
	return flatte.Frame{
		Root:        flatui.CardLayout(rows...),
		CursorStyle: cursorStyle(s),
	}
}

// fileLine is the open file's name, marked when edited, and the last file
// operation's status.
func fileLine(s *State) string {
	line := nameOr(s.path, "untitled")
	if s.dirty() {
		line += " (edited)"
	}
	if s.status != "" {
		line += " — " + s.status
	}
	return line
}

func main() {
	gui := flag.Bool("gui", false, "run in a native window instead of the terminal")
	copyOnSelect := flag.Bool("copy-on-select", false, "copy a mouse selection to the clipboard when the drag ends")
	flag.Parse()
	s := NewState()
	s.copyOnSelect = *copyOnSelect
	app := flatte.App[State]{
		State:       s,
		Handle:      Handle,
		View:        View,
		Menus:       Menus,
		ContextMenu: ContextMenu,
		Windows:     Windows,
	}
	var err error
	if *gui {
		err = window.Run(context.Background(), app, window.Options{Title: "Flat Editor", Width: 72, Height: 20, Autosave: "flat-editor.main"})
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
