package flatte

import (
	"unicode"
	"unicode/utf8"

	uv "github.com/charmbracelet/ultraviolet"
)

// translateEvent maps a substrate event onto the closed event set. Events
// Flatte does not model yet (key releases, capability
// reports) are dropped — the closed set grows deliberately, not by leaking
// substrate types.
func translateEvent(event uv.Event) (Event, bool) {
	switch event := event.(type) {
	case uv.KeyPressEvent:
		return translateKey(uv.Key(event))
	case uv.PasteEvent:
		return PasteEvent{Text: event.Content}, true
	case uv.ClipboardEvent:
		return ClipboardEvent{Text: event.Content}, true
	case uv.FocusEvent:
		return FocusEvent{Focused: true}, true
	case uv.BlurEvent:
		return FocusEvent{Focused: false}, true
	case uv.WindowSizeEvent:
		return ResizeEvent{Width: event.Width, Height: event.Height}, true
	case uv.MouseClickEvent:
		return translateMouse(uv.Mouse(event), MousePress), true
	case uv.MouseReleaseEvent:
		return translateMouse(uv.Mouse(event), MouseRelease), true
	case uv.MouseMotionEvent:
		return translateMouse(uv.Mouse(event), MouseMotion), true
	case uv.MouseWheelEvent:
		return translateMouse(uv.Mouse(event), MousePress), true
	}
	return nil, false
}

func translateKey(key uv.Key) (Event, bool) {
	mod := translateMod(key.Mod)
	if key.Mod.Contains(uv.ModCtrl) && (key.Code == 'c' || key.Code == 'C') {
		return KeyEvent{Key: KeyCtrlC, Mod: mod}, true
	}
	switch key.Code {
	case uv.KeyUp:
		return KeyEvent{Key: KeyUp, Mod: mod}, true
	case uv.KeyDown:
		return KeyEvent{Key: KeyDown, Mod: mod}, true
	case uv.KeyLeft:
		return KeyEvent{Key: KeyLeft, Mod: mod}, true
	case uv.KeyRight:
		return KeyEvent{Key: KeyRight, Mod: mod}, true
	case uv.KeyEnter:
		return KeyEvent{Key: KeyEnter, Mod: mod}, true
	case uv.KeyTab:
		return KeyEvent{Key: KeyTab, Mod: mod}, true
	case uv.KeyEscape:
		return KeyEvent{Key: KeyEscape, Mod: mod}, true
	case uv.KeyBackspace:
		return KeyEvent{Key: KeyBackspace, Mod: mod}, true
	case uv.KeyDelete:
		return KeyEvent{Key: KeyDelete, Mod: mod}, true
	case uv.KeyHome, uv.KeyKpHome:
		return KeyEvent{Key: KeyHome, Mod: mod}, true
	case uv.KeyEnd, uv.KeyKpEnd:
		return KeyEvent{Key: KeyEnd, Mod: mod}, true
	case uv.KeyPgUp, uv.KeyKpPgUp:
		return KeyEvent{Key: KeyPageUp, Mod: mod}, true
	case uv.KeyPgDown, uv.KeyKpPgDown:
		return KeyEvent{Key: KeyPageDown, Mod: mod}, true
	case uv.KeyF1:
		return KeyEvent{Key: KeyF1, Mod: mod}, true
	case uv.KeyF2:
		return KeyEvent{Key: KeyF2, Mod: mod}, true
	case uv.KeyF3:
		return KeyEvent{Key: KeyF3, Mod: mod}, true
	case uv.KeyF4:
		return KeyEvent{Key: KeyF4, Mod: mod}, true
	case uv.KeyF5:
		return KeyEvent{Key: KeyF5, Mod: mod}, true
	case uv.KeyF6:
		return KeyEvent{Key: KeyF6, Mod: mod}, true
	case uv.KeyF7:
		return KeyEvent{Key: KeyF7, Mod: mod}, true
	case uv.KeyF8:
		return KeyEvent{Key: KeyF8, Mod: mod}, true
	case uv.KeyF9:
		return KeyEvent{Key: KeyF9, Mod: mod}, true
	case uv.KeyF10:
		return KeyEvent{Key: KeyF10, Mod: mod}, true
	case uv.KeyF11:
		return KeyEvent{Key: KeyF11, Mod: mod}, true
	case uv.KeyF12:
		return KeyEvent{Key: KeyF12, Mod: mod}, true
	}
	if key.Text != "" {
		// Multi-rune Text (IME composites) is truncated to its first rune
		// until the grapheme work in Phase 7.
		r, _ := utf8.DecodeRuneInString(key.Text)
		return KeyEvent{Key: KeyCharacter, Rune: r, Mod: mod}, true
	}
	if mod != 0 && isPrintableKeyCode(key.Code) {
		return KeyEvent{Key: KeyCharacter, Rune: unicode.ToLower(key.Code), Mod: mod}, true
	}
	return nil, false
}

func isPrintableKeyCode(code rune) bool {
	return code >= ' ' && code < uv.KeyExtended
}

func translateMouse(mouse uv.Mouse, action MouseAction) MouseEvent {
	return MouseEvent{
		X:      mouse.X,
		Y:      mouse.Y,
		Button: translateButton(mouse.Button),
		Action: action,
		Mod:    translateMod(mouse.Mod),
	}
}

func translateButton(button uv.MouseButton) MouseButton {
	switch button {
	case uv.MouseLeft:
		return MouseLeft
	case uv.MouseMiddle:
		return MouseMiddle
	case uv.MouseRight:
		return MouseRight
	case uv.MouseWheelUp:
		return MouseWheelUp
	case uv.MouseWheelDown:
		return MouseWheelDown
	}
	return MouseNone
}

func translateMod(mod uv.KeyMod) Mod {
	var m Mod
	if mod.Contains(uv.ModShift) {
		m |= ModShift
	}
	if mod.Contains(uv.ModAlt) {
		m |= ModAlt
	}
	if mod.Contains(uv.ModCtrl) {
		m |= ModCtrl
	}
	// Command reaches an app as Super (kitty protocol) or, in the xterm
	// modifier encoding macOS terminals use (Warp: ⌘← is CSI 1;9D), as
	// Meta. Both are ModSuper, so an app binds one modifier.
	if mod.Contains(uv.ModSuper) || mod.Contains(uv.ModMeta) {
		m |= ModSuper
	}
	return m
}
