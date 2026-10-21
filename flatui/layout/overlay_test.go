package layout

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
)

func TestOverlayCentersLayerOverBase(t *testing.T) {
	base := strings.Join([]string{
		"..........",
		"..........",
		"..........",
	}, "\n")
	layer := "XX\nXX"
	got := Overlay(base, layer)
	lines := strings.Split(got, "\n")
	// 10x3 base, 2x2 layer → origin x=(10-2)/2=4, y=(3-2)/2=0
	if !strings.HasPrefix(lines[0], "....XX") {
		t.Fatalf("row0 = %q, want layer at col 4", lines[0])
	}
	if !strings.HasPrefix(lines[1], "....XX") {
		t.Fatalf("row1 = %q, want layer at col 4", lines[1])
	}
}

func TestOverlayOriginMatchesOverlay(t *testing.T) {
	base := "..........\n..........\n.........."
	layer := "XX\nXX"
	x, y := OverlayOrigin(base, layer)
	if x != 4 || y != 0 {
		t.Fatalf("origin = (%d,%d), want (4,0)", x, y)
	}
}

func TestOverlayEmptyBaseReturnsStyledLayer(t *testing.T) {
	layer := lipgloss.NewStyle().Width(4).Height(2).Foreground(lipgloss.Color("196")).Render("X")
	got := Overlay("", layer)
	if got != layer {
		t.Fatalf("Overlay empty base changed layer styling/content:\ngot:  %q\nwant: %q", got, layer)
	}
	if bounds := uv.NewStyledString(got).Bounds(); bounds.Dx() != 4 || bounds.Dy() != 2 {
		t.Fatalf("layer bounds = %dx%d, want 4x2", bounds.Dx(), bounds.Dy())
	}
}

func TestAnchoredOverlay(t *testing.T) {
	width := 6
	menu := func(anchor string, h int) Node {
		return Col{NodeBase: NodeBase{ID: "menu", Overlay: true, Anchor: anchor, W: Fixed(width), H: Fixed(h)}, Children: []Node{Label{Text: "item"}}}
	}
	root := func(anchor string, h int) Node {
		return Col{Children: []Node{
			Row{Children: []Node{Label{NodeBase: NodeBase{W: Fixed(3)}}, Button{NodeBase: NodeBase{ID: "pick"}, Label: "Pick"}}},
			NewSpacer(),
			Row{Children: []Node{NewSpacer(), Button{NodeBase: NodeBase{ID: "low"}, Label: "Low"}}},
			menu(anchor, h),
		}}
	}
	for _, tc := range []struct {
		anchor string
		h      int
		want   Rect
	}{
		{"pick", 3, Rect{X: 3, Y: 1, W: 6, H: 3}},   // below, left-aligned
		{"low", 3, Rect{X: 13, Y: 6, W: 6, H: 3}},   // no room below: above
		{"gone", 3, Rect{X: 7, Y: 3, W: 6, H: 3}},   // not laid out: centered
		{"pick", 12, Rect{X: 3, Y: 0, W: 6, H: 10}}, // taller than the frame: clipped to it
	} {
		_, rects := SolveAndCompose(root(tc.anchor, tc.h), 20, 10)
		if rects["menu"] != tc.want {
			t.Errorf("anchor %q h %d: menu %+v, want %+v", tc.anchor, tc.h, rects["menu"], tc.want)
		}
	}
	width = 10 // wider than the room right of "low": pulled inside the frame
	if _, rects := SolveAndCompose(root("low", 3), 20, 10); rects["menu"] != (Rect{X: 10, Y: 6, W: 10, H: 3}) {
		t.Errorf("wide menu %+v, want kept inside the frame", rects["menu"])
	}
	width = 6
	// Points place it the same way.
	s := LayoutScene(root("pick", 3), 20*testMetrics.Column, 10*testMetrics.Line, testMetrics)
	if r, p := s.Rects["menu"], s.Rects["pick"]; r.X != p.X || r.Y != p.Y+p.H {
		t.Errorf("points: menu %+v under %+v", r, p)
	}
}

func TestOverlayChildDoesNotSizeItsParent(t *testing.T) {
	// A dropdown: a one-line Row holding a button and its (taller) open list.
	root := Col{Children: []Node{
		Row{NodeBase: NodeBase{ID: "row"}, Children: []Node{
			Button{NodeBase: NodeBase{ID: "b"}, Label: "Pick"},
			Col{NodeBase: NodeBase{Overlay: true, Anchor: "b", W: Fixed(6), H: Fixed(4)}},
		}},
		Label{NodeBase: NodeBase{ID: "next"}, Text: "next"},
	}}
	_, rects := SolveAndCompose(root, 20, 10)
	if rects["row"].H != 1 || rects["next"].Y != 1 {
		t.Fatalf("row %+v next %+v: the overlay sized its parent", rects["row"], rects["next"])
	}
}

func TestOverlayAtGrowsToHoldTheLayer(t *testing.T) {
	got := OverlayAt("ab\ncd", "XYZ", 1, 2)
	if got != "ab\ncd\n XYZ" {
		t.Errorf("OverlayAt = %q", got)
	}
	if got := OverlayAt("abcd", "X", -3, -1); got != "Xbcd" {
		t.Errorf("negative origin clamps: %q", got)
	}
}
