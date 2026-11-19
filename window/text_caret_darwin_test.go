package window

import (
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

// TestSnapshotDrawsTextCaret: a Text caret shows as the window's caret bar
// at its cell's left edge, in the accent color, and nowhere without one.
func TestSnapshotDrawsTextCaret(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	root := func(c *layout.Caret) func() layout.Node {
		return func() layout.Node {
			return layout.Text{NodeBase: layout.NodeBase{ID: "code"}, String: "abcdef\nghijkl", Caret: c}
		}
	}
	m := metrics()
	accent := toColor(darkTheme.caret)
	img, err := Snapshot(snapApp(root(&layout.Caret{X: 3, Y: 1})), Options{Width: 20, Height: 4}, 2)
	if err != nil {
		t.Fatal(err)
	}
	bar := layout.PointRect{X: 3*m.Column - 1, Y: m.Line + 2, W: 3, H: m.Line - 4}
	if !region(img, bar, 2, accent, 40) {
		t.Fatal("no caret bar at column 3 of line 1")
	}
	img, err = Snapshot(snapApp(root(nil)), Options{Width: 20, Height: 4}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if region(img, layout.PointRect{W: 20 * m.Column, H: 4 * m.Line}, 2, accent, 20) {
		t.Fatal("caret drawn without a Caret")
	}
}

// TestSnapshotDrawsExtraCarets: Text.Carets draw as further caret bars.
func TestSnapshotDrawsExtraCarets(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	root := func() layout.Node {
		return layout.Text{String: "abcdef\nghijkl\nmnopqr", Caret: &layout.Caret{X: 1}, Carets: []layout.Caret{{X: 4, Y: 1}, {X: 2, Y: 2}}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 20, Height: 4}, 2)
	if err != nil {
		t.Fatal(err)
	}
	accent := toColor(darkTheme.caret)
	for _, c := range []layout.Caret{{X: 1}, {X: 4, Y: 1}, {X: 2, Y: 2}} {
		bar := layout.PointRect{X: float64(c.X)*m.Column - 1, Y: float64(c.Y)*m.Line + 2, W: 3, H: m.Line - 4}
		if !region(img, bar, 2, accent, 40) {
			t.Errorf("no caret bar at %+v", c)
		}
	}
}
