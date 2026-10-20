package window

import (
	"reflect"
	"testing"

	"github.com/lunguini/flatte"
)

func TestMacKey(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chars rune
		flags uint
		want  flatte.KeyEvent
		ok    bool
	}{
		{"up", 0xF700, 0, flatte.KeyEvent{Key: flatte.KeyUp}, true},
		{"shift-left", 0xF702, macShift, flatte.KeyEvent{Key: flatte.KeyLeft, Mod: flatte.ModShift}, true},
		{"option-right", 0xF703, macOption, flatte.KeyEvent{Key: flatte.KeyRight, Mod: flatte.ModAlt}, true},
		{"f12", 0xF70F, 0, flatte.KeyEvent{Key: flatte.KeyF12}, true},
		{"page down", 0xF72D, 0, flatte.KeyEvent{Key: flatte.KeyPageDown}, true},
		{"forward delete", 0xF728, 0, flatte.KeyEvent{Key: flatte.KeyDelete}, true},
		{"return", '\r', 0, flatte.KeyEvent{Key: flatte.KeyEnter}, true},
		{"keypad enter", 0x03, 0, flatte.KeyEvent{Key: flatte.KeyEnter}, true},
		{"backtab", 0x19, macShift, flatte.KeyEvent{Key: flatte.KeyTab, Mod: flatte.ModShift}, true},
		{"backtab without shift flag", 0x19, 0, flatte.KeyEvent{Key: flatte.KeyTab, Mod: flatte.ModShift}, true},
		{"escape", 0x1b, 0, flatte.KeyEvent{Key: flatte.KeyEscape}, true},
		{"backspace", 0x7f, 0, flatte.KeyEvent{Key: flatte.KeyBackspace}, true},
		{"ctrl-c", 'c', macControl, flatte.KeyEvent{Key: flatte.KeyCtrlC, Mod: flatte.ModCtrl}, true},
		{"ctrl-shift-C", 'C', macControl | macShift, flatte.KeyEvent{Key: flatte.KeyCtrlC, Mod: flatte.ModCtrl | flatte.ModShift}, true},
		{"ctrl-w", 'w', macControl, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'w', Mod: flatte.ModCtrl}, true},
		{"plain letter is text", 'j', 0, flatte.KeyEvent{}, false},
		{"option letter is text", 'b', macOption, flatte.KeyEvent{}, false},
		{"command chord is the menu's", 'q', macCommand, flatte.KeyEvent{}, false},
		{"command arrow is the menu's", 0xF700, macCommand, flatte.KeyEvent{}, false},
	} {
		got, ok := macKey(tc.chars, tc.flags)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: macKey = %+v %v, want %+v %v", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestTextKeys(t *testing.T) {
	got := textKeys("aé日\n\x07")
	want := []flatte.KeyEvent{
		{Key: flatte.KeyCharacter, Rune: 'a'},
		{Key: flatte.KeyCharacter, Rune: 'é'},
		{Key: flatte.KeyCharacter, Rune: '日'},
		{Key: flatte.KeyEnter},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("textKeys = %+v, want %+v", got, want)
	}
}
