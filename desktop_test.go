package flatte

import "testing"

func TestDesktopEffectsAreOptionalAndOwnShortcut(t *testing.T) {
	var fx Effects[struct{}]
	fx.Desktop(DesktopCommand{Kind: "show"}) // terminal and zero effects safe
	var got []DesktopCommand
	fx = WithHostDesktop(fx, func(c DesktopCommand) { got = append(got, c) })
	key := KeyEvent{Key: KeyCharacter, Rune: ' ', Mod: ModSuper | ModShift}
	fx.Desktop(DesktopCommand{Kind: "shortcut", Shortcut: &key})
	key.Rune = 'x'
	if len(got) != 1 || got[0].Kind != "shortcut" || got[0].Shortcut.Rune != ' ' {
		t.Fatalf("desktop command did not own shortcut: %#v", got)
	}
}

func TestDesktopEventsAreFrameworkInputs(t *testing.T) {
	var ev Event = DesktopEvent{ID: "capture", Source: "service", Text: "explicit selection"}
	if got := ev.(DesktopEvent); got.ID != "capture" || got.Source != "service" {
		t.Fatal(got)
	}
}
