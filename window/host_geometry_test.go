package window

import (
	"context"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
)

func TestHostListAdoptUsesMeasuredPointRows(t *testing.T) {
	var list flatui.List
	list.SetCount(20)
	s := &testState{}
	s.root = func(*testState) layout.Node {
		rows := list.Rows("songs", func(int, bool) layout.Node {
			return layout.Label{NodeBase: layout.NodeBase{H: layout.Points(30)}, Text: "song"}
		})
		rows.ID = "songs"
		rows.W, rows.H = layout.Grow(1), layout.Grow(1)
		return rows
	}
	app := testApp(s)
	original := app.Handle
	app.Handle = func(state *testState, event flatte.Event, fx flatte.Effects[testState]) {
		original(state, event, fx)
		if e, ok := event.(flatte.LayoutEvent); ok {
			list.Adopt(e, "songs")
		}
	}
	b := &fakeBackend{}
	h := newHost(context.Background(), app, cellMetrics, b.post, func() {}, func() {})
	t.Cleanup(h.cancel)
	h.start(160, 100)
	h.render()
	if list.Height() != 3 {
		t.Fatalf("native list height %d, rounded cells would incorrectly give 6", list.Height())
	}
	if r := h.scene.Rects["songs:0"]; r.H != 30 {
		t.Fatalf("row geometry %v", r)
	}
	h.render()
	before := s.layouts
	h.resize(160, 100.5)
	h.render()
	if s.layouts != before+1 {
		t.Fatalf("fractional resize layouts %d want %d", s.layouts, before+1)
	}
	event := s.events[len(s.events)-1].(flatte.LayoutEvent)
	if event.PointRects["songs"].H != 100.5 || event.Rects["songs"].H != 6 {
		t.Fatalf("native/cell geometry %+v / %+v", event.PointRects["songs"], event.Rects["songs"])
	}
}

type corruptLayoutTracer struct {
	flatte.NoopTracer
	layouts int
}

func (tr *corruptLayoutTracer) Event(event flatte.Event) {
	if e, ok := event.(flatte.LayoutEvent); ok {
		tr.layouts++
		e.Rects["viewport"] = layout.Rect{H: -1}
		e.PointRects["viewport"] = layout.PointRect{H: -1}
	}
}

func TestHostLayoutGeometryMapsAreOwnedByRecipient(t *testing.T) {
	s := &testState{root: func(*testState) layout.Node {
		return layout.Col{NodeBase: layout.NodeBase{ID: "viewport", H: layout.Grow(1)}}
	}}
	app := testApp(s)
	tr := &corruptLayoutTracer{}
	app.Tracer = tr
	received := 0
	app.Handle = func(_ *testState, event flatte.Event, _ flatte.Effects[testState]) {
		if e, ok := event.(flatte.LayoutEvent); ok {
			received++
			if e.Rects["viewport"].H != 6 || e.PointRects["viewport"].H != 100 {
				t.Fatalf("tracer mutated handler geometry %+v", e)
			}
			e.Rects["viewport"] = layout.Rect{H: -2}
			e.PointRects["viewport"] = layout.PointRect{H: -2}
		}
	}
	b := &fakeBackend{}
	h := newHost(context.Background(), app, cellMetrics, b.post, func() {}, func() {})
	t.Cleanup(h.cancel)
	h.start(160, 100)
	h.render()
	h.render()
	if received != 1 || tr.layouts != 1 {
		t.Fatalf("recipient mutation changed settling: handler %d tracer %d", received, tr.layouts)
	}
	if h.scene.Rects["viewport"].H != 100 {
		t.Fatal("recipient changed native scene geometry")
	}
}
