package main

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func key(k flatte.Key) flatte.KeyEvent { return flatte.KeyEvent{Key: k} }

func space() flatte.KeyEvent { return flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: ' '} }

func send(s *State, evs ...flatte.Event) {
	for _, ev := range evs {
		Handle(s, ev, flatte.Effects[State]{})
	}
}

func TestKeyboardDrivesEveryControl(t *testing.T) {
	s := NewState()
	send(s, flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '!'})
	if s.name.Value != "Ada!" {
		t.Fatalf("typing in the name field: %q", s.name.Value)
	}
	send(s, key(flatte.KeyTab), space()) // notifications off
	if s.settings.Notifications {
		t.Fatal("space did not toggle the switch")
	}
	send(s, key(flatte.KeyTab), space()) // sounds is disabled now
	if !s.settings.Sounds {
		t.Fatal("a disabled checkbox changed")
	}
	send(s, key(flatte.KeyTab), key(flatte.KeyRight), key(flatte.KeyRight))
	if s.settings.Theme != 2 {
		t.Fatalf("theme %d, want Dark", s.settings.Theme)
	}
	send(s, key(flatte.KeyTab), key(flatte.KeyLeft), key(flatte.KeyLeft))
	if v := s.settings.Volume; v < 0.49 || v > 0.51 {
		t.Fatalf("volume %v, want 0.5", v)
	}
	send(s, key(flatte.KeyTab), key(flatte.KeyEnter), key(flatte.KeyDown), key(flatte.KeyDown), key(flatte.KeyEnter))
	if s.language.Value() != "Deutsch" || s.language.IsOpen() {
		t.Fatalf("language %q open %v", s.language.Value(), s.language.IsOpen())
	}
	if !s.dirty() {
		t.Fatal("changes not dirty")
	}
	send(s, key(flatte.KeyTab), key(flatte.KeyTab), space()) // save
	if s.dirty() || s.status != "Saved." {
		t.Fatalf("after save: dirty %v status %q", s.dirty(), s.status)
	}
	send(s, flatte.KeyEvent{Key: flatte.KeyTab, Mod: flatte.ModShift}, key(flatte.KeyEnter)) // reset
	if s.current() != defaults() || !s.dirty() {
		t.Fatalf("reset: %+v dirty %v", s.current(), s.dirty())
	}
}

func TestEscapeClosesTheDropdownBeforeQuitting(t *testing.T) {
	s := NewState()
	quit := false
	fx := flatte.NewHarnessEffects[State](t.Context(), nil, func() { quit = true }, nil, nil)
	s.language.Open()
	Handle(s, key(flatte.KeyEscape), fx)
	if quit || s.language.IsOpen() {
		t.Fatalf("esc with the list open: quit %v open %v", quit, s.language.IsOpen())
	}
	Handle(s, key(flatte.KeyEscape), fx)
	if !quit {
		t.Fatal("esc did not quit")
	}
}

func TestMouseDrivesEveryControl(t *testing.T) {
	d := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 64)
	d.Send(flatte.ResizeEvent{Width: 64, Height: 22})
	s := d.State()
	click := func(id string, dx int) {
		t.Helper()
		_, comp := flatte.ComposeFrame(d.Frame(), 64, 22)
		r, ok := comp.Rects[id]
		if !ok {
			t.Fatalf("no rect for %q", id)
		}
		d.Send(flatte.MouseEvent{X: r.X + dx, Y: r.Y, Button: flatte.MouseLeft, Action: flatte.MousePress})
		d.Send(flatte.MouseEvent{X: r.X + dx, Y: r.Y, Button: flatte.MouseLeft, Action: flatte.MouseRelease})
	}
	click("notify", 1)
	if s.settings.Notifications || !s.focus.Focused(fieldNotify) {
		t.Fatalf("switch click: on %v focus %d", s.settings.Notifications, s.focus.Index())
	}
	click("theme:light", 1)
	if s.settings.Theme != 1 {
		t.Fatalf("radio click: theme %d", s.settings.Theme)
	}
	click("volume", 0)
	if s.settings.Volume != 0 {
		t.Fatalf("slider press at its start: %v", s.settings.Volume)
	}
	// A drag past the slider's end ends at 1.
	_, comp := flatte.ComposeFrame(d.Frame(), 64, 22)
	r := comp.Rects["volume"]
	d.Send(flatte.MouseEvent{X: r.X, Y: r.Y, Button: flatte.MouseLeft, Action: flatte.MousePress})
	d.Send(flatte.MouseEvent{X: r.X + r.W + 5, Y: r.Y + 3, Button: flatte.MouseLeft, Action: flatte.MouseMotion})
	d.Send(flatte.MouseEvent{X: r.X + r.W + 5, Y: r.Y + 3, Button: flatte.MouseLeft, Action: flatte.MouseRelease})
	if s.settings.Volume != 1 || s.dragging {
		t.Fatalf("slider drag: %v dragging %v", s.settings.Volume, s.dragging)
	}
	click("language", 1)
	if !s.language.IsOpen() {
		t.Fatal("dropdown click did not open it")
	}
	click("language:option:3", 2)
	if s.language.Value() != "Español" || s.language.IsOpen() {
		t.Fatalf("option click: %q open %v", s.language.Value(), s.language.IsOpen())
	}
	click("name", 1)
	if !s.focus.Focused(fieldName) || s.name.Cursor != 1 {
		t.Fatalf("name click: focus %d cursor %d", s.focus.Index(), s.name.Cursor)
	}
	click("save", 2)
	if s.dirty() {
		t.Fatal("save click did not save")
	}
}

func TestSettingsSnapshot(t *testing.T) {
	s := NewState()
	f, _ := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: 64}), 64, 22)
	s.language.Open()
	open, _ := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: 64}), 64, 22)
	flatest.AssertFrames(t, "testdata/settings.golden", []flatte.Frame{f, open})
}
