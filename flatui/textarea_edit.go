package flatui

import "github.com/lunguini/flatte"

// Edit applies key as a native multi-line text area does, and reports whether
// it was an editing key. It handles typing (replacing the selection), arrows
// (Shift selects, Alt or Ctrl moves by word, Super moves to the line boundary),
// Home/End, Backspace/Delete (word or line with modifiers), Super+A, and the
// terminal's Ctrl+A/E/U/K/W. Enter, Escape, Tab, Up/Down, and clipboard chords
// remain app-owned.
func (t *Textarea) Edit(key flatte.KeyEvent) bool {
	shift := key.Mod.Contains(flatte.ModShift)
	super := key.Mod.Contains(flatte.ModSuper)
	word := key.Mod.Contains(flatte.ModAlt) || key.Mod.Contains(flatte.ModCtrl)
	switch key.Key {
	case flatte.KeyCharacter:
		switch {
		case key.Mod&^flatte.ModShift == 0:
			t.Insert(key.Rune)
		case super && (key.Rune == 'a' || key.Rune == 'A') && key.Mod&^(flatte.ModSuper|flatte.ModShift) == 0:
			t.SelectAll()
		case key.Mod == flatte.ModCtrl:
			switch key.Rune {
			case 'a', 'A':
				t.MoveLineStart()
			case 'e', 'E':
				t.MoveLineEnd()
			case 'u', 'U':
				t.deleteToLineStart()
			case 'k', 'K':
				t.deleteToLineEnd()
			case 'w', 'W':
				t.DeleteWordLeft()
			default:
				return false
			}
		case key.Mod == flatte.ModAlt:
			// Readline's word keys, as TextField.Edit.
			switch key.Rune {
			case 'b':
				t.MoveWordLeft()
			case 'f':
				t.MoveWordRight()
			case 'd':
				t.DeleteWordRight()
			default:
				return false
			}
		default:
			return false
		}
	case flatte.KeyLeft, flatte.KeyRight:
		left := key.Key == flatte.KeyLeft
		switch {
		case super && shift && left:
			t.MoveLineStartSelecting()
		case super && shift:
			t.MoveLineEndSelecting()
		case super && left:
			t.MoveLineStart()
		case super:
			t.MoveLineEnd()
		case word && shift && left:
			t.MoveWordLeftSelecting()
		case word && shift:
			t.MoveWordRightSelecting()
		case word && left:
			t.MoveWordLeft()
		case word:
			t.MoveWordRight()
		case shift && left:
			t.MoveLeftSelecting()
		case shift:
			t.MoveRightSelecting()
		default:
			if selection, ok := t.Selection(); ok {
				to := selection.End
				if left {
					to = selection.Start
				}
				t.MoveTo(t.offsetOf(to))
			} else if left {
				t.MoveLeft()
			} else {
				t.MoveRight()
			}
		}
	case flatte.KeyHome, flatte.KeyEnd:
		start := key.Key == flatte.KeyHome
		switch {
		case shift && start:
			t.MoveLineStartSelecting()
		case shift:
			t.MoveLineEndSelecting()
		case start:
			t.MoveLineStart()
		default:
			t.MoveLineEnd()
		}
	case flatte.KeyBackspace:
		switch {
		case super:
			t.deleteToLineStart()
		case word:
			t.DeleteWordLeft()
		default:
			t.Backspace()
		}
	case flatte.KeyDelete:
		switch {
		case super:
			t.deleteToLineEnd()
		case word:
			t.DeleteWordRight()
		default:
			t.Delete()
		}
	default:
		return false
	}
	return true
}

func (t *Textarea) deleteToLineStart() {
	if _, selected := t.Selection(); selected {
		t.Backspace()
		return
	}
	t.ensure()
	t.lines[t.row] = t.lines[t.row][t.col:]
	t.col = 0
	t.syncGoal()
	t.keepVisible()
}

func (t *Textarea) deleteToLineEnd() {
	if _, selected := t.Selection(); selected {
		t.Backspace()
		return
	}
	t.ensure()
	t.lines[t.row] = t.lines[t.row][:t.col]
	t.syncGoal()
	t.keepVisible()
}
