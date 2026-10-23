package window

import (
	"os"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestOptionsFontsRegisterAFamily(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	mono, err := os.ReadFile("testdata/flat-mono.ttf")
	if err != nil {
		t.Fatal(err)
	}
	// Twice, as two Snapshots would: the second is a no-op, not an error.
	for range 2 {
		if _, err := Snapshot(snapApp(func() layout.Node { return layout.Label{Text: "x"} }), Options{Width: 4, Height: 1, Fonts: [][]byte{mono}}, 1); err != nil {
			t.Fatal(err)
		}
	}
	// testdata/flat-mono.ttf is Go Mono renamed Fl Mono, a family no system
	// has: an unknown name falls back to a proportional face, so a
	// fixed-pitch one is the registered font.
	f := newTheme(Theme{Font: "Fl Mono"}).font(fontKey{})
	if i, M := f.width("iiii"), f.width("MMMM"); i != M {
		t.Errorf("Fl Mono: iiii %v vs MMMM %v, want equal — not the registered font", i, M)
	}
	for _, bad := range [][]byte{{}, []byte("not a font")} {
		if err := registerFonts([][]byte{bad}); err == nil {
			t.Errorf("registering %q: no error", bad)
		}
	}
}
