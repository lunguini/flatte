package flatui

import (
	"strings"
	"unicode/utf8"

	"github.com/lunguini/flatte"
)

// The editing commands every text input has: the IDs of its context menu's
// items (EditMenu), and what Do runs — an app's own Edit menu may use them
// too.
const (
	EditUndo      = "flatui.edit.undo"
	EditRedo      = "flatui.edit.redo"
	EditCut       = "flatui.edit.cut"
	EditCopy      = "flatui.edit.copy"
	EditPaste     = "flatui.edit.paste"
	EditDelete    = "flatui.edit.delete"
	EditSelectAll = "flatui.edit.selectAll"
)

// Clipboard is what an input needs to cut, copy, and paste: flatte.Effects.
// ReadClipboard's answer comes back as a ClipboardEvent, for Handle.
type Clipboard interface {
	SetClipboard(text string)
	ReadClipboard()
}

// EditCommand is the editing command key asks for, "" for none: ⌘ (or Ctrl)
// with X, C, V, Z, ⇧Z, or Y, and ⌘A.
func EditCommand(key flatte.KeyEvent) string {
	if key.Key != flatte.KeyCharacter {
		return ""
	}
	shift := key.Mod.Contains(flatte.ModShift)
	mod := key.Mod &^ flatte.ModShift
	if mod != flatte.ModSuper && mod != flatte.ModCtrl {
		return ""
	}
	switch r := key.Rune | 0x20; { // either case
	case r == 'z' && shift:
		return EditRedo
	case shift:
		return ""
	case r == 'z':
		return EditUndo
	case r == 'y' && mod == flatte.ModCtrl:
		return EditRedo
	case r == 'x':
		return EditCut
	case r == 'c':
		return EditCopy
	case r == 'v':
		return EditPaste
	case r == 'a' && mod == flatte.ModSuper: // Ctrl+A goes to the line's start
		return EditSelectAll
	}
	return ""
}

// editMenu is an input's context menu.
func editMenu(undo, redo, selected, text bool) []flatte.MenuItem {
	item := func(id, label string, r rune, mod flatte.Mod, on bool) flatte.MenuItem {
		return flatte.MenuItem{ID: id, Label: label, Disabled: !on, Keys: []flatte.KeyEvent{{Key: flatte.KeyCharacter, Rune: r, Mod: flatte.ModSuper | mod}}}
	}
	del := item(EditDelete, "Delete", 0, 0, selected)
	del.Keys = nil
	return []flatte.MenuItem{
		item(EditUndo, "Undo", 'z', 0, undo),
		item(EditRedo, "Redo", 'z', flatte.ModShift, redo),
		{Separator: true},
		item(EditCut, "Cut", 'x', 0, selected),
		item(EditCopy, "Copy", 'c', 0, selected),
		item(EditPaste, "Paste", 'v', 0, true),
		del,
		{Separator: true},
		item(EditSelectAll, "Select All", 'a', 0, text),
	}
}

// editState is an input's text, cursor, and selection, as it was.
type editState struct {
	value    string
	at, from int // cursor and, when selected, the selection's other end
	selected bool
	before   *editState
	n        int // states, this one included
}

// editHistory is what can be undone and redone. It is a value whose states
// are never changed, so a copied input has its own.
type editHistory struct {
	undo, redo *editState
	typedAt    int // where the last typed rune ended; -1: not typing
	typedIn    string
}

const maxUndo = 200

func push(list *editState, s editState) *editState {
	s.before, s.n = list, 1
	if list != nil {
		s.n = list.n + 1
	}
	if s.n > 2*maxUndo { // keep the newest half
		var kept []editState
		for e := &s; e != nil && len(kept) < maxUndo; e = e.before {
			kept = append(kept, *e)
		}
		list = nil
		for i := len(kept) - 1; i >= 0; i-- {
			list = push(list, kept[i])
		}
		return list
	}
	return &s
}

// input is what TextField and Textarea share.
type input interface {
	SelectedText() string
	InsertText(string)
	SelectAll()
	Edit(flatte.KeyEvent) bool
	HandleMouse(id string, e flatte.MouseEvent) bool
	removeSelection()
	edits() *editHistory
	state() editState
	restore(editState)
}

// editing runs fn and remembers what it changed. typed is text typed, which
// undoes together with what was typed just before it, word by word.
func editing(in input, typed string, fn func()) {
	was, h := in.state(), in.edits()
	fn()
	now := in.state()
	if now.value == was.value {
		return
	}
	joins := typed != "" && !was.selected && h.undo != nil && h.typedAt == was.at && h.typedIn == was.value &&
		!(strings.TrimSpace(typed) == "" && endsInWord(was.value[:was.at]))
	if !joins {
		h.undo = push(h.undo, was)
	}
	h.redo = nil
	h.typedAt, h.typedIn = -1, ""
	if typed != "" {
		h.typedAt, h.typedIn = now.at, now.value
	}
}

func endsInWord(s string) bool {
	r, _ := utf8.DecodeLastRuneInString(s)
	return s != "" && strings.TrimSpace(string(r)) != ""
}

// do runs an editing command; it reports whether cmd is one.
func do(in input, cmd string, clip Clipboard) bool {
	h := in.edits()
	step := func(from, to **editState) {
		if *from == nil {
			return
		}
		was, now := **from, in.state()
		*from, *to = was.before, push(*to, now)
		h.typedAt, h.typedIn = -1, ""
		in.restore(was)
	}
	switch cmd {
	case EditUndo:
		step(&h.undo, &h.redo)
	case EditRedo:
		step(&h.redo, &h.undo)
	case EditCut, EditCopy:
		text := in.SelectedText()
		if text == "" {
			return true
		}
		clip.SetClipboard(text)
		if cmd == EditCut {
			editing(in, "", in.removeSelection)
		}
	case EditPaste:
		clip.ReadClipboard()
	case EditDelete:
		editing(in, "", in.removeSelection)
	case EditSelectAll:
		in.SelectAll()
	default:
		return false
	}
	return true
}

func handle(in input, id string, ev flatte.Event, clip Clipboard) (used bool) {
	switch e := ev.(type) {
	case flatte.KeyEvent:
		if do(in, EditCommand(e), clip) {
			return true
		}
		typed := ""
		if e.Key == flatte.KeyCharacter && e.Mod&^flatte.ModShift == 0 {
			typed = string(e.Rune)
		}
		editing(in, typed, func() { used = in.Edit(e) })
	case flatte.PasteEvent:
		editing(in, "", func() { in.InsertText(e.Text) })
		return true
	case flatte.ClipboardEvent:
		editing(in, "", func() { in.InsertText(e.Text) })
		return true
	case flatte.MouseEvent:
		editing(in, "", func() { used = in.HandleMouse(id, e) })
	case flatte.MenuEvent:
		return e.Target == id && do(in, e.ID, clip)
	}
	return used
}

// Handle applies ev to the field laid out with id, and reports whether it
// used it: everything a one-line field does — Edit's keys, the clipboard
// and undo chords (EditCommand), pastes, the pointer (HandleMouse), and a
// choice from its context menu (EditMenu). What it changes can be undone.
// The app gives it keys and pastes while the field has focus, after its own
// keys (Enter, Esc, Tab); clip is the app's Effects. In a terminal app
// where Ctrl+C quits, take that first.
func (f *TextField) Handle(id string, ev flatte.Event, clip Clipboard) bool {
	return handle(f, id, ev, clip)
}

// Do runs an editing command (EditCopy and the like, from a menu of the
// app's own); it reports whether cmd is one.
func (f *TextField) Do(cmd string, clip Clipboard) bool { return do(f, cmd, clip) }

// EditMenu is the field's context menu, for App.ContextMenu to return for
// its ID; Handle takes the choice.
func (f *TextField) EditMenu() []flatte.MenuItem {
	return editMenu(f.CanUndo(), f.CanRedo(), f.SelectedText() != "", f.Value != "")
}

// CanUndo and CanRedo report whether Handle or Do changed the field in a
// way to undo, or to redo.
func (f *TextField) CanUndo() bool { return f.history.undo != nil }
func (f *TextField) CanRedo() bool { return f.history.redo != nil }

func (f *TextField) removeSelection()    { f.clampCursor(); f.deleteSelection() }
func (f *TextField) edits() *editHistory { return &f.history }

func (f *TextField) state() editState {
	f.clampCursor()
	s := editState{value: f.Value, at: f.Cursor}
	if _, _, ok := f.Selection(); ok {
		s.from, s.selected = f.selectionAnchor, true
	}
	return s
}

func (f *TextField) restore(s editState) {
	f.Value = s.value
	f.SetCursor(s.at)
	f.selectionAnchor, f.selectionActive = s.from, s.selected
}

// Handle applies ev to the textarea laid out with id, as TextField.Handle
// does. Enter, Tab, and ↑/↓ stay the app's, as with Edit.
func (t *Textarea) Handle(id string, ev flatte.Event, clip Clipboard) bool {
	return handle(t, id, ev, clip)
}

// Do, EditMenu, CanUndo, and CanRedo are TextField's.
func (t *Textarea) Do(cmd string, clip Clipboard) bool { return do(t, cmd, clip) }

func (t *Textarea) EditMenu() []flatte.MenuItem {
	return editMenu(t.CanUndo(), t.CanRedo(), t.SelectedText() != "", t.Value() != "")
}

func (t *Textarea) CanUndo() bool { return t.history.undo != nil }
func (t *Textarea) CanRedo() bool { return t.history.redo != nil }

// HandleMouse applies pointer editing for the textarea laid out with id: a
// press places the cursor (Shift selects to it), a drag selects, a double
// click selects a word, a triple click a line. It reports whether the event
// belonged to this textarea; focus is the app's.
func (t *Textarea) HandleMouse(id string, e flatte.MouseEvent) bool {
	if e.Target != id || e.Button != flatte.MouseLeft || !e.HasCaret {
		return false
	}
	switch e.Action {
	case flatte.MousePress:
		t.mouseClicks = max(e.Clicks, 1)
		switch {
		case e.Clicks >= 2:
			t.MoveTo(e.Caret)
			line := t.lines[t.row]
			a, b := 0, len(line)
			if e.Clicks == 2 {
				a, b = fieldWordRange(line, t.col)
			}
			start := t.offsetOf(TextPosition{Row: t.row})
			t.MoveTo(start + a)
			t.MoveToSelecting(start + b)
		case e.Mod.Contains(flatte.ModShift):
			t.MoveToSelecting(e.Caret)
		default:
			t.MoveTo(e.Caret)
		}
	case flatte.MouseMotion:
		if t.mouseClicks == 0 {
			return false
		}
		if t.mouseClicks == 1 {
			t.MoveToSelecting(e.Caret)
		}
	case flatte.MouseRelease:
		t.mouseClicks = 0
	default:
		return false
	}
	return true
}

func (t *Textarea) removeSelection()    { t.ensure(); t.deleteSelection(); t.keepVisible() }
func (t *Textarea) edits() *editHistory { return &t.history }

func (t *Textarea) state() editState {
	t.ensure()
	s := editState{value: t.Value(), at: t.offsetOf(TextPosition{Row: t.row, Col: t.col})}
	if _, ok := t.Selection(); ok {
		s.from, s.selected = t.offsetOf(TextPosition{Row: t.anchorRow, Col: t.anchorCol}), true
	}
	return s
}

func (t *Textarea) restore(s editState) {
	t.lines = strings.Split(s.value, "\n")
	t.ClearSelection()
	if s.selected {
		t.MoveTo(s.from)
		t.MoveToSelecting(s.at)
		return
	}
	t.MoveTo(s.at)
}
