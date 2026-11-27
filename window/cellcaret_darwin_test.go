package window

import (
	"image/color"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// Frame.CursorStyle shapes a cell grid's caret: a bar by default, a
// partial block over the whole cell, or an underline at its foot.
func TestSnapshotShapesCellCaret(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	red := color.NRGBA{0xff, 0, 0, 0xff}
	type probe struct{ fx, fy float64 } // within the caret's cell, 0..1
	for _, tc := range []struct {
		name     string
		style    *flatte.CursorStyle
		red, not []probe
	}{
		{"bar", &flatte.CursorStyle{Color: red}, []probe{{0.05, 0.5}}, []probe{{0.6, 0.5}, {0.6, 0.97}}},
		{"block", &flatte.CursorStyle{Shape: flatte.CursorShapeBlock, Color: red}, []probe{{0.6, 0.3}, {0.6, 0.97}}, nil},
		{"underline", &flatte.CursorStyle{Shape: flatte.CursorShapeUnderline, Color: red}, []probe{{0.6, 0.97}}, []probe{{0.6, 0.4}}},
	} {
		type st struct{}
		app := flatte.App[st]{State: &st{}, View: func(*st, flatte.RenderContext) flatte.Frame {
			return flatte.Frame{CursorStyle: tc.style, Root: layout.Text{String: "    \n    ", Caret: &layout.Caret{X: 1, Y: 0}}}
		}}
		img, err := Snapshot(app, Options{Width: 10, Height: 3}, 2)
		if err != nil {
			t.Fatal(err)
		}
		m := newTheme(themeFor(nil, true, nil)).metrics()
		cell := m.Cell(layout.FaceCode)
		at := func(p probe) color.Color {
			return img.At(int((cell.Column*(1+p.fx))*2), int(cell.Line*p.fy*2))
		}
		reddish := func(c color.Color) bool {
			r, g, b, _ := c.RGBA()
			return r > 0x6000 && r > 2*g && r > 2*b
		}
		for _, p := range tc.red {
			if !reddish(at(p)) {
				t.Errorf("%s: %v at %v, want the caret", tc.name, at(p), p)
			}
		}
		for _, p := range tc.not {
			if reddish(at(p)) {
				t.Errorf("%s: caret at %v", tc.name, p)
			}
		}
	}
}
