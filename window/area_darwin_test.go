package window

import (
	"image/color"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

// over is c composited over base.
func over(c, base rgba) color.Color {
	return toColor(rgba{c.R*c.A + base.R*(1-c.A), c.G*c.A + base.G*(1-c.A), c.B*c.A + base.B*(1-c.A), 1})
}

func TestSnapshotTextArea(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	value := "first line\nsecond\nthird\nfourth\nfifth"
	area := func(caret, selStart, selEnd int) func() layout.Node {
		return func() layout.Node {
			return layout.Col{Children: []layout.Node{
				layout.Input{NodeBase: layout.NodeBase{ID: "ed", H: layout.Fixed(3)}, Value: value, Caret: caret,
					SelStart: selStart, SelEnd: selEnd, Focused: true, Multiline: true, Wrap: true},
			}}
		}
	}
	shot := func(root func() layout.Node) (func(layout.PointRect, color.Color, uint32) bool, layout.PointRect) {
		img, err := Snapshot(snapApp(root), Options{Width: 30, Height: 4}, 2)
		if err != nil {
			t.Fatal(err)
		}
		r := layout.LayoutScene(root(), 30*m.Column, 4*m.Line, m).Rects["ed"]
		return func(pr layout.PointRect, c color.Color, tol uint32) bool { return region(img, pr, 2, c, tol) }, r
	}
	rowRect := func(r layout.PointRect, i int, x0, x1 float64) layout.PointRect {
		return layout.PointRect{X: r.X + x0, Y: r.Y + float64(i)*m.Line + 2, W: x1 - x0, H: m.Line - 4}
	}
	accent := toColor(darkTheme.caret)

	// The caret in "fifth" (row 4 of 5) scrolls three rows into view:
	// third, fourth, fifth — the caret on the last visible row.
	has, r := shot(area(len(value), 0, 0))
	if !has(rowRect(r, 2, fieldInset, 6*m.Column), accent, 40) {
		t.Error("caret not on the last visible row")
	}
	for i := range 2 {
		if has(rowRect(r, i, fieldInset, r.W-fieldInset), accent, 40) {
			t.Errorf("caret drawn on row %d too", i)
		}
	}

	// At the top, with "first" selected: highlighted on row 0, not row 1.
	has, r = shot(area(0, 0, 5))
	sel := over(darkTheme.selection, darkTheme.field)
	if !has(rowRect(r, 0, fieldInset+2, fieldInset+12), sel, 6) {
		t.Error("selection not highlighted on the first row")
	}
	if has(rowRect(r, 1, fieldInset+2, r.W-fieldInset), sel, 3) {
		t.Error("selection highlighted on the second row")
	}
	if !has(rowRect(r, 0, 0, fieldInset+3), accent, 40) {
		t.Error("caret not at the start of the first row")
	}
}
