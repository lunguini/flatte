package layout

import (
	"math"
	"testing"
)

func TestPointControlsShareMinimumHeight(t *testing.T) {
	m := testMetrics
	m.DefaultPadding = true
	m.ControlHeight = 32
	s := LayoutScene(Col{Align: AlignStart, Children: []Node{
		Button{NodeBase: NodeBase{ID: "button"}, Label: "OK"},
		Toggle{NodeBase: NodeBase{ID: "toggle"}, Label: "On"},
		Input{NodeBase: NodeBase{ID: "input"}, Value: "abc"},
		Button{NodeBase: NodeBase{ID: "icon"}, Icon: IconSearch, Label: "search"},
		Input{NodeBase: NodeBase{ID: "bare"}, Bare: true},
		Input{NodeBase: NodeBase{ID: "area", H: Points(70)}, Multiline: true},
		Button{NodeBase: NodeBase{ID: "fixed", H: Points(12)}, Label: "small"},
		Input{NodeBase: NodeBase{ID: "zero", H: Fixed(0)}},
	}}, 200, 400, m)
	for _, id := range []string{"button", "toggle", "input", "icon"} {
		if got := s.Rects[id].H; got != 32 {
			t.Errorf("%s height %v", id, got)
		}
	}
	if r := s.Rects["icon"]; r.W != r.H {
		t.Errorf("auto icon not square: %v", r)
	}
	if s.Rects["bare"].H != 16 || s.Rects["area"].H != 70 || s.Rects["fixed"].H != 12 || s.Rects["zero"].H != 0 {
		t.Fatalf("explicit/bare geometry changed: %v", s.Rects)
	}
}

func TestPointControlHeightGrowsForFontAndAcceptsOverride(t *testing.T) {
	m := testMetrics
	m.DefaultPadding = true
	m.ControlHeight = 40
	m.MeasureText = func(_ string, style TextStyle, _ float64) (float64, float64) {
		if style.Size > 30 {
			return 20, 64
		}
		return 20, 16
	}
	s := LayoutScene(Col{Align: AlignStart, Children: []Node{
		Button{NodeBase: NodeBase{ID: "normal"}, Label: "OK"},
		Button{NodeBase: NodeBase{ID: "large"}, Label: "OK", Style: TextStyle{Size: 40}},
		Input{NodeBase: NodeBase{ID: "field"}, Style: TextStyle{Size: 40}},
		Toggle{NodeBase: NodeBase{ID: "toggle"}, Label: "On", Style: TextStyle{Size: 40}},
	}}, 200, 400, m)
	if s.Rects["normal"].H != 40 {
		t.Fatalf("theme minimum ignored: %v", s.Rects["normal"])
	}
	for _, id := range []string{"large", "field", "toggle"} {
		if s.Rects[id].H != 72 {
			t.Errorf("%s clips larger text: %v", id, s.Rects[id])
		}
	}
}

func TestInvalidPointControlMinimumDoesNotPoisonGeometry(t *testing.T) {
	for _, minimum := range []float64{-1, math.NaN(), math.Inf(1)} {
		m := testMetrics
		m.DefaultPadding = true
		m.ControlHeight = minimum
		r := LayoutScene(Col{Align: AlignStart, Children: []Node{Button{NodeBase: NodeBase{ID: "b"}, Label: "OK"}}}, 200, 100, m).Rects["b"]
		if math.IsNaN(r.H) || math.IsInf(r.H, 0) || r.H != 24 {
			t.Fatalf("minimum %v rect %v", minimum, r)
		}
	}
}
