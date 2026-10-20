package layout

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestIconIsItsTextInTheTerminal(t *testing.T) {
	style := TextStyle{Fg: lipgloss.Color("6"), Bold: true}
	icon := Row{Children: []Node{Icon{Name: IconChevronRight, Text: "> ", Style: style}, Label{Text: "item"}}}
	label := Row{Children: []Node{Label{Text: "> ", Style: style}, Label{Text: "item"}}}
	if got, want := Compose(icon, 10, 1).Frame, Compose(label, 10, 1).Frame; got != want {
		t.Fatalf("icon row %q, want the label row %q", got, want)
	}
	w, h := Icon{Name: IconDot, Text: "●"}.Size()
	if w != (Size{Kind: SizeContent, Value: 1}) || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Fatalf("Icon size = %+v × %+v, want its text's cells", w, h)
	}
}

func TestBoxSelectedIsInvisibleInTheTerminal(t *testing.T) {
	// Selection's terminal look is the app's (markers, styles); Selected
	// only tells a GUI to highlight.
	row := func(sel bool) Node {
		return Row{NodeBase: NodeBase{ID: "r", Box: Box{Selected: sel}}, Children: []Node{Label{Text: "> item"}}}
	}
	on, off := Compose(row(true), 10, 1), Compose(row(false), 10, 1)
	if on.Frame != off.Frame {
		t.Fatalf("Selected changed the terminal frame: %q vs %q", on.Frame, off.Frame)
	}
}

func TestSceneIconAndSelection(t *testing.T) {
	root := Col{Children: []Node{
		Row{NodeBase: NodeBase{ID: "row", Box: Box{Selected: true}}, Children: []Node{
			Icon{NodeBase: NodeBase{ID: "icon"}, Name: IconDisclosureOpen, Text: "▾ ", Style: TextStyle{Role: TextSubtle}},
			Label{Text: "folder"},
		}},
	}}
	s := LayoutScene(root, 200, 100, testMetrics)
	var box *BoxOp
	var icon *IconOp
	for _, op := range s.Ops {
		switch op := op.(type) {
		case BoxOp:
			box = &op
		case IconOp:
			icon = &op
		}
	}
	if box == nil || !box.Box.Selected || box.Rect != s.Rects["row"] {
		t.Fatalf("selected row box op = %+v, want one over the row %+v", box, s.Rects["row"])
	}
	// The icon takes its text's columns, so it lines up with the terminal.
	want := PointRect{W: 2 * testMetrics.Column, H: testMetrics.Line}
	if icon == nil || icon.Rect != want || icon.Name != IconDisclosureOpen || icon.Text != "▾ " || icon.Style.Role != TextSubtle {
		t.Fatalf("icon op = %+v, want %v at %+v", icon, IconDisclosureOpen, want)
	}
	if s.Rects["icon"] != want {
		t.Fatalf("icon rect = %+v, want %+v", s.Rects["icon"], want)
	}
}
