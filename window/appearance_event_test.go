package window

import (
	"context"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

func TestSurfaceReportsAppearanceChangesToApp(t *testing.T) {
	type state struct{ appearances []flatte.Appearance }
	s := &state{}
	app := flatte.App[state]{
		State: s,
		Handle: func(s *state, ev flatte.Event, _ flatte.Effects[state]) {
			if e, ok := ev.(flatte.AppearanceEvent); ok {
				s.appearances = append(s.appearances, e.Appearance)
			}
		},
		View: func(*state, flatte.RenderContext) flatte.Frame { return flatte.Frame{} },
	}
	session := flatte.NewSession(context.Background(), app)
	defer session.Cancel()
	h := newSessionHost(context.Background(), session, layout.Metrics{Column: 8, Line: 16}, func(f func()) { f() }, func() {}, func() {})
	defer h.close()

	h.surface.setAppearance(flatte.AppearanceLight)
	h.start(320, 240)
	h.surface.setAppearance(flatte.AppearanceLight)
	h.surface.setAppearance(flatte.AppearanceDark)

	want := []flatte.Appearance{flatte.AppearanceLight, flatte.AppearanceDark}
	if len(s.appearances) != len(want) {
		t.Fatalf("appearance events = %v, want %v", s.appearances, want)
	}
	for i := range want {
		if s.appearances[i] != want[i] {
			t.Fatalf("appearance events = %v, want %v", s.appearances, want)
		}
	}
}

func TestSurfaceKeepsFrameAppearance(t *testing.T) {
	type state struct{ a flatte.Appearance }
	s := &state{a: flatte.AppearanceLight}
	app := flatte.App[state]{
		State: s,
		View:  func(s *state, _ flatte.RenderContext) flatte.Frame { return flatte.Frame{Appearance: s.a} },
	}
	session := flatte.NewSession(context.Background(), app)
	defer session.Cancel()
	h := newSessionHost(context.Background(), session, layout.Metrics{Column: 8, Line: 16}, func(f func()) { f() }, func() {}, func() {})
	defer h.close()
	if got := h.surface.frameAppearance(); got != flatte.AppearanceUnknown {
		t.Fatalf("before a frame: %d", got)
	}
	h.start(320, 240)
	h.surface.render()
	if got := h.surface.frameAppearance(); got != flatte.AppearanceLight {
		t.Fatalf("frame appearance = %d, want light", got)
	}
	s.a = flatte.AppearanceUnknown
	h.surface.render()
	if got := h.surface.frameAppearance(); got != flatte.AppearanceUnknown {
		t.Fatalf("frame appearance = %d, want unknown", got)
	}
}
