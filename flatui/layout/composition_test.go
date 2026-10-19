package layout

import (
	"reflect"
	"testing"
)

func hitTree(withModal bool) Node {
	children := []Node{
		Text{NodeBase: NodeBase{ID: "header", H: Fixed(1)}, String: "header"},
		Row{
			NodeBase: NodeBase{ID: "body", H: Grow(1)},
			Children: []Node{
				Text{NodeBase: NodeBase{ID: "sidebar", W: Fixed(10)}, String: "side"},
				Col{NodeBase: NodeBase{W: Grow(1)}, Children: []Node{ // no ID: transparent to hits
					Text{NodeBase: NodeBase{ID: "item", H: Fixed(1)}, String: "item"},
				}},
			},
		},
	}
	if withModal {
		children = append(children, Col{
			NodeBase: NodeBase{Overlay: true, W: Fixed(20), H: Fixed(6)}, // no ID: still blocks
			Children: []Node{
				Text{NodeBase: NodeBase{ID: "ok", H: Fixed(1)}, String: "ok"},
			},
		})
	}
	return Col{Children: children}
}

func TestComposeMatchesSolveAndCompose(t *testing.T) {
	for _, modal := range []bool{false, true} {
		root := hitTree(modal)
		c := Compose(root, 40, 12)
		frame, rects, diags := SolveAndComposeWithDiagnostics(root, 40, 12)
		if c.Frame != frame || !reflect.DeepEqual(c.Rects, rects) || !reflect.DeepEqual(c.Diagnostics, diags) {
			t.Fatalf("modal=%v: Compose diverges from SolveAndComposeWithDiagnostics", modal)
		}
		if !reflect.DeepEqual(c.Rects, Solve(root, 40, 12)) {
			t.Fatalf("modal=%v: Compose rects diverge from Solve", modal)
		}
	}
}

func TestCompositionHitTest(t *testing.T) {
	c := Compose(hitTree(false), 40, 12)
	for _, tc := range []struct {
		x, y int
		want string
	}{
		{0, 0, "header"},
		{39, 0, "header"},
		{0, 1, "sidebar"}, // deepest ID'd node wins over its ID'd parent
		{10, 1, "item"},   // reaches through the ID-less Col
		{10, 2, "body"},   // inside body, below the item: the ID'd ancestor
		{40, 0, ""},       // right of the frame
		{-1, 0, ""},
		{0, 12, ""}, // below the frame
	} {
		if got := c.HitTest(tc.x, tc.y); got != tc.want {
			t.Errorf("HitTest(%d,%d) = %q, want %q", tc.x, tc.y, got, tc.want)
		}
	}
}

func TestCompositionHitTestOverlayBlocksBase(t *testing.T) {
	c := Compose(hitTree(true), 40, 12)
	r, ok := c.Rects["ok"]
	if !ok {
		t.Fatal("overlay child has no rect")
	}
	if got := c.HitTest(r.X, r.Y); got != "ok" {
		t.Errorf("HitTest on overlay child = %q, want ok", got)
	}
	// Inside the overlay but off its ID'd child: the layer absorbs the hit
	// rather than letting it reach the base node painted underneath.
	if got := c.HitTest(r.X, r.Y+2); got != "" {
		t.Errorf("HitTest inside overlay = %q, want \"\"", got)
	}
	// Outside the overlay the base is still reachable.
	if got := c.HitTest(0, 0); got != "header" {
		t.Errorf("HitTest outside overlay = %q, want header", got)
	}
}

func TestCompositionHitTestOverlayRoot(t *testing.T) {
	root := Text{NodeBase: NodeBase{ID: "solo", Overlay: true, W: Fixed(4), H: Fixed(2)}, String: "x"}
	c := Compose(root, 10, 6)
	r := c.Rects["solo"]
	if got := c.HitTest(r.X, r.Y); got != "solo" {
		t.Errorf("HitTest on overlay root = %q, want solo", got)
	}
	if got := c.HitTest(0, 0); got != "" {
		t.Errorf("HitTest outside overlay root = %q, want \"\"", got)
	}
}

func TestCompositionHitTestDuplicateIDsPaintOrder(t *testing.T) {
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "dup", H: Fixed(1)}, String: "a"},
		Text{NodeBase: NodeBase{ID: "dup", H: Fixed(1)}, String: "b"},
	}}
	c := Compose(root, 5, 2)
	if got := c.HitTest(0, 0); got != "dup" {
		t.Errorf("first duplicate = %q, want dup", got)
	}
	if got := c.HitTest(0, 1); got != "dup" {
		t.Errorf("second duplicate = %q, want dup", got)
	}
	if len(c.Diagnostics) != 1 {
		t.Errorf("diagnostics = %d, want 1", len(c.Diagnostics))
	}
}

func TestCompositionEmpty(t *testing.T) {
	for _, c := range []Composition{Compose(hitTree(true), 0, 5), Compose(hitTree(true), 5, -1), {}} {
		if c.Frame != "" || len(c.Rects) != 0 || c.HitTest(0, 0) != "" {
			t.Errorf("empty composition = %+v", c)
		}
	}
}
