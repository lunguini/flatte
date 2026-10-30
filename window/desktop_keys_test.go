package window

import (
	"github.com/lunguini/flatte"
	"testing"
)

func TestDesktopHotkeyMappingAndTypingSafety(t *testing.T) {
	code, mods, ok := desktopHotkey(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: ' ', Mod: flatte.ModSuper | flatte.ModShift})
	if !ok || code != 49 || mods != 1<<8|1<<9 {
		t.Fatalf("space chord: %d %d %v", code, mods, ok)
	}
	for _, key := range []flatte.KeyEvent{
		{Key: flatte.KeyCharacter, Rune: ' '},
		{Key: flatte.KeyCharacter, Rune: ' ', Mod: flatte.ModShift},
		{Key: flatte.KeyCharacter, Rune: 'x', Mod: flatte.ModSuper},
		{Key: flatte.KeyCharacter, Rune: ' ', Mod: flatte.Mod(128) | flatte.ModSuper},
	} {
		if _, _, ok := desktopHotkey(key); ok {
			t.Errorf("unsafe/unsupported key accepted: %+v", key)
		}
	}
	for _, key := range []flatte.Key{flatte.KeyEnter, flatte.KeyEscape, flatte.KeyTab, flatte.KeyF1, flatte.KeyF12} {
		if _, _, ok := desktopHotkey(flatte.KeyEvent{Key: key, Mod: flatte.ModAlt}); !ok {
			t.Errorf("supported key rejected: %v", key)
		}
	}
}
