package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
)

type styledState struct {
	pick *ds.Theme
	last flatte.RenderContext
	got  []flatte.ResizeEvent
}

// A frame's Theme reaches the backend's restyle once per change, its
// metrics lay out that very render, and new cell sizes are delivered as a
// resize afterwards.
func TestHostRestylesForFrameTheme(t *testing.T) {
	s := &styledState{}
	app := flatte.App[styledState]{State: s,
		Handle: func(s *styledState, ev flatte.Event, _ flatte.Effects[styledState]) {
			if r, ok := ev.(flatte.ResizeEvent); ok {
				s.got = append(s.got, r)
			}
		},
		View: func(s *styledState, ctx flatte.RenderContext) flatte.Frame {
			s.last = ctx
			return flatte.Frame{Theme: s.pick, Root: layout.Row{Children: []layout.Node{layout.Label{NodeBase: layout.NodeBase{ID: "a", Pad: 1}, Text: "x"}}}}
		}}
	h, b := newTestHostWithApp(t, app)
	var styled []*ds.Theme
	h.restyle = func(d *ds.Theme) layout.Metrics {
		styled = append(styled, d)
		m := cellMetrics
		m.Step, m.Column = d.Space[1], 10
		return m
	}
	h.start(800, 400)
	h.render()
	if len(styled) != 0 {
		t.Fatalf("restyled without a frame theme: %d", len(styled))
	}
	compact := ds.Default().With(ds.Seeds{Density: ds.Compact})
	s.pick = &compact
	scene := h.render()
	if len(styled) != 1 || styled[0].Space != compact.Space {
		t.Fatalf("restyles %d", len(styled))
	}
	if r := scene.Rects["a"]; r.W != 2*compact.Space[1]+10 { // padded "x" in 10pt columns
		t.Fatalf("not laid out in the new metrics: %+v", r)
	}
	if s.last.Theme.Space != compact.Space {
		t.Fatal("view did not see the frame's theme")
	}
	h.render()
	if len(styled) != 1 {
		t.Fatal("restyled an unchanged theme")
	}
	b.drain(t, 1)
	n := len(s.got)
	h.resize(800, 400)
	if len(s.got) != n+1 || s.got[n].Width != 80 {
		t.Fatalf("no resize in the new columns: %+v", s.got)
	}
}
