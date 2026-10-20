package window

import (
	"unicode"

	"github.com/lunguini/flatte"
)

// macOS modifier flags (NSEventModifierFlags).
const (
	macShift   = 1 << 17
	macControl = 1 << 18
	macOption  = 1 << 19
	macCommand = 1 << 20
)

func macMods(flags uint) flatte.Mod {
	var m flatte.Mod
	if flags&macShift != 0 {
		m |= flatte.ModShift
	}
	if flags&macOption != 0 {
		m |= flatte.ModAlt
	}
	if flags&macControl != 0 {
		m |= flatte.ModCtrl
	}
	return m
}

// macSpecialKeys maps the characters AppKit reports for non-text keys
// (NSEvent charactersIgnoringModifiers) to Flatte keys.
var macSpecialKeys = map[rune]flatte.Key{
	0xF700: flatte.KeyUp, 0xF701: flatte.KeyDown, 0xF702: flatte.KeyLeft, 0xF703: flatte.KeyRight,
	0xF704: flatte.KeyF1, 0xF705: flatte.KeyF2, 0xF706: flatte.KeyF3, 0xF707: flatte.KeyF4,
	0xF708: flatte.KeyF5, 0xF709: flatte.KeyF6, 0xF70A: flatte.KeyF7, 0xF70B: flatte.KeyF8,
	0xF70C: flatte.KeyF9, 0xF70D: flatte.KeyF10, 0xF70E: flatte.KeyF11, 0xF70F: flatte.KeyF12,
	0xF728: flatte.KeyDelete, 0xF729: flatte.KeyHome, 0xF72B: flatte.KeyEnd,
	0xF72C: flatte.KeyPageUp, 0xF72D: flatte.KeyPageDown,
	'\r': flatte.KeyEnter, 0x03: flatte.KeyEnter, // Return, keypad Enter
	'\t': flatte.KeyTab, 0x19: flatte.KeyTab, // Tab, Shift-Tab (backtab)
	0x1b: flatte.KeyEscape, 0x7f: flatte.KeyBackspace,
}

// macCommandKeys are the Command chords with a Flatte key.
var macCommandKeys = map[rune]flatte.Key{0xF702: flatte.KeyHome, 0xF703: flatte.KeyEnd}

// macKey decides what a key press means before text input sees it: a
// non-text key, or a Control chord, becomes a key event directly; anything
// else is text for the input system (which also runs input methods) and
// macKey reports false. Command chords the menu does not claim (it sees
// them first) are keys with ModSuper — letters lowercased, as Control
// chords are — except Command-Left and -Right, the platform's line start
// and end, which become Home and End (as AppKit's moveToBeginningOfLine:
// does), keeping Shift so they can select.
func macKey(chars rune, flags uint) (flatte.KeyEvent, bool) {
	mod := macMods(flags)
	if flags&macCommand != 0 {
		if k, ok := macCommandKeys[chars]; ok {
			return flatte.KeyEvent{Key: k, Mod: mod}, true
		}
		if k, ok := macSpecialKeys[chars]; ok {
			return flatte.KeyEvent{Key: k, Mod: mod | flatte.ModSuper}, true
		}
		if unicode.IsPrint(chars) {
			return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: unicode.ToLower(chars), Mod: mod | flatte.ModSuper}, true
		}
		return flatte.KeyEvent{}, false
	}
	if k, ok := macSpecialKeys[chars]; ok {
		if chars == 0x19 {
			mod |= flatte.ModShift
		}
		return flatte.KeyEvent{Key: k, Mod: mod}, true
	}
	if flags&macControl != 0 && unicode.IsPrint(chars) {
		r := unicode.ToLower(chars)
		if r == 'c' {
			return flatte.KeyEvent{Key: flatte.KeyCtrlC, Mod: mod}, true
		}
		return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r, Mod: mod}, true
	}
	return flatte.KeyEvent{}, false
}

// textKeys turns committed text into one key event per rune, as a terminal
// delivers typed text.
func textKeys(text string) []flatte.KeyEvent {
	out := make([]flatte.KeyEvent, 0, len(text))
	for _, r := range text {
		if r == '\n' || r == '\r' {
			out = append(out, flatte.KeyEvent{Key: flatte.KeyEnter})
			continue
		}
		if unicode.IsPrint(r) {
			out = append(out, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: r})
		}
	}
	return out
}
