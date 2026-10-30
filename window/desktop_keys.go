package window

import "github.com/lunguini/flatte"

// desktopHotkey maps the shared key vocabulary to macOS's Carbon key codes.
// Require a non-Shift modifier so a global shortcut cannot swallow typing.
func desktopHotkey(k flatte.KeyEvent) (code, modifiers uint32, ok bool) {
	if k.Mod & ^(flatte.ModShift|flatte.ModAlt|flatte.ModCtrl|flatte.ModSuper) != 0 || k.Mod&(flatte.ModAlt|flatte.ModCtrl|flatte.ModSuper) == 0 {
		return 0, 0, false
	}
	switch k.Key {
	case flatte.KeyCharacter:
		if k.Rune != ' ' {
			return 0, 0, false
		}
		code = 49
	case flatte.KeyEnter:
		code = 36
	case flatte.KeyEscape:
		code = 53
	case flatte.KeyTab:
		code = 48
	default:
		if k.Key < flatte.KeyF1 || k.Key > flatte.KeyF12 {
			return 0, 0, false
		}
		code = []uint32{122, 120, 99, 118, 96, 97, 98, 100, 101, 109, 103, 111}[k.Key-flatte.KeyF1]
	}
	for _, m := range []struct {
		mod  flatte.Mod
		flag uint32
	}{{flatte.ModShift, 1 << 9}, {flatte.ModAlt, 1 << 11}, {flatte.ModCtrl, 1 << 12}, {flatte.ModSuper, 1 << 8}} {
		if k.Mod.Contains(m.mod) {
			modifiers |= m.flag
		}
	}
	return code, modifiers, true
}
