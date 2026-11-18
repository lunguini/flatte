package window

import (
	"image/color"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
)

// TestSnapshotPaintsFillRoles: a Fill box paints the theme's color for its
// role, in each theme, and an explicit Background wins over Fill.
func TestSnapshotPaintsFillRoles(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	red := lipgloss.Color("#ff0000")
	root := func() layout.Node {
		cell := func(id string, b layout.Box) layout.Node {
			return layout.Col{NodeBase: layout.NodeBase{ID: id, W: layout.Grow(1), H: layout.Grow(1), Box: b}}
		}
		return layout.Row{Children: []layout.Node{
			cell("surface", layout.Box{Fill: layout.FillSurface}),
			cell("field", layout.Box{Fill: layout.FillField}),
			cell("sep", layout.Box{Fill: layout.FillSeparator}),
			cell("both", layout.Box{Fill: layout.FillField, Background: red}),
		}}
	}
	m := metrics()
	for name, th := range map[string]Theme{"dark": Dark(), "light": Light()} {
		img, err := Snapshot(snapApp(root), Options{Width: 40, Height: 6, Theme: &th}, 1)
		if err != nil {
			t.Fatal(err)
		}
		rects := layout.LayoutScene(root(), 40*m.Column, 6*m.Line, m).Rects
		want := map[string]color.Color{"surface": th.Surface, "field": th.Field, "sep": th.palette().Divider, "both": color.RGBA{255, 0, 0, 255}}
		for id, c := range want {
			r := rects[id]
			got := img.At(int(r.X+r.W/2), int(r.Y+r.H/2))
			if !near(got, c, 2) {
				t.Errorf("%s %s: center %v, want %v", name, id, got, c)
			}
		}
	}
}
