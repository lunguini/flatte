package kit

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestSegmentedUsesPopupWhenOptionsDoNotFit(t *testing.T) {
	seg := Segmented{ID: "density", Options: []string{"Compact", "Regular", "Comfortable"}, Selected: 1}

	wide := layout.Compose(seg.Layout(), 40, 2)
	if _, ok := wide.Rects[OptionID("density", 2)]; !ok {
		t.Fatal("wide control did not lay out every option")
	}
	if !strings.Contains(wide.Frame, "Comfortable") {
		t.Fatalf("wide control clipped its last option: %q", wide.Frame)
	}

	narrow := layout.Compose(seg.Layout(), 15, 2)
	if _, ok := narrow.Rects["density"]; !ok {
		t.Fatal("narrow popup button has no target")
	}
	if _, ok := narrow.Rects[OptionID("density", 0)]; ok {
		t.Fatal("narrow control still laid out segmented options")
	}
	if !strings.Contains(narrow.Frame, "Regular") {
		t.Fatalf("popup button does not show the selected option: %q", narrow.Frame)
	}

	seg.Open = true
	open := layout.Compose(seg.Layout(), 15, 8)
	for i, label := range seg.Options {
		if _, ok := open.Rects[OptionID("density", i)]; !ok {
			t.Errorf("open popup is missing option %q", label)
		}
	}
}

func TestSegmentedUsesPointTextMetricsToChoosePopup(t *testing.T) {
	seg := Segmented{ID: "density", Options: []string{"Compact", "Regular", "Comfortable"}, Selected: 1}
	m := points
	m.MeasureText = func(text string, _ layout.TextStyle, _ float64) (float64, float64) {
		switch text {
		case "Compact":
			return 34, 16
		case "Regular":
			return 42, 16
		case "Comfortable":
			return 73, 16
		default:
			return float64(len(text)) * 6, 16
		}
	}

	scene := layout.LayoutScene(seg.Layout(), 150, 40, m)
	if _, ok := scene.Rects["density"]; !ok {
		t.Fatal("narrow point layout did not use the popup button")
	}
	if _, ok := scene.Rects[OptionID("density", 2)]; ok {
		t.Fatal("narrow point layout kept the segmented options")
	}

	scene = layout.LayoutScene(seg.Layout(), 197, 40, m)
	if _, ok := scene.Rects[OptionID("density", 2)]; !ok {
		t.Fatal("the options did not fit at their measured 197pt width")
	}
}

func TestSegmentedPopupShowsAllOptionsWhenOpen(t *testing.T) {
	seg := Segmented{ID: "density", Options: []string{"Compact", "Regular", "Comfortable"}, Selected: 1, Open: true, Highlighted: 2}
	scene := layout.LayoutScene(seg.Layout(), 100, 160, points)
	labels := map[string]bool{}
	for _, op := range scene.Ops {
		if text, ok := op.(layout.TextOp); ok {
			labels[text.Text] = true
		}
	}
	for _, label := range seg.Options {
		if !labels[label] {
			t.Errorf("popup is missing option %q", label)
		}
	}
	if _, ok := scene.Rects[OptionID("density", 2)]; !ok {
		t.Fatal("the highlighted option has no click target")
	}
}
