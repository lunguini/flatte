package window

import (
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// TestSnapshotReportsThemeAppearanceToView: View sees the active theme's
// appearance in RenderContext.
func TestSnapshotReportsThemeAppearanceToView(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	light, dark := Light(), Dark()
	for name, c := range map[string]struct {
		th   *Theme
		want flatte.Appearance
	}{
		"light": {&light, flatte.AppearanceLight},
		"dark":  {&dark, flatte.AppearanceDark},
		"none":  {nil, flatte.AppearanceDark}, // Snapshot draws Dark without a theme
	} {
		var seen []flatte.Appearance
		app := flatte.App[snapState]{
			State: &snapState{},
			View: func(_ *snapState, ctx flatte.RenderContext) flatte.Frame {
				seen = append(seen, ctx.Appearance)
				return flatte.Frame{Root: layout.Label{Text: "x"}}
			},
		}
		if _, err := Snapshot(app, Options{Width: 10, Height: 2, Theme: c.th}, 1); err != nil {
			t.Fatal(err)
		}
		if len(seen) == 0 {
			t.Fatalf("%s: View never ran", name)
		}
		for _, got := range seen {
			if got != c.want {
				t.Fatalf("%s: View saw appearance %d, want %d", name, got, c.want)
			}
		}
	}
}
