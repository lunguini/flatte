package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
)

type faceState struct {
	pick    *ds.Theme
	layouts []flatte.LayoutEvent
}

func faceApp(s *faceState) flatte.App[faceState] {
	return flatte.App[faceState]{State: s,
		Handle: func(s *faceState, ev flatte.Event, _ flatte.Effects[faceState]) {
			if l, ok := ev.(flatte.LayoutEvent); ok {
				s.layouts = append(s.layouts, l)
			}
		},
		View: func(s *faceState, _ flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Theme: s.pick, Root: layout.Text{NodeBase: layout.NodeBase{ID: "term", W: layout.Grow(1), H: layout.Grow(1)}, Face: layout.FaceTerminal}}
		}}
}

// A LayoutEvent carries every face's cell, so an app can size a grid set
// in the terminal face from its node's point rect.
func TestHostLayoutReportsFaceCells(t *testing.T) {
	s := &faceState{}
	h, _ := newTestHostWithApp(t, faceApp(s))
	m := cellMetrics
	m.Faces = map[layout.Face]layout.CellSize{layout.FaceTerminal: {Column: 10, Line: 20}}
	h.metrics = m
	h.start(400, 200)
	scene := h.render()
	if len(s.layouts) == 0 {
		t.Fatal("no layout")
	}
	ev := s.layouts[len(s.layouts)-1]
	if ev.Faces[layout.FaceTerminal] != (layout.CellSize{Column: 10, Line: 20}) || ev.Faces[layout.FaceCode] != (layout.CellSize{Column: 8, Line: 16}) {
		t.Fatalf("faces %v", ev.Faces)
	}
	if got := ev.Faces[layout.FaceTerminal].Cells(ev.PointRects["term"]); got.W != 40 || got.H != 10 {
		t.Fatalf("terminal grid %v", got)
	}
	if op, ok := opAt[layout.CellsOp](scene, 0); !ok || op.Face != layout.FaceTerminal || op.Cols != 40 || op.Lines != 10 {
		t.Fatalf("drawn grid %#v", scene.Ops)
	}
	// Without a terminal face, it is the code cell.
	s2 := &faceState{}
	h2, _ := newTestHostWithApp(t, faceApp(s2))
	h2.start(400, 200)
	h2.render()
	if ev := s2.layouts[len(s2.layouts)-1]; ev.Faces[layout.FaceTerminal] != (layout.CellSize{Column: 8, Line: 16}) {
		t.Fatalf("default terminal face %v", ev.Faces)
	}
}

// A theme that changes only the terminal face's cell delivers a layout
// with it, though no rect moved; an unchanged one delivers nothing.
func TestHostRedeliversLayoutWhenAFaceChanges(t *testing.T) {
	s := &faceState{}
	h, _ := newTestHostWithApp(t, faceApp(s))
	h.restyle = func(d *ds.Theme) layout.Metrics {
		m := cellMetrics
		if d.Type.Terminal.Size > 0 {
			m.Faces = map[layout.Face]layout.CellSize{layout.FaceTerminal: {Column: d.Type.Terminal.Size, Line: 2 * d.Type.Terminal.Size}}
		}
		return m
	}
	h.start(400, 200)
	h.render()
	n := len(s.layouts)
	h.render()
	if len(s.layouts) != n {
		t.Fatalf("redelivered an unchanged layout: %d, was %d", len(s.layouts), n)
	}
	big := ds.Default()
	big.Type.Terminal.Size = 20
	s.pick = &big
	h.render()
	if len(s.layouts) != n+1 {
		t.Fatalf("layouts %d, want %d", len(s.layouts), n+1)
	}
	ev := s.layouts[n]
	if ev.Faces[layout.FaceTerminal] != (layout.CellSize{Column: 20, Line: 40}) {
		t.Fatalf("new face %v", ev.Faces)
	}
	if ev.PointRects["term"] != s.layouts[n-1].PointRects["term"] {
		t.Fatal("the rect moved")
	}
	h.render()
	if len(s.layouts) != n+1 {
		t.Fatal("redelivered after the change")
	}
}
