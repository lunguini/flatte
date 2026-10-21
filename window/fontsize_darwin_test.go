package window

import (
	"math"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestTextStyleSize(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	body := darkTheme.styleFont(layout.TextStyle{})
	big := darkTheme.styleFont(layout.TextStyle{Size: 30})
	if big.pitch() <= body.pitch()*1.5 {
		t.Fatalf("30pt pitch %v not well above body %v", big.pitch(), body.pitch())
	}
	// A title at an explicit size takes that size, not the title size.
	if title := darkTheme.styleFont(layout.TextStyle{Role: layout.TextTitle, Size: 30}); math.Abs(title.ascent-big.ascent) > 2 {
		t.Errorf("30pt title ascent %v, 30pt body %v", title.ascent, big.ascent)
	}
	// Bad sizes fall back to the role's size; huge ones are capped; near
	// sizes share a font.
	for _, s := range []float64{-4, math.NaN(), math.Inf(1)} {
		if f := darkTheme.styleFont(layout.TextStyle{Size: s}); f != body {
			t.Errorf("size %v: font %+v, want body %+v", s, f, body)
		}
	}
	if a, b := darkTheme.styleFont(layout.TextStyle{Size: 1e9}), darkTheme.styleFont(layout.TextStyle{Size: maxFontSize}); a != b {
		t.Errorf("1e9 not capped to %v", maxFontSize)
	}
	if a, b := darkTheme.styleFont(layout.TextStyle{Size: 30.01}), big; a != b {
		t.Errorf("30.01 and 30 differ")
	}
	// Scene measurement follows it.
	m := darkTheme.metrics()
	_, h1 := m.MeasureText("Hi", layout.TextStyle{}, 1000)
	_, h2 := m.MeasureText("Hi", layout.TextStyle{Size: 30}, 1000)
	if h2 <= h1 {
		t.Errorf("30pt text %vpt tall, body %vpt", h2, h1)
	}
}
