package window

import (
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestThemeFillColor(t *testing.T) {
	for _, th := range []Theme{Dark(), Light()} {
		cases := map[layout.Fill]any{
			layout.FillSurface:   th.Surface,
			layout.FillField:     th.Field,
			layout.FillSeparator: th.palette().Divider, // the design's divider, not the control border
			layout.FillChrome:    th.palette().Chrome,
			layout.FillAccent:    th.palette().Accent,
		}
		for f, want := range cases {
			if got := th.fillColor(f); got != want {
				t.Errorf("fill %d = %v, want %v", f, got, want)
			}
		}
		if th.fillColor(layout.FillNone) != nil || th.fillColor(layout.Fill(99)) != nil {
			t.Error("FillNone and unknown roles must have no color")
		}
	}
}
