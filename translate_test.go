package flatte

import (
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

func TestTranslateEvent(t *testing.T) {
	cases := []struct {
		name string
		in   uv.Event
		want Event
	}{
		{"letter", uv.KeyPressEvent{Code: 'q', Text: "q"}, KeyEvent{Key: KeyCharacter, Rune: 'q'}},
		{"capital", uv.KeyPressEvent{Code: 'q', Text: "Q", Mod: uv.ModShift}, KeyEvent{Key: KeyCharacter, Rune: 'Q', Mod: ModShift}},
		{"utf8", uv.KeyPressEvent{Code: 'é', Text: "é"}, KeyEvent{Key: KeyCharacter, Rune: 'é'}},
		{"space", uv.KeyPressEvent{Code: uv.KeySpace, Text: " "}, KeyEvent{Key: KeyCharacter, Rune: ' '}},
		{"ctrl-c", uv.KeyPressEvent{Code: 'c', Mod: uv.ModCtrl}, KeyEvent{Key: KeyCtrlC, Mod: ModCtrl}},
		{"ctrl-z", uv.KeyPressEvent{Code: 'z', Mod: uv.ModCtrl}, KeyEvent{Key: KeyCharacter, Rune: 'z', Mod: ModCtrl}},
		{"enter", uv.KeyPressEvent{Code: uv.KeyEnter}, KeyEvent{Key: KeyEnter}},
		{"tab", uv.KeyPressEvent{Code: uv.KeyTab}, KeyEvent{Key: KeyTab}},
		{"escape", uv.KeyPressEvent{Code: uv.KeyEscape}, KeyEvent{Key: KeyEscape}},
		{"backspace", uv.KeyPressEvent{Code: uv.KeyBackspace}, KeyEvent{Key: KeyBackspace}},
		{"delete", uv.KeyPressEvent{Code: uv.KeyDelete}, KeyEvent{Key: KeyDelete}},
		{"up", uv.KeyPressEvent{Code: uv.KeyUp}, KeyEvent{Key: KeyUp}},
		{"down", uv.KeyPressEvent{Code: uv.KeyDown}, KeyEvent{Key: KeyDown}},
		{"left", uv.KeyPressEvent{Code: uv.KeyLeft}, KeyEvent{Key: KeyLeft}},
		{"right", uv.KeyPressEvent{Code: uv.KeyRight}, KeyEvent{Key: KeyRight}},
		{"home", uv.KeyPressEvent{Code: uv.KeyHome}, KeyEvent{Key: KeyHome}},
		{"end", uv.KeyPressEvent{Code: uv.KeyEnd}, KeyEvent{Key: KeyEnd}},
		{"keypad-home", uv.KeyPressEvent{Code: uv.KeyKpHome}, KeyEvent{Key: KeyHome}},
		{"keypad-end", uv.KeyPressEvent{Code: uv.KeyKpEnd}, KeyEvent{Key: KeyEnd}},
		{"alt-arrow", uv.KeyPressEvent{Code: uv.KeyUp, Mod: uv.ModAlt}, KeyEvent{Key: KeyUp, Mod: ModAlt}},
		{"ctrl-arrow", uv.KeyPressEvent{Code: uv.KeyRight, Mod: uv.ModCtrl}, KeyEvent{Key: KeyRight, Mod: ModCtrl}},
		{"ctrl-alt-arrow", uv.KeyPressEvent{Code: uv.KeyLeft, Mod: uv.ModCtrl | uv.ModAlt}, KeyEvent{Key: KeyLeft, Mod: ModAlt | ModCtrl}},
		{"paste", uv.PasteEvent{Content: "hello"}, PasteEvent{Text: "hello"}},
		{"focus", uv.FocusEvent{}, FocusEvent{Focused: true}},
		{"blur", uv.BlurEvent{}, FocusEvent{Focused: false}},
		{"resize", uv.WindowSizeEvent{Width: 80, Height: 24}, ResizeEvent{Width: 80, Height: 24}},
		{"click", uv.MouseClickEvent{X: 3, Y: 4, Button: uv.MouseLeft}, MouseEvent{X: 3, Y: 4, Button: MouseLeft, Action: MousePress}},
		{"release", uv.MouseReleaseEvent{X: 3, Y: 4, Button: uv.MouseLeft}, MouseEvent{X: 3, Y: 4, Button: MouseLeft, Action: MouseRelease}},
		{"motion", uv.MouseMotionEvent{X: 5, Y: 6}, MouseEvent{X: 5, Y: 6, Button: MouseNone, Action: MouseMotion}},
		{"wheel", uv.MouseWheelEvent{Button: uv.MouseWheelDown}, MouseEvent{Button: MouseWheelDown, Action: MousePress}},
		{"f1", uv.KeyPressEvent{Code: uv.KeyF1}, KeyEvent{Key: KeyF1}},
		{"f2", uv.KeyPressEvent{Code: uv.KeyF2}, KeyEvent{Key: KeyF2}},
		{"f3", uv.KeyPressEvent{Code: uv.KeyF3}, KeyEvent{Key: KeyF3}},
		{"f4", uv.KeyPressEvent{Code: uv.KeyF4}, KeyEvent{Key: KeyF4}},
		{"f5", uv.KeyPressEvent{Code: uv.KeyF5}, KeyEvent{Key: KeyF5}},
		{"f6", uv.KeyPressEvent{Code: uv.KeyF6}, KeyEvent{Key: KeyF6}},
		{"f7", uv.KeyPressEvent{Code: uv.KeyF7}, KeyEvent{Key: KeyF7}},
		{"f8", uv.KeyPressEvent{Code: uv.KeyF8}, KeyEvent{Key: KeyF8}},
		{"f9", uv.KeyPressEvent{Code: uv.KeyF9}, KeyEvent{Key: KeyF9}},
		{"f10", uv.KeyPressEvent{Code: uv.KeyF10}, KeyEvent{Key: KeyF10}},
		{"f11", uv.KeyPressEvent{Code: uv.KeyF11}, KeyEvent{Key: KeyF11}},
		{"f12", uv.KeyPressEvent{Code: uv.KeyF12}, KeyEvent{Key: KeyF12}},
		{"clipboard", uv.ClipboardEvent{Selection: uv.SystemClipboard, Content: "hello"}, ClipboardEvent{Text: "hello"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := translateEvent(tc.in)
			if !ok {
				t.Fatalf("translateEvent(%#v) dropped", tc.in)
			}
			if got != tc.want {
				t.Fatalf("translateEvent(%#v) = %#v, want %#v", tc.in, got, tc.want)
			}
		})
	}
}

func TestTranslateEventDropsUnmappedEvents(t *testing.T) {
	for _, in := range []uv.Event{
		uv.KeyReleaseEvent{Code: 'q'},
	} {
		if got, ok := translateEvent(in); ok {
			t.Fatalf("translateEvent(%#v) = %#v, want dropped", in, got)
		}
	}
}
