package layout

import "testing"

// In points a bordered overlay is pulled toward its anchor so its drawn
// border (the band's middle) sits anchorGap below the anchor and lines up
// with its left edge; cells keep the border at the overlay's edge.
func TestBorderedOverlayHugsItsAnchor(t *testing.T) {
	root := func(border BorderKind) Node {
		return Col{Children: []Node{
			Row{NodeBase: NodeBase{Pad: 2}, Children: []Node{Button{NodeBase: NodeBase{ID: "open"}, Label: "Open"}, NewSpacer()}},
			Col{NodeBase: NodeBase{ID: "pop", Overlay: true, Anchor: "open", W: Fixed(20), Box: Box{Border: border}}, Children: []Node{Label{Text: "item"}}},
		}}
	}
	m := Metrics{Column: 8, Line: 16, Step: 8, Border: 12, DefaultPadding: true}
	s := LayoutScene(root(BorderRounded), 400, 300, m)
	a, p := s.Rects["open"], s.Rects["pop"]
	if got, want := p.Y+m.Border/2, a.Y+a.H+anchorGap; got != want {
		t.Errorf("border drawn at y %v, want %v (anchor bottom %v + gap)", got, want, a.Y+a.H)
	}
	if got := p.X + m.Border/2; got != a.X {
		t.Errorf("border drawn at x %v, want the anchor's %v", got, a.X)
	}
	// Without a border nothing is pulled.
	s = LayoutScene(root(BorderNone), 400, 300, m)
	if a, p := s.Rects["open"], s.Rects["pop"]; p.Y != a.Y+a.H || p.X != a.X {
		t.Errorf("borderless overlay moved: %+v under %+v", p, a)
	}
	// Cells: just below, left-aligned, as before.
	c := Compose(root(BorderRounded), 60, 20)
	if a, p := c.Rects["open"], c.Rects["pop"]; p.Y != a.Y+a.H || p.X != a.X {
		t.Errorf("cells: %+v under %+v", p, a)
	}
}
