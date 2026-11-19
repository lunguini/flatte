package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
)

// A snapshot draws the frame's Theme (its accent, its density) in the
// appearance Options fixes.
func TestSnapshotDrawsFrameTheme(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	teal := ds.Pair{Light: ds.Hex("#0e7468"), Dark: ds.Hex("#4fd1c0")}
	mine := ds.Default().With(ds.Seeds{Accent: &teal, Density: ds.Comfortable})
	type st struct{}
	app := flatte.App[st]{State: &st{}, View: func(*st, flatte.RenderContext) flatte.Frame {
		return flatte.Frame{Theme: &mine, Root: layout.Row{Children: []layout.Node{
			layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillAccent}}},
			layout.Col{NodeBase: layout.NodeBase{W: layout.Grow(1), H: layout.Grow(1), Box: layout.Box{Fill: layout.FillContent}}},
		}}}
	}}
	for _, tc := range []struct {
		app  flatte.Appearance
		pal  *ds.Palette
		name string
	}{{flatte.AppearanceUnknown, &mine.Dark, "dark"}, {flatte.AppearanceLight, &mine.Light, "light"}, {flatte.AppearanceDark, &mine.Dark, "fixed dark"}} {
		img, err := Snapshot(app, Options{Width: 40, Height: 4, Appearance: tc.app}, 1)
		if err != nil {
			t.Fatal(err)
		}
		b := img.Bounds()
		if got := img.At(b.Dx()/4, b.Dy()/2); !near(got, tc.pal.Accent, 2) {
			t.Errorf("%s: accent %v, want %v", tc.name, got, tc.pal.Accent)
		}
		if got := img.At(3*b.Dx()/4, b.Dy()/2); !near(got, tc.pal.Content, 2) {
			t.Errorf("%s: content %v, want %v", tc.name, got, tc.pal.Content)
		}
	}
}

func TestOptionsAppearance(t *testing.T) {
	if (Options{}).dark(false) || !(Options{}).dark(true) || !(Options{}).follows() {
		t.Fatal("unset appearance should follow the system")
	}
	if (Options{Appearance: flatte.AppearanceLight}).dark(true) || !(Options{Appearance: flatte.AppearanceDark}).dark(false) {
		t.Fatal("fixed appearance ignored")
	}
	if (Options{Appearance: flatte.AppearanceDark}).follows() || (Options{Theme: &Theme{}}).follows() {
		t.Fatal("a fixed look follows the system")
	}
}
