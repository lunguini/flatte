package flatui

import (
	"math"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
	flattehost "github.com/lunguini/flatte/host"
)

func TestListAdoptMeasuresTerminalRowsAndInitialProbe(t *testing.T) {
	var list List
	list.SetCount(20)
	event := flatte.LayoutEvent{Rects: map[string]layout.Rect{"songs": {W: 30, H: 9}}}
	list.Adopt(event, "songs")
	if list.Height() != 1 {
		t.Fatalf("initial measurement probe %d", list.Height())
	}
	event.Rects["songs:0"] = layout.Rect{W: 30, H: 2}
	event.Rects["songs:1"] = layout.Rect{Y: 2, W: 30, H: 2}
	event.Rects["songs:2"] = layout.Rect{Y: 4, W: 30, H: 2}
	event.Rects["songs:3"] = layout.Rect{Y: 6, W: 30, H: 2}
	event.Rects["songs:4"] = layout.Rect{Y: 8, W: 30, H: 1} // clipped trailing row is not the row height
	event.Rects["songs:0:title"] = layout.Rect{H: 99}
	event.Rects["songs-other:0"] = layout.Rect{H: 99}
	list.Adopt(event, "songs")
	if list.Height() != 4 || list.Count() != 20 {
		t.Fatalf("rows %d count %d", list.Height(), list.Count())
	}
}

func TestListAdoptUsesNativePointsBeforeCellRounding(t *testing.T) {
	var list List
	list.SetCount(20)
	list.SetHeight(5)
	list.Select(7)
	event := flatte.LayoutEvent{
		Rects:      map[string]layout.Rect{"songs": {W: 30, H: 6}, "songs:5": {H: 1}},
		PointRects: map[string]layout.PointRect{"songs": {W: 240, H: 100.9}, "songs:5": {W: 240, H: 30.4}, "songs:6": {Y: 30.4, H: 30.4}, "songs:7": {Y: 60.8, H: 30.4}, "songs:8": {Y: 91.2, H: 9.7}},
	}
	list.Adopt(event, "songs")
	if list.Height() != 3 || list.Cursor() != 7 {
		t.Fatalf("height %d cursor %d", list.Height(), list.Cursor())
	}
	// A density change is measured afresh, not from the previous native row.
	event.PointRects["songs:5"] = layout.PointRect{H: 20}
	event.PointRects["songs:6"] = layout.PointRect{H: 20}
	event.PointRects["songs:7"] = layout.PointRect{H: 20}
	list.Adopt(event, "songs")
	if list.Height() != 5 {
		t.Fatalf("density change height %d", list.Height())
	}
}

func TestListAdoptZeroSmallAndMissingViewport(t *testing.T) {
	var list List
	list.SetCount(10)
	list.SetHeight(3)
	list.Adopt(flatte.LayoutEvent{}, "songs")
	if list.Height() != 3 {
		t.Fatal("missing viewport discarded paging")
	}
	event := flatte.LayoutEvent{PointRects: map[string]layout.PointRect{"songs": {W: 100, H: 0}}}
	list.Adopt(event, "songs")
	if list.Height() != 0 {
		t.Fatal("zero viewport still has rows")
	}
	event.PointRects["songs"] = layout.PointRect{W: 100, H: 10}
	list.Adopt(event, "songs")
	if list.Height() != 1 {
		t.Fatal("small viewport did not request measurement")
	}
	event.PointRects["songs:0"] = layout.PointRect{W: 100, H: 10}
	list.Adopt(event, "songs")
	list.Adopt(event, "songs")
	if list.Height() != 1 {
		t.Fatal("partially visible single row oscillated")
	}
	event.PointRects["songs"] = layout.PointRect{W: 100, H: 64}
	event.PointRects["songs:0"] = layout.PointRect{W: 100, H: 32}
	list.Adopt(event, "songs")
	if list.Height() != 2 {
		t.Fatal("resized viewport did not adopt full row measurement")
	}
}

func TestListAdoptSettlesFromZeroHeight(t *testing.T) {
	var list List
	list.SetCount(20)
	view := func() flatte.Frame {
		rows := list.Rows("songs", func(int, bool) layout.Node {
			return layout.Label{NodeBase: layout.NodeBase{H: layout.Fixed(2)}, Text: "song"}
		})
		rows.ID = "songs"
		rows.W, rows.H = layout.Grow(1), layout.Grow(1)
		return flatte.Frame{Root: rows}
	}
	delivered := 0
	frame, _, _ := flattehost.SettleFrame(view, func(event flatte.LayoutEvent) { delivered++; list.Adopt(event, "songs") }, nil, 20, 9)
	if list.Height() != 4 || delivered != 2 {
		t.Fatalf("height %d delivered %d frame %q", list.Height(), delivered, frame.Content)
	}
}

func TestListAdoptBoundsMalformedGeometry(t *testing.T) {
	var list List
	list.SetCount(20)
	for _, height := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, 0} {
		list.SetHeight(3)
		list.Adopt(flatte.LayoutEvent{PointRects: map[string]layout.PointRect{"songs": {H: height}}}, "songs")
		if list.Height() != 0 {
			t.Fatalf("height %v adopted %d", height, list.Height())
		}
	}
	list.Adopt(flatte.LayoutEvent{PointRects: map[string]layout.PointRect{"songs": {H: 1e100}, "songs:0": {H: 1}}}, "songs")
	if list.Height() != maxWidgetDimension {
		t.Fatalf("unbounded page %d", list.Height())
	}
}
