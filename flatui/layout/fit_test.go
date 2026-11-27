package layout

import (
	"strings"
	"testing"
)

func TestFitUsesTheWideTreeOnlyWhenItsMeasuredWidthFits(t *testing.T) {
	preferred := Row{Children: []Node{
		Label{NodeBase: NodeBase{ID: "wide:first"}, Text: "WW"},
		Label{NodeBase: NodeBase{ID: "wide:second"}, Text: "i"},
	}}
	fallback := Button{NodeBase: NodeBase{ID: "narrow"}, Label: "i ▾"}
	fit := Fit{Preferred: preferred, Fallback: fallback}

	wide := Compose(fit, 4, 1)
	if _, ok := wide.Rects["wide:first"]; !ok {
		t.Fatal("the preferred tree did not fit in four cells")
	}
	if _, ok := wide.Rects["narrow"]; ok {
		t.Fatal("the fallback was laid out while the preferred tree fit")
	}

	narrow := Compose(fit, 2, 1)
	if _, ok := narrow.Rects["narrow"]; !ok {
		t.Fatal("the fallback was not laid out when the preferred tree was too wide")
	}
	if _, ok := narrow.Rects["wide:first"]; ok {
		t.Fatal("the preferred tree was laid out after it stopped fitting")
	}
}

func TestFitMeasuresPreferredTextInWindowPoints(t *testing.T) {
	preferred := Row{Children: []Node{
		Label{NodeBase: NodeBase{ID: "wide:first"}, Text: "A"},
		Label{NodeBase: NodeBase{ID: "wide:second"}, Text: "B"},
	}}
	fallback := Button{NodeBase: NodeBase{ID: "narrow"}, Label: "B ▾"}
	m := Metrics{
		Column: 8, Line: 16, Step: 8,
		MeasureText: func(text string, _ TextStyle, _ float64) (float64, float64) {
			if text == "A" {
				return 27, 16
			}
			return 31, 16
		},
	}
	fit := Fit{Preferred: preferred, Fallback: fallback}

	tooNarrow := LayoutScene(fit, 57, 32, m)
	if _, ok := tooNarrow.Rects["narrow"]; !ok {
		t.Fatal("the fallback was not chosen below the measured 58pt width")
	}
	justFits := LayoutScene(fit, 58, 32, m)
	if _, ok := justFits.Rects["wide:first"]; !ok {
		t.Fatal("the preferred tree was not chosen at its measured 58pt width")
	}
}

func TestFitFallbackClaimsSpaceBeforeClippingSiblings(t *testing.T) {
	fit := func(id string) Node {
		return Fit{
			Preferred: Label{NodeBase: NodeBase{ID: id + ":wide"}, Text: "Longest"},
			Fallback:  Button{NodeBase: NodeBase{ID: id}, Label: id},
		}
	}
	root := Row{Children: []Node{fit("A"), fit("B"), Label{NodeBase: NodeBase{ID: "last"}, Text: "x"}}}
	composed := Compose(root, 12, 1)
	for _, id := range []string{"A", "B", "last"} {
		if _, ok := composed.Rects[id]; !ok {
			t.Errorf("fallback %s was clipped out: %q", id, composed.Frame)
		}
	}
	if strings.Contains(composed.Frame, "Longest") || !strings.Contains(composed.Frame, "[ A ]") || !strings.Contains(composed.Frame, "[ B ]") {
		t.Fatalf("narrow row clipped its popup buttons: %q", composed.Frame)
	}
}

func TestFitFallbackMeasuresPointButtonsBeforeClippingSiblings(t *testing.T) {
	fit := func(id, label string) Node {
		return Fit{
			Preferred: Label{NodeBase: NodeBase{ID: id + ":wide"}, Text: "Longest"},
			Fallback:  Button{NodeBase: NodeBase{ID: id}, Label: label},
		}
	}
	m := Metrics{
		Column: 8, Line: 16, Step: 8, DefaultPadding: true,
		MeasureText: func(text string, _ TextStyle, _ float64) (float64, float64) {
			if text == "Longest" {
				return 30, 16
			}
			return 10, 16
		},
	}
	root := Row{Children: []Node{fit("first", "A"), fit("second", "B"), Label{NodeBase: NodeBase{ID: "last"}, Text: "x"}}}
	scene := LayoutScene(root, 64, 32, m)
	for _, id := range []string{"first", "second", "last"} {
		if _, ok := scene.Rects[id]; !ok {
			t.Errorf("point fallback %s was clipped out", id)
		}
	}
	if _, ok := scene.Rects["first:wide"]; ok {
		t.Fatal("a wide option stayed in a row that needed the fallback")
	}
}

func TestFitOnlyLaysOutOverlaysInTheChosenTree(t *testing.T) {
	preferred := Row{Children: []Node{Label{NodeBase: NodeBase{ID: "wide"}, Text: "wide"}}}
	fallback := Col{Children: []Node{
		Button{NodeBase: NodeBase{ID: "trigger"}, Label: "x"},
		Col{NodeBase: NodeBase{ID: "menu", Overlay: true, Anchor: "trigger", Bordered: true}, Children: []Node{
			Label{NodeBase: NodeBase{ID: "option"}, Text: "option"},
		}},
	}}
	fit := Fit{Preferred: preferred, Fallback: fallback}

	wide := LayoutScene(fit, 100, 40, Metrics{Column: 8, Line: 16, Step: 8, Border: 1})
	if _, ok := wide.Rects["menu"]; ok {
		t.Fatal("an overlay from the unused fallback was laid out")
	}

	narrow := LayoutScene(fit, 24, 40, Metrics{Column: 8, Line: 16, Step: 8, Border: 1})
	if _, ok := narrow.Rects["menu"]; !ok {
		t.Fatal("the chosen fallback's popup was not laid out")
	}
	if _, ok := narrow.Rects["option"]; !ok {
		t.Fatal("the popup option was not laid out")
	}
}
