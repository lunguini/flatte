package window

import (
	"os"
	"testing"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestFontFamiliesListsInstalledFamilies(t *testing.T) {
	start := time.Now()
	fams := FontFamilies()
	t.Logf("%d families in %v", len(fams), time.Since(start))
	if len(fams) == 0 {
		t.Fatal("no font families")
	}
	byName := map[string]FontFamily{}
	mono := 0
	for i, f := range fams {
		if f.Name == "" || f.Name[0] == '.' {
			t.Errorf("family %d: %q, want a visible name", i, f.Name)
		}
		if i > 0 && lessFamily(f, fams[i-1]) {
			t.Errorf("%q before %q: not sorted", fams[i-1].Name, f.Name)
		}
		if f.Monospace {
			mono++
		}
		byName[f.Name] = f
	}
	if mono == 0 {
		t.Fatal("no monospaced family")
	}
	// Every Mac has Menlo (fixed-pitch) and Helvetica (proportional).
	if f, ok := byName["Menlo"]; !ok || !f.Monospace {
		t.Errorf("Menlo: %+v %v, want a monospaced family", f, ok)
	}
	if f, ok := byName["Helvetica"]; !ok || f.Monospace {
		t.Errorf("Helvetica: %+v %v, want a proportional family", f, ok)
	}
}

func lessFamily(a, b FontFamily) bool {
	s := []FontFamily{b, a}
	sortFamilies(s)
	return s[0] == a && a != b
}

// TestTextStyleFontSetsTheFamily: a style's Font is the family its text is
// set in, whatever the theme's.
func TestTextStyleFontSetsTheFamily(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	mono, err := os.ReadFile("testdata/flat-mono.ttf")
	if err != nil {
		t.Fatal(err)
	}
	if err := registerFonts([][]byte{mono}); err != nil {
		t.Fatal(err)
	}
	th := newTheme(Theme{Font: "Helvetica"})
	fixed := func(f font) bool { return f.width("iiii") == f.width("MMMM") }
	if fixed(th.styleFont(layout.TextStyle{})) {
		t.Fatal("the theme's Helvetica is fixed-pitch")
	}
	for _, s := range []layout.TextStyle{{Font: "Fl Mono"}, {Font: "Fl Mono", Role: layout.TextTitle}, {Font: "Fl Mono", Size: 20}} {
		if !fixed(th.styleFont(s)) {
			t.Errorf("%+v: not set in Fl Mono", s)
		}
	}
	// Distinct fonts per family, and the theme's again without one.
	if a, b := th.styleFont(layout.TextStyle{Font: "Fl Mono"}), th.styleFont(layout.TextStyle{}); a.ref == b.ref {
		t.Error("Fl Mono and the theme's font are one font")
	}
}
