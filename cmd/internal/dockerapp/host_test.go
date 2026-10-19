package dockerapp

import (
	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// testHost mirrors the runtime for tests that drive Handle directly: every
// event is followed by a layout settle (LayoutEvents delivered before the
// "paint"), and mouse events are hit-tested against the frame on screen. It
// goes through flatte.SettleFrame, the same function Run uses.
type testHost struct {
	delivered map[string]layout.Rect
	comp      layout.Composition
	frame     flatte.Frame
	layouts   int // LayoutEvents delivered, for tests that assert stability
}

// hosts is keyed by state; tests in this package do not run in parallel.
var hosts = map[*State]*testHost{}

func hostFor(s *State) *testHost {
	h, ok := hosts[s]
	if !ok {
		h = &testHost{}
		hosts[s] = h
	}
	return h
}

// size is the terminal size the host composes at: the last resize, or the
// 80×24 default the tests were written against.
func (s *State) testSize() (int, int) {
	w, h := s.width, s.height
	if w <= 0 {
		w = 80
	}
	if h <= 0 {
		h = 24
	}
	return w, h
}

func (h *testHost) settle(s *State, fx flatte.Effects[State]) {
	w, ht := s.testSize()
	h.frame, h.comp, h.delivered = flatte.SettleFrame(
		func() flatte.Frame { return View(s, flatte.RenderContext{Width: w}) },
		func(ev flatte.LayoutEvent) { h.layouts++; Handle(s, ev, fx) },
		h.delivered, w, ht,
	)
}

// send delivers ev as the runtime would and settles the next frame.
func send(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	h := hostFor(s)
	if m, ok := ev.(flatte.MouseEvent); ok {
		m.Target, m.TargetRect = h.comp.HitTest(m.X, m.Y)
		ev = m
	}
	Handle(s, ev, fx)
	h.settle(s, fx)
}

// render returns the frame the runtime would paint for the current state,
// settling layout first (a test may have mutated state directly).
func render(s *State) flatte.Frame {
	h := hostFor(s)
	h.settle(s, flatte.Effects[State]{})
	return h.frame
}
