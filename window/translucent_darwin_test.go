package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// TestFrameTranslucentLeavesTheBackgroundToTheBlur: a frame asking for
// translucency draws no window background (the snapshot's corner is
// transparent), where an opaque one paints it; a node's own fill stays.
func TestFrameTranslucentLeavesTheBackgroundToTheBlur(t *testing.T) {
	for _, on := range []bool{false, true} {
		app := flatte.App[struct{}]{State: &struct{}{}, View: func(*struct{}, flatte.RenderContext) flatte.Frame {
			return flatte.Frame{Translucent: on, Root: layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1)}, Children: []layout.Node{
				layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Fixed(2), Box: layout.Box{Fill: layout.FillField}}},
			}}}
		}}
		img, err := Snapshot(app, Options{Width: 20, Height: 6}, 1)
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		_, _, _, corner := img.At(b.Max.X-1, b.Max.Y-1).RGBA()
		_, _, _, filled := img.At(2, 2).RGBA()
		if on && corner != 0 || !on && corner == 0 {
			t.Errorf("Translucent %v: background alpha %d", on, corner)
		}
		if filled == 0 {
			t.Errorf("Translucent %v: the field's own fill is gone", on)
		}
	}
}
