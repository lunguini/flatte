package window

import (
	"image/color"
	"testing"

	"github.com/lunguini/flatte"
)

func TestThemeAppearanceFollowsBackgroundLuminance(t *testing.T) {
	white := Dark()
	white.Background = color.RGBA{0xfa, 0xfa, 0xfa, 0xff}
	mid := Light()
	mid.Background = color.RGBA{0x30, 0x30, 0x30, 0xff}
	cases := []struct {
		name string
		th   Theme
		want flatte.Appearance
	}{
		{"dark", Dark(), flatte.AppearanceDark},
		{"light", Light(), flatte.AppearanceLight},
		{"dark base, white background", white, flatte.AppearanceLight},
		{"light base, dark background", mid, flatte.AppearanceDark},
		{"zero theme", Theme{}, flatte.AppearanceDark},
	}
	for _, c := range cases {
		if got := c.th.appearance(); got != c.want {
			t.Errorf("%s: appearance = %d, want %d", c.name, got, c.want)
		}
	}
}
