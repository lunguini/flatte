package window

import (
	"testing"

	"github.com/lunguini/flatte"
)

type fakeDesktop struct{ commands []flatte.DesktopCommand }

func (f *fakeDesktop) command(c flatte.DesktopCommand) { f.commands = append(f.commands, c) }

func TestDesktopCommandsAreDeferredAndOwned(t *testing.T) {
	s := &testState{}
	key := flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: ' ', Mod: flatte.ModSuper | flatte.ModShift}
	app := testApp(s)
	app.Handle = func(_ *testState, ev flatte.Event, fx flatte.Effects[testState]) {
		if k, ok := ev.(flatte.KeyEvent); ok && k.Rune == 'd' {
			fx.Desktop(flatte.DesktopCommand{Kind: "shortcut", Shortcut: &key})
		}
	}
	h, b := newTestHostWithApp(t, app)
	d := &fakeDesktop{}
	h.desktop = d
	h.start(80, 24)
	h.key(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'd'})
	key.Rune = 'x'
	if len(d.commands) != 0 {
		t.Fatal("desktop command ran inside handler")
	}
	b.drain(t, 1)
	if len(d.commands) != 1 || d.commands[0].Shortcut.Rune != ' ' {
		t.Fatalf("commands %+v", d.commands)
	}
}

func TestDesktopCloseHidesAndDeliversEventWithoutQuitting(t *testing.T) {
	s := &testState{content: "private selection"}
	h, b := newTestHost(t, s)
	d := &fakeDesktop{}
	h.desktop = d
	h.start(300, 200)
	h.render()
	s.events = nil
	h.closeRequested("")
	if len(d.commands) != 1 || d.commands[0].Kind != "hide" {
		t.Fatalf("commands %+v", d.commands)
	}
	if len(s.events) != 1 || s.events[0] != (flatte.DesktopEvent{ID: "close", Source: "window"}) {
		t.Fatalf("events %#v", s.events)
	}
	if h.quitting || b.stops != 0 {
		t.Fatal("desktop close quit app")
	}
	if len(h.scene.Ops) != 0 {
		t.Fatal("hidden scene retains content")
	}
}

func TestDesktopMenusFollowStateWhileHidden(t *testing.T) {
	h, s, m := menuHost(t)
	h.desktop = &fakeDesktop{}
	before := len(m.bars)
	s.content = "Updated from background"
	h.after()
	if len(m.bars) != before+1 || m.bars[len(m.bars)-1][0].Items[0].Label != s.content {
		t.Fatalf("bars %#v", m.bars)
	}
}
