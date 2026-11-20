package layout

import (
	"image/color"
	"testing"

	"github.com/lunguini/flatte/theme"
)

// A hairline splitter is grabbed a few points either side of its box, over
// the panes beside it, but not through an overlay above it.
func TestSplitterGripWidensItsHitArea(t *testing.T) {
	pane := func(id string, w Size) Node {
		return Col{NodeBase: NodeBase{ID: id, W: w, H: Grow(1)}}
	}
	split := Col{NodeBase: NodeBase{ID: "split", W: Points(1), H: Grow(1), Box: Box{Resize: ResizeColumns, Fill: FillSeparator}}}
	row := Row{Children: []Node{pane("left", Points(50)), split, pane("right", Grow(1))}}
	s := LayoutScene(row, 200, 100, Metrics{})
	for _, tc := range []struct {
		x    float64
		want string
	}{{40, "left"}, {47.5, "split"}, {50.5, "split"}, {53.5, "split"}, {55, "right"}} {
		if id, _ := s.HitTest(tc.x, 50); id != tc.want {
			t.Errorf("x=%v: %q, want %q", tc.x, id, tc.want)
		}
	}
	if _, r := s.HitTest(53.5, 50); r.X != 50 || r.W != 1 {
		t.Errorf("grip reports the area, not the box: %+v", r)
	}

	rows := Col{Children: []Node{pane("top", Points(0)), Col{NodeBase: NodeBase{ID: "hsplit", W: Grow(1), H: Points(1), Box: Box{Resize: ResizeRows}}}, Col{NodeBase: NodeBase{ID: "bottom", W: Grow(1), H: Grow(1)}}}}
	rows.Children[0] = Col{NodeBase: NodeBase{ID: "top", W: Grow(1), H: Points(40)}}
	s = LayoutScene(rows, 100, 100, Metrics{})
	if id, _ := s.HitTest(50, 43); id != "hsplit" {
		t.Errorf("below a row splitter: %q", id)
	}

	over := Col{Children: []Node{row, Col{NodeBase: NodeBase{ID: "popover", W: Points(100), H: Points(40), Overlay: true}}}}
	s = LayoutScene(over, 200, 100, Metrics{})
	if id, _ := s.HitTest(52, 50); id != "popover" {
		t.Errorf("a grip under an overlay: %q", id)
	}
}

// Cells have no grip: a terminal's splitter is a whole cell already.
func TestCellSplitterHasNoGrip(t *testing.T) {
	split := Col{NodeBase: NodeBase{ID: "split", W: Fixed(1), H: Grow(1), Box: Box{Resize: ResizeColumns}}}
	row := Row{Children: []Node{Col{NodeBase: NodeBase{ID: "left", W: Fixed(5), H: Grow(1)}}, split, Col{NodeBase: NodeBase{ID: "right", W: Grow(1), H: Grow(1)}}}}
	_, rects := SolveAndCompose(row, 20, 3)
	if r := rects["split"]; r.X != 5 || r.W != 1 {
		t.Fatalf("split rect %+v", r)
	}
}

// Glass is white in both appearances: mostly opaque on light, a faint
// wash on dark (the dark content surface at 80% read as black).
func TestGlassIsWhite(t *testing.T) {
	d := theme.Default()
	for p, a := range map[*theme.Palette]uint8{&d.Light: GlassLight, &d.Dark: GlassDark} {
		if c, ok := FillColor(p, FillGlass); !ok || c != (color.NRGBA{0xff, 0xff, 0xff, a}) {
			t.Errorf("glass %v, want white at %#x", c, a)
		}
	}
}
