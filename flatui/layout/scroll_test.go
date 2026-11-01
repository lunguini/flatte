package layout

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// scrollLines is a Col of n one-line ID'd rows "l0".."l<n-1>".
func scrollLines(n int) Col {
	rows := make([]Node, n)
	for i := range rows {
		rows[i] = Text{NodeBase: NodeBase{ID: fmt.Sprintf("l%d", i), H: Fixed(1)}, String: fmt.Sprintf("line %d", i)}
	}
	return Col{NodeBase: NodeBase{ID: "content"}, Children: rows}
}

func plainLines(frame string) []string {
	lines := strings.Split(ansi.Strip(frame), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " ")
	}
	return lines
}

func TestScrollShowsWindowAtOffset(t *testing.T) {
	root := Scroll{NodeBase: NodeBase{ID: "s"}, OffsetY: 3, Child: scrollLines(10)}
	c := Compose(root, 10, 4)
	want := []string{"line 3", "line 4", "line 5", "line 6"}
	if got := plainLines(c.Frame); !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %q, want %q", got, want)
	}
	for id, want := range map[string]Rect{
		"s":       {0, 0, 10, 4},
		"content": {0, -3, 10, 10}, // natural height, shifted by -OffsetY
		"l0":      {0, -3, 10, 1},
		"l3":      {0, 0, 10, 1},
		"l9":      {0, 6, 10, 1},
	} {
		if got := c.Rects[id]; got != want {
			t.Errorf("rect %s = %+v, want %+v", id, got, want)
		}
	}
}

func TestScrollClampsOffset(t *testing.T) {
	for _, tc := range []struct {
		offset, wantY int
		first         string
	}{
		{-5, 0, "line 0"},
		{0, 0, "line 0"},
		{6, -6, "line 6"},
		{100, -6, "line 6"}, // past the end: last viewport-full of content
	} {
		c := Compose(Scroll{OffsetY: tc.offset, Child: scrollLines(10)}, 10, 4)
		if got := c.Rects["content"].Y; got != tc.wantY {
			t.Errorf("offset %d: content.Y = %d, want %d", tc.offset, got, tc.wantY)
		}
		if got := plainLines(c.Frame)[0]; got != tc.first {
			t.Errorf("offset %d: first line = %q, want %q", tc.offset, got, tc.first)
		}
	}
}

func TestClampScroll(t *testing.T) {
	for _, tc := range []struct{ offset, viewport, content, want int }{
		{3, 4, 10, 3},
		{-1, 4, 10, 0},
		{7, 4, 10, 6},
		{2, 4, 3, 0}, // content fits: no scrolling
		{maxInt, 0, maxInt, maxInt},
		{5, -3, 4, 4}, // malformed viewport counts as zero
	} {
		if got := ClampScroll(tc.offset, tc.viewport, tc.content); got != tc.want {
			t.Errorf("ClampScroll(%d, %d, %d) = %d, want %d", tc.offset, tc.viewport, tc.content, got, tc.want)
		}
	}
}

func TestScrollContentHeight(t *testing.T) {
	for _, tc := range []struct {
		name  string
		child Node
		wantH int
	}{
		// Measured content shorter than the viewport fills it, so wrapping a
		// fitting layout in a Scroll changes nothing visible.
		{"content fills short", Text{NodeBase: NodeBase{ID: "content"}, String: "a\nb"}, 6},
		{"content natural tall", Text{NodeBase: NodeBase{ID: "content"}, String: strings.Repeat("x\n", 9) + "x"}, 10},
		// A Fixed claim is honored exactly, even below the viewport.
		{"fixed exact", Text{NodeBase: NodeBase{ID: "content", H: Fixed(2)}, String: "a"}, 2},
		{"fixed tall", Text{NodeBase: NodeBase{ID: "content", H: Fixed(20)}, String: "a"}, 20},
		// Nothing to measure: the child is the viewport.
		{"grow", Text{NodeBase: NodeBase{ID: "content", H: Grow(1)}, String: "a"}, 6},
	} {
		c := Compose(Scroll{Child: tc.child}, 8, 6)
		if got := c.Rects["content"].H; got != tc.wantH {
			t.Errorf("%s: content.H = %d, want %d", tc.name, got, tc.wantH)
		}
	}
}

func TestScrollWrappedFittingContentIsUnchanged(t *testing.T) {
	inner := Col{
		NodeBase: NodeBase{Bordered: true, Pad: 1},
		Children: []Node{Text{String: "hello"}, NewSpacer(), Text{String: "bye"}},
	}
	plain := Compose(inner, 20, 8)
	scrolled := Compose(Scroll{Child: inner}, 20, 8)
	if plain.Frame != scrolled.Frame {
		t.Fatalf("Scroll around fitting content changed the frame:\n%s\nvs\n%s", plain.Frame, scrolled.Frame)
	}
}

func TestScrollClipsHitTesting(t *testing.T) {
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "header", H: Fixed(1)}, String: "header"},
		Scroll{NodeBase: NodeBase{ID: "s", H: Fixed(3)}, OffsetY: 2, Child: scrollLines(10)},
		Text{NodeBase: NodeBase{ID: "footer", H: Fixed(1)}, String: "footer"},
	}}
	c := Compose(root, 10, 5)
	want := []string{"header", "line 2", "line 3", "line 4", "footer"}
	if got := plainLines(c.Frame); !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %q, want %q", got, want)
	}
	// l1 sits under the header and l5 under the footer: scrolled-out rows
	// are not hittable, the painted neighbours win.
	if r := c.Rects["l1"]; r != (Rect{0, 0, 10, 1}) {
		t.Fatalf("l1 rect = %+v", r)
	}
	for _, tc := range []struct {
		y    int
		want string
	}{{0, "header"}, {1, "l2"}, {3, "l4"}, {4, "footer"}} {
		if got, _ := c.HitTest(0, tc.y); got != tc.want {
			t.Errorf("HitTest(0,%d) = %q, want %q", tc.y, got, tc.want)
		}
	}
}

func TestScrollHitReturnsFullRect(t *testing.T) {
	// A node cut by the viewport edge reports its whole solved rect, so
	// target-local coordinates (y - rect.Y) index into the full node.
	child := Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "big", H: Fixed(5)}, String: "a\nb\nc\nd\ne"},
		Text{NodeBase: NodeBase{ID: "after", H: Fixed(5)}, String: "f"},
	}}
	c := Compose(Scroll{NodeBase: NodeBase{ID: "s"}, OffsetY: 2, Child: child}, 4, 4)
	id, r := c.HitTest(0, 0)
	if id != "big" || r != (Rect{0, -2, 4, 5}) {
		t.Fatalf("HitTest(0,0) = %q %+v, want big {0 -2 4 5}", id, r)
	}
	if got := plainLines(c.Frame); !reflect.DeepEqual(got, []string{"c", "d", "e", "f"}) {
		t.Fatalf("frame = %q", got)
	}
	// The next node starts inside the viewport and is hittable there.
	if id, r := c.HitTest(3, 3); id != "after" || r.H != 5 {
		t.Errorf("HitTest(3,3) = %q %+v, want after", id, r)
	}
}

func TestScrollEmptyAreaHitsScroll(t *testing.T) {
	c := Compose(Scroll{NodeBase: NodeBase{ID: "s"}, Child: Text{NodeBase: NodeBase{H: Fixed(1)}, String: "x"}}, 4, 3)
	if id, r := c.HitTest(0, 2); id != "s" || r != (Rect{0, 0, 4, 3}) {
		t.Errorf("HitTest below content = %q %+v, want s", id, r)
	}
}

func TestScrollPartialLeafKeepsStyling(t *testing.T) {
	red := lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	leaf := Text{NodeBase: NodeBase{H: Fixed(3)}, String: red.Render("aaa\nbbb\nccc")}
	scrolled := Compose(Scroll{OffsetY: 1, Child: Col{Children: []Node{leaf, Text{NodeBase: NodeBase{H: Fixed(3)}, String: "z"}}}}, 3, 2)
	// Reference: the same styled rows painted unclipped.
	ref := Compose(Text{String: red.Render("bbb\nccc")}, 3, 2)
	if scrolled.Frame != ref.Frame {
		t.Fatalf("clipped leaf lost styling:\n%q\nwant\n%q", scrolled.Frame, ref.Frame)
	}
}

func TestScrollPartialLeafStaysInsideWindow(t *testing.T) {
	// The window starts below a header and ends above a footer; a leaf cut by
	// either edge must not paint over its neighbours.
	for _, tc := range []struct {
		offset int
		want   []string
	}{
		{1, []string{"header", "b", "c", "footer"}},
		{0, []string{"header", "a", "b", "footer"}},
	} {
		root := Col{Children: []Node{
			Text{NodeBase: NodeBase{H: Fixed(1)}, String: "header"},
			Scroll{NodeBase: NodeBase{H: Fixed(2)}, OffsetY: tc.offset, Child: Text{NodeBase: NodeBase{H: Fixed(3)}, String: "a\nb\nc"}},
			Text{NodeBase: NodeBase{H: Fixed(1)}, String: "footer"},
		}}
		if got := plainLines(Compose(root, 6, 4).Frame); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("offset %d: frame = %q, want %q", tc.offset, got, tc.want)
		}
	}
	// Nothing is painted below the window to hide an overrun: the bottom edge
	// must clip by itself.
	root := Col{Children: []Node{
		Scroll{NodeBase: NodeBase{H: Fixed(2)}, Child: Col{Children: []Node{
			Text{NodeBase: NodeBase{H: Fixed(3)}, String: "a\nb\nc"},
		}}},
	}}
	frame := Compose(Row{Children: []Node{root}}, 3, 3).Frame
	if got := plainLines(frame); !reflect.DeepEqual(got, []string{"a", "b", ""}) {
		t.Errorf("bottom edge: frame = %q", got)
	}
}

func TestScrollPaintsChromeAndInsetsViewport(t *testing.T) {
	root := Scroll{NodeBase: NodeBase{ID: "s", Bordered: true}, OffsetY: 1, Child: scrollLines(10)}
	c := Compose(root, 10, 4)
	lines := plainLines(c.Frame)
	if !strings.HasPrefix(lines[0], "╭") || !strings.HasPrefix(lines[3], "╰") {
		t.Fatalf("border missing: %q", lines)
	}
	if lines[1] != "│line 1  │" || lines[2] != "│line 2  │" {
		t.Fatalf("viewport rows = %q", lines[1:3])
	}
	if got := c.Rects["content"]; got != (Rect{1, 0, 8, 10}) {
		t.Errorf("content rect = %+v, want {1 0 8 10}", got)
	}
	// The border row covers l0's solved rect; it is not hittable there.
	if id, _ := c.HitTest(1, 0); id != "s" {
		t.Errorf("HitTest on border = %q, want s", id)
	}
}

func TestNestedScrollClipsToBoth(t *testing.T) {
	inner := Scroll{NodeBase: NodeBase{ID: "inner", H: Fixed(4)}, OffsetY: 1, Child: scrollLines(10)}
	outer := Scroll{NodeBase: NodeBase{ID: "outer"}, OffsetY: 2, Child: Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "top", H: Fixed(3)}, String: "t0\nt1\nt2"},
		inner,
		Text{NodeBase: NodeBase{ID: "bottom", H: Fixed(3)}, String: "b0\nb1\nb2"},
	}}}
	c := Compose(outer, 6, 4)
	want := []string{"t2", "line 1", "line 2", "line 3"}
	if got := plainLines(c.Frame); !reflect.DeepEqual(got, want) {
		t.Fatalf("frame = %q, want %q", got, want)
	}
	// l0 is scrolled out of the inner viewport even though it sits inside the
	// outer one; the outer's own content wins there.
	if r := c.Rects["l0"]; r != (Rect{0, 0, 6, 1}) {
		t.Fatalf("l0 rect = %+v", r)
	}
	if id, _ := c.HitTest(0, 0); id != "top" {
		t.Errorf("HitTest(0,0) = %q, want top", id)
	}
}

func TestScrollOverlayStaysViewportCentered(t *testing.T) {
	child := Col{Children: []Node{
		scrollLines(20),
		Text{NodeBase: NodeBase{ID: "modal", Overlay: true, W: Fixed(4), H: Fixed(2)}, String: "MM\nMM"},
	}}
	c := Compose(Scroll{OffsetY: 5, Child: child}, 10, 6)
	if r := c.Rects["modal"]; r != (Rect{3, 2, 4, 2}) {
		t.Fatalf("modal rect = %+v, want viewport-centered {3 2 4 2}", r)
	}
	if id, _ := c.HitTest(3, 2); id != "modal" {
		t.Errorf("HitTest on modal = %q", id)
	}
	if !strings.Contains(plainLines(c.Frame)[2], "MM") {
		t.Errorf("modal not painted: %q", plainLines(c.Frame))
	}
}

func TestScrollOverlayChildIsSkippedInBase(t *testing.T) {
	modal := Text{NodeBase: NodeBase{ID: "modal", Overlay: true, W: Fixed(2), H: Fixed(1)}, String: "M"}
	c := Compose(Scroll{NodeBase: NodeBase{ID: "s"}, Child: modal}, 6, 3)
	if r := c.Rects["modal"]; r != (Rect{2, 1, 2, 1}) {
		t.Errorf("overlay child rect = %+v, want centered {2 1 2 1}", r)
	}
}

func TestScrollNilChild(t *testing.T) {
	c := Compose(Scroll{NodeBase: NodeBase{ID: "s", Bordered: true}}, 4, 3)
	if r := c.Rects["s"]; r != (Rect{0, 0, 4, 3}) {
		t.Errorf("rect = %+v", r)
	}
	if lines := plainLines(c.Frame); lines[0] != "╭──╮" {
		t.Errorf("chrome = %q", lines)
	}
	if got := (Scroll{}).Render(Rect{W: 3, H: 2}); ansi.Strip(got) != "   \n   " {
		t.Errorf("Render nil child = %q", got)
	}
}

func TestScrollSize(t *testing.T) {
	for _, tc := range []struct {
		name   string
		s      Scroll
		wantW  Size
		wantHK SizeKind
	}{
		{"auto measures child width, grows tall", Scroll{Child: Text{String: "hello"}}, Size{Kind: SizeContent, Value: 5}, SizeGrow},
		{"inset adds to width", Scroll{NodeBase: NodeBase{Bordered: true}, Child: Text{String: "hello"}}, Size{Kind: SizeContent, Value: 7}, SizeGrow},
		{"growing child", Scroll{Child: NewSpacer()}, Grow(1), SizeGrow},
		{"explicit wins", Scroll{NodeBase: NodeBase{W: Fixed(3), H: Fixed(2)}, Child: Text{String: "hello"}}, Fixed(3), SizeFixed},
		{"nil child", Scroll{}, Auto(), SizeGrow},
	} {
		w, h := tc.s.Size()
		if w != tc.wantW || h.Kind != tc.wantHK {
			t.Errorf("%s: Size = %+v %+v, want %+v kind %v", tc.name, w, h, tc.wantW, tc.wantHK)
		}
	}
}

func TestScrollPointerMatchesValue(t *testing.T) {
	s := Scroll{NodeBase: NodeBase{ID: "s"}, OffsetY: 3, Child: scrollLines(10)}
	if a, b := Compose(s, 10, 4), Compose(&s, 10, 4); a.Frame != b.Frame || !reflect.DeepEqual(a.Rects, b.Rects) {
		t.Fatal("*Scroll composes differently from Scroll")
	}
}

func TestScrollRenderMatchesCompose(t *testing.T) {
	s := Scroll{OffsetY: 4, Child: scrollLines(10)}
	if got, want := s.Render(Rect{W: 8, H: 3}), Compose(s, 8, 3).Frame; got != want {
		t.Fatalf("Render = %q, want %q", got, want)
	}
	// Nested inside a Row rendered as a leaf block.
	row := Row{Children: []Node{Text{NodeBase: NodeBase{W: Fixed(2)}, String: ">"}, s}}
	if got := plainLines(row.Render(Rect{W: 10, H: 3})); !reflect.DeepEqual(got, []string{"> line 4", "  line 5", "  line 6"}) {
		t.Fatalf("row render = %q", got)
	}
}

func TestScrollSolveMatchesCompose(t *testing.T) {
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{ID: "h", H: Fixed(1)}, String: "h"},
		Scroll{NodeBase: NodeBase{ID: "s"}, OffsetY: 7, Child: scrollLines(30)},
	}}
	c := Compose(root, 12, 6)
	if !reflect.DeepEqual(c.Rects, Solve(root, 12, 6)) {
		t.Fatal("Solve rects diverge from Compose for a scrolled tree")
	}
}

func TestScrollHugeOffsetAndContent(t *testing.T) {
	// Saturating geometry: no overflow, no panic, and the viewport is not
	// painted from absurd coordinates.
	child := Col{NodeBase: NodeBase{ID: "content"}, Children: []Node{
		Text{NodeBase: NodeBase{ID: "a", H: Fixed(1)}, String: "a"},
		Col{NodeBase: NodeBase{ID: "huge", H: Fixed(maxInt)}}, // a childless container is never Render'd
	}}
	c := Compose(Scroll{OffsetY: maxInt, Child: child}, 4, 2)
	if r := c.Rects["content"]; r.H != maxInt || r.Y > 0 {
		t.Fatalf("content rect = %+v", r)
	}
	c = Compose(Scroll{OffsetY: -maxInt - 1, Child: child}, 4, 2)
	if got := plainLines(c.Frame)[0]; got != "a" {
		t.Fatalf("first line = %q", got)
	}
}
