package flatte

import "math"

// browserKey is the DOM-free part of a KeyboardEvent used by the browser
// host. Keeping translation here makes modifier behavior testable without a
// js/wasm runtime.
type browserKey struct {
	Name     string
	Mod      Mod
	AltGraph bool
}

type browserModifiers struct {
	Shift, Alt, Ctrl, Super bool
}

func browserModifierMask(mods browserModifiers) Mod {
	var mod Mod
	if mods.Shift {
		mod |= ModShift
	}
	if mods.Alt {
		mod |= ModAlt
	}
	if mods.Ctrl {
		mod |= ModCtrl
	}
	if mods.Super {
		mod |= ModSuper
	}
	return mod
}

func browserMouseButtonValue(button int) MouseButton {
	switch button {
	case 0:
		return MouseLeft
	case 1:
		return MouseMiddle
	case 2:
		return MouseRight
	default:
		return MouseNone
	}
}

func translateBrowserKey(key browserKey) (KeyEvent, bool) {
	var translated Key
	switch key.Name {
	case "ArrowUp":
		translated = KeyUp
	case "ArrowDown":
		translated = KeyDown
	case "ArrowLeft":
		translated = KeyLeft
	case "ArrowRight":
		translated = KeyRight
	case "Enter":
		translated = KeyEnter
	case "Escape":
		translated = KeyEscape
	case "Backspace":
		translated = KeyBackspace
	case "Delete":
		translated = KeyDelete
	case "Tab":
		translated = KeyTab
	case "Home":
		translated = KeyHome
	case "End":
		translated = KeyEnd
	case "PageUp":
		translated = KeyPageUp
	case "PageDown":
		translated = KeyPageDown
	default:
		translated = browserFunctionKey(key.Name)
	}
	if translated != KeyUnknown {
		return KeyEvent{Key: translated, Mod: key.Mod}, true
	}
	runes := []rune(key.Name)
	if len(runes) != 1 || key.AltGraph || key.Mod&(ModCtrl|ModAlt|ModSuper) == 0 {
		return KeyEvent{}, false
	}
	if key.Mod.Contains(ModCtrl) && !key.Mod.Contains(ModAlt) && (runes[0] == 'c' || runes[0] == 'C') {
		return KeyEvent{Key: KeyCtrlC, Mod: key.Mod}, true
	}
	return KeyEvent{Key: KeyCharacter, Rune: runes[0], Mod: key.Mod}, true
}

func browserFunctionKey(name string) Key {
	if len(name) < 2 || name[0] != 'F' {
		return KeyUnknown
	}
	n := 0
	for i := 1; i < len(name); i++ {
		if name[i] < '0' || name[i] > '9' {
			return KeyUnknown
		}
		n = n*10 + int(name[i]-'0')
	}
	if n < 1 || n > 12 {
		return KeyUnknown
	}
	return Key(int(KeyF1) + n - 1)
}

func browserPasteGesture(key browserKey) bool {
	if key.AltGraph || key.Mod&ModAlt != 0 || key.Name != "v" && key.Name != "V" {
		return false
	}
	mods := key.Mod &^ ModShift
	return mods == ModCtrl || mods == ModSuper
}

func browserTextEvents(text string, bracketed bool) []Event {
	if text == "" {
		return nil
	}
	if bracketed {
		return []Event{PasteEvent{Text: text}}
	}
	runes := []rune(text)
	events := make([]Event, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\r' || r == '\n' {
			events = append(events, KeyEvent{Key: KeyEnter})
			if r == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
				i++ // consume CRLF as one newline
			}
			continue
		}
		events = append(events, KeyEvent{Key: KeyCharacter, Rune: r})
	}
	return events
}

// browserWheelAccumulator turns pixel deltas into signed whole-line notches
// while retaining a fractional remainder between animation frames.
type browserWheelAccumulator struct{ rest float64 }

func (w *browserWheelAccumulator) notches(delta, line float64) int {
	if line <= 0 || math.IsNaN(delta) || math.IsInf(delta, 0) {
		return 0
	}
	w.rest += delta / line
	n := math.Trunc(w.rest)
	w.rest -= n
	return int(n)
}

func browserCellCoordinates(clientX, clientY, left, top, scrollX, scrollY float64, cols, rows int, cellWidth, lineHeight, padLeft, padTop float64) (int, int) {
	if cols < 1 {
		cols = 1
	}
	if rows < 1 {
		rows = 1
	}
	if cellWidth <= 0 {
		cellWidth = 1
	}
	if lineHeight <= 0 {
		lineHeight = 1
	}
	x := int((clientX - left - padLeft + scrollX) / cellWidth)
	y := int((clientY - top - padTop + scrollY) / lineHeight)
	return min(max(x, 0), cols-1), min(max(y, 0), rows-1)
}
