package flatte

import (
	"reflect"
	"testing"
)

func TestTranslateBrowserKeyPreservesModifiers(t *testing.T) {
	if got := browserModifierMask(browserModifiers{Shift: true, Alt: true, Ctrl: true, Super: true}); got != ModShift|ModAlt|ModCtrl|ModSuper {
		t.Fatalf("browser modifier mask = %v, want all four modifiers", got)
	}
	for _, tc := range []struct {
		name string
		in   browserKey
		want KeyEvent
		ok   bool
	}{
		{"ctrl shortcut", browserKey{Name: "s", Mod: ModCtrl}, KeyEvent{Key: KeyCharacter, Rune: 's', Mod: ModCtrl}, true},
		{"command shortcut", browserKey{Name: "s", Mod: ModSuper}, KeyEvent{Key: KeyCharacter, Rune: 's', Mod: ModSuper}, true},
		{"shifted arrow", browserKey{Name: "ArrowLeft", Mod: ModSuper | ModShift}, KeyEvent{Key: KeyLeft, Mod: ModSuper | ModShift}, true},
		{"ctrl c", browserKey{Name: "c", Mod: ModCtrl}, KeyEvent{Key: KeyCtrlC, Mod: ModCtrl}, true},
		{"alt graph text", browserKey{Name: "@", Mod: ModCtrl | ModAlt, AltGraph: true}, KeyEvent{}, false},
		{"plain text waits for input", browserKey{Name: "é"}, KeyEvent{}, false},
		{"function key", browserKey{Name: "F12", Mod: ModAlt}, KeyEvent{Key: KeyF12, Mod: ModAlt}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := translateBrowserKey(tc.in)
			if got != tc.want || ok != tc.ok {
				t.Fatalf("translateBrowserKey(%#v) = %#v, %v; want %#v, %v", tc.in, got, ok, tc.want, tc.ok)
			}
		})
	}
}

func TestBrowserPasteTranslation(t *testing.T) {
	if !browserPasteGesture(browserKey{Name: "v", Mod: ModSuper}) ||
		!browserPasteGesture(browserKey{Name: "V", Mod: ModCtrl | ModShift}) ||
		browserPasteGesture(browserKey{Name: "v", Mod: ModCtrl | ModAlt, AltGraph: true}) {
		t.Fatal("browser paste gesture modifier mapping is incorrect")
	}
	if got, want := browserTextEvents("hello\n界", true), []Event{PasteEvent{Text: "hello\n界"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("bracketed paste = %#v, want %#v", got, want)
	}
	if got, want := browserTextEvents("a\r\n界", false), []Event{
		KeyEvent{Key: KeyCharacter, Rune: 'a'},
		KeyEvent{Key: KeyEnter},
		KeyEvent{Key: KeyCharacter, Rune: '界'},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("unbracketed paste = %#v, want %#v", got, want)
	}
}

func TestBrowserWheelAccumulatorAndCellCoordinates(t *testing.T) {
	var wheel browserWheelAccumulator
	for _, tc := range []struct {
		delta float64
		want  int
	}{{8, 0}, {8, 1}, {-24, -1}, {8, 0}} {
		if got := wheel.notches(tc.delta, 16); got != tc.want {
			t.Fatalf("notches(%v) = %d, want %d", tc.delta, got, tc.want)
		}
	}
	if got := wheel.notches(16, 0); got != 0 {
		t.Fatalf("notches with invalid line height = %d, want 0", got)
	}
	x, y := browserCellCoordinates(43, 63, 10, 20, 8, 4, 8, 4, 8, 16, 5, 3)
	if x != 4 || y != 2 {
		t.Fatalf("browser cell = (%d,%d), want (4,2)", x, y)
	}
	x, y = browserCellCoordinates(-20, -30, 0, 0, 0, 0, 8, 4, 8, 16, 0, 0)
	if x != 0 || y != 0 {
		t.Fatalf("negative browser cell = (%d,%d), want (0,0)", x, y)
	}
	for value, want := range map[int]MouseButton{0: MouseLeft, 1: MouseMiddle, 2: MouseRight, 3: MouseBack, 4: MouseForward, 5: MouseNone} {
		if got := browserMouseButtonValue(value); got != want {
			t.Errorf("browser mouse button %d = %v, want %v", value, got, want)
		}
	}
}
