package window

import (
	"math"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
	ds "github.com/lunguini/flatte/theme"
)

// The code grid is set in the design's code size, apart from the interface
// text: bigger code leaves body text as it was, and equal sizes measure as
// before.
func TestCodeSizeIsApartFromTheInterface(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	base := ds.Default()
	big := ds.Default()
	big.Type.Code.Size = 20
	m0 := newTheme(fromDesign(&base, true)).metrics()
	m1 := newTheme(fromDesign(&big, true)).metrics()
	if !(m1.Column > m0.Column*1.4) || !(m1.Line > m0.Line*1.3) {
		t.Errorf("20pt code: column %v line %v, was %v %v", m1.Column, m1.Line, m0.Column, m0.Line)
	}
	w0, _ := m0.MeasureText("Settings", ds0Body(), 0)
	w1, _ := m1.MeasureText("Settings", ds0Body(), 0)
	if math.Abs(w0-w1) > 0.01 {
		t.Errorf("body text %v wide with 20pt code, %v before", w1, w0)
	}
	// An app theme that sets only FontSize still sizes both, as it did.
	th := newTheme(Theme{FontSize: 20}.over(Dark()))
	if m := th.metrics(); !(m.Column > m0.Column*1.4) {
		t.Errorf("FontSize 20 alone: column %v", m.Column)
	}
	// Equal sizes: the grid measures exactly as the body-sized grid did.
	if m := newTheme(Dark()).metrics(); m.Line != m0.Line || m.Column != m0.Column {
		t.Errorf("default grid changed: %v×%v, want %v×%v", m.Column, m.Line, m0.Column, m0.Line)
	}
}

func ds0Body() layout.TextStyle { return layout.TextStyle{Role: layout.TextBody} }
