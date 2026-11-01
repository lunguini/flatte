package layout

import (
	"image"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestImageTerminalShowsText(t *testing.T) {
	img := Image{Source: image.NewRGBA(image.Rect(0, 0, 64, 32)), Text: "logo"}
	if w, h := img.Size(); w != (Size{Kind: SizeContent, Value: 4}) || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Fatalf("Size = %+v %+v", w, h)
	}
	got := img.Render(Rect{W: 6, H: 1})
	if ansi.Strip(got) != "logo  " {
		t.Errorf("Render = %q", got)
	}
	if got != (Label{Text: "logo", Style: TextStyle{Role: TextSubtle}}).Render(Rect{W: 6, H: 1}) {
		t.Errorf("not drawn as a subtle label: %q", got)
	}
	// The text wraps in a box narrower than it.
	long := Image{Text: "a sun over hills"}
	if got := ansi.Strip(long.Render(Rect{W: 9, H: 2})); got != "a sun    \nover     " {
		t.Errorf("wrapped = %q", got)
	}
}

func TestImageScene(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 64, 32))
	root := Col{Children: []Node{
		Image{NodeBase: NodeBase{ID: "auto"}, Source: src, Text: "logo"},
		Image{NodeBase: NodeBase{ID: "sized", W: Points(20), H: Points(10)}, Source: src, Fit: ImageCover},
		Image{NodeBase: NodeBase{ID: "none"}, Text: "missing"},
	}}
	s := LayoutScene(root, 400, 300, testMetrics)
	// One pixel to a point on Auto axes (the width stretches in a Col).
	if r := s.Rects["auto"]; r.H != 32 {
		t.Errorf("auto rect %+v, want 32pt tall", r)
	}
	if w, _ := (pointMetric{testMetrics}).leaf(root.Children[0], -1); w.Value != 64 {
		t.Errorf("auto width %+v, want 64pt", w)
	}
	if r := s.Rects["sized"]; r.W != 20 || r.H != 10 {
		t.Errorf("sized rect %+v", r)
	}
	var images []ImageOp
	var texts []TextOp
	for _, op := range s.Ops {
		switch op := op.(type) {
		case ImageOp:
			images = append(images, op)
		case TextOp:
			texts = append(texts, op)
		}
	}
	if len(images) != 2 || images[0].Source != image.Image(src) || images[1].Fit != ImageCover || images[1].Rect != s.Rects["sized"] {
		t.Errorf("image ops %+v", images)
	}
	// Without a picture a GUI shows the text.
	if len(texts) != 1 || texts[0].Text != "missing" || texts[0].Style.Role != TextSubtle || !texts[0].Wrap || s.Rects["none"].H != 16 {
		t.Errorf("text ops %+v, rect %+v", texts, s.Rects["none"])
	}
}
