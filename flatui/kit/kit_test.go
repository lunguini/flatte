package kit

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
	"github.com/lunguini/flatte/theme"
)

var points = layout.Metrics{Column: 8, Line: 16, Step: 8, Border: 1, DefaultPadding: true}

func sample() layout.Node {
	return layout.Col{Children: []layout.Node{
		PaneHeader{ID: "head", Title: "Files", Meta: "3", Actions: []layout.Node{layout.Button{NodeBase: layout.NodeBase{ID: "add"}, Icon: layout.IconPlus, Label: "+"}}}.Layout(),
		Tabs{ID: "tabs", Tabs: []Tab{{ID: "t0", Label: "main.go"}, {ID: "t1", Label: "go.mod", Modified: true}}, Active: 0}.Layout(),
		ListRow{ID: "r0", Icon: layout.IconFile, Title: "main.go", Meta: "M", Selected: true, Active: true}.Layout(),
		ListRow{ID: "r1", Title: "util.go", Selected: true}.Layout(),
		ListRow{ID: "r2", Title: "deep.go", Indent: 2}.Layout(),
		Segmented{ID: "seg", Options: []string{"Off", "On"}, Selected: 1}.Layout(),
		Badge{Text: "2", Tone: ToneWarning}.Layout(),
		Section{ID: "sec", Title: "General", Children: []layout.Node{
			Setting{ID: "set", Name: "Wrap", Description: "Soft-wrap long lines.", Control: layout.Toggle{NodeBase: layout.NodeBase{ID: "wrap"}, Kind: layout.ToggleSwitch}}.Layout(),
		}}.Layout(),
	}}
}

// In a window the components take the token heights; density restyles
// them through Metrics.Sizes.
func TestComponentHeightsInPoints(t *testing.T) {
	s := layout.LayoutScene(sample(), 600, 800, points)
	for id, want := range map[string]float64{"head": 36, "tabs": 36, "t0": 36, "r0": 24, "r1": 24, "seg": 28} {
		if h := s.Rects[id].H; h != want {
			t.Errorf("%s height %v, want %v", id, h, want)
		}
	}
	compact := theme.Default().With(theme.Seeds{Density: theme.Compact})
	m := points
	m.Sizes = layout.Sizes{Row: compact.Size.Row, Control: compact.Size.Control, Bar: compact.Size.Bar, Header: compact.Size.Header}
	s = layout.LayoutScene(sample(), 600, 800, m)
	if h := s.Rects["r0"].H; h != compact.Size.Row {
		t.Errorf("compact row %v", h)
	}
	// Indent shifts a row's content by steps.
	if d := s.Rects["r2"]; d.W == 0 {
		t.Fatal("indented row missing")
	}
}

// In a terminal every bar and row is one line, and the IDs are where the
// clicks land.
func TestComponentsInCells(t *testing.T) {
	c := layout.Compose(sample(), 60, 30)
	for _, id := range []string{"head", "tabs", "t0", "t1", "r0", "r1", "r2", "seg", OptionID("seg", 0), OptionID("seg", 1), "set", "wrap", "add"} {
		r, ok := c.Rects[id]
		if !ok {
			t.Errorf("%s not laid out", id)
			continue
		}
		if (id == "head" || id == "tabs" || strings.HasPrefix(id, "r")) && r.H != 1 {
			t.Errorf("%s is %d lines", id, r.H)
		}
	}
	if got := c.Rects[OptionID("seg", 1)].X; got <= c.Rects[OptionID("seg", 0)].X {
		t.Error("segments not side by side")
	}
	if c.Rects["r2"].W != c.Rects["r0"].W {
		t.Error("rows differ in width")
	}
}

// Selection: the active list's row is Selected (the strong fill, and a
// highlight in an unthemed terminal), an inactive one takes the quiet fill.
func TestListRowSelection(t *testing.T) {
	box := func(n layout.Node) layout.Box { return n.(layout.Row).Box }
	if b := box(ListRow{Selected: true, Active: true}.Layout()); !b.Selected || b.Fill != layout.FillNone {
		t.Errorf("active selection %+v", b)
	}
	if b := box(ListRow{Selected: true}.Layout()); b.Selected || b.Fill != layout.FillSelectionInactive {
		t.Errorf("inactive selection %+v", b)
	}
	if b := box(ListRow{}.Layout()); b.Selected || b.Fill != layout.FillNone || !b.Interactive {
		t.Errorf("plain row %+v", b)
	}
	if b := box(ListRow{Disabled: true}.Layout()); b.Interactive {
		t.Error("disabled row hovers")
	}
}

func TestTonesHaveTints(t *testing.T) {
	dark := theme.Default().Dark
	seen := map[layout.Fill]bool{}
	for tone := ToneNeutral; tone <= ToneDanger; tone++ {
		f := tone.fill()
		if f == layout.FillNone || seen[f] {
			t.Errorf("tone %d fill %d", tone, f)
		}
		seen[f] = true
		if _, ok := layout.FillColor(&dark, f); !ok {
			t.Errorf("tone %d fill has no color", tone)
		}
	}
}

// The active tab carries the accent edge on top; the others hover.
func TestActiveTabEdge(t *testing.T) {
	row := Tabs{Tabs: []Tab{{ID: "a", Label: "one"}, {ID: "b", Label: "two"}}, Active: 1}.Layout().(layout.Row)
	a, b := row.Children[0].(layout.Row).Box, row.Children[1].(layout.Row).Box
	if a.Edge != layout.SideNone || !a.Interactive {
		t.Errorf("inactive tab %+v", a)
	}
	if b.Edge != layout.SideTop || b.Fill != layout.FillContent {
		t.Errorf("active tab %+v", b)
	}
	s := layout.LayoutScene(row, 400, 40, points)
	for _, op := range s.Ops {
		if bo, ok := op.(layout.BoxOp); ok && bo.ID == "b" && bo.Box.Edge == layout.SideTop {
			return
		}
	}
	t.Error("the active tab's edge is not in the scene")
}

// A segmented option's label sits in the middle of the control's height,
// not at its top.
func TestSegmentedLabelsCentered(t *testing.T) {
	s := layout.LayoutScene(Segmented{ID: "seg", Options: []string{"Off", "On"}}.Layout(), 300, 100, points)
	track := s.Rects["seg"]
	found := 0
	for _, op := range s.Ops {
		if text, ok := op.(layout.TextOp); ok {
			found++
			if text.Rect.H >= track.H {
				t.Errorf("%q stretched to the control height %v: its text draws at the top", text.Text, text.Rect.H)
			}
			top, bottom := text.Rect.Y-track.Y, track.Y+track.H-(text.Rect.Y+text.Rect.H)
			if d := top - bottom; d < -1 || d > 1 {
				t.Errorf("%q: %v above, %v below", text.Text, top, bottom)
			}
		}
	}
	if found != 2 {
		t.Fatalf("%d labels drawn", found)
	}
}

// A tab's close action sits inside the tab, after its label, on the tab's
// line; a modified tab shows the dot in its place under the same ID.
func TestTabCloseAction(t *testing.T) {
	strip := Tabs{Tabs: []Tab{{ID: "a", Label: "main.go", Close: "a:x"}, {ID: "b", Label: "go.mod", Close: "b:x", Modified: true}}}
	for _, rects := range []map[string]layout.PointRect{
		layout.LayoutScene(strip.Layout(), 600, 36, points).Rects,
		cellRects(layout.Compose(strip.Layout(), 60, 1).Rects),
	} {
		for _, id := range []string{"a", "b"} {
			tab, x := rects[id], rects[id+":x"]
			if x.W == 0 || x.X < tab.X || x.X+x.W > tab.X+tab.W || x.Y < tab.Y || x.Y+x.H > tab.Y+tab.H {
				t.Errorf("%s: action %+v not inside tab %+v", id, x, tab)
			}
		}
	}
	row := strip.Layout().(layout.Row)
	icons := func(n layout.Node) (names []layout.IconName) {
		for _, k := range n.(layout.Row).Children {
			if a, ok := k.(layout.Row); ok {
				for _, c := range a.Children {
					if i, ok := c.(layout.Icon); ok {
						names = append(names, i.Name)
					}
				}
			}
		}
		return names
	}
	if got := icons(row.Children[0]); len(got) != 1 || got[0] != layout.IconClose {
		t.Errorf("closable tab icons %v", got)
	}
	if got := icons(row.Children[1]); len(got) != 1 || got[0] != layout.IconDot {
		t.Errorf("modified tab icons %v", got)
	}
}

// Fill shares the strip: equal tab widths, no spacer.
func TestTabsFill(t *testing.T) {
	s := layout.LayoutScene(Tabs{Fill: true, Tabs: []Tab{{ID: "a", Label: "a"}, {ID: "b", Label: "a much longer name"}}}.Layout(), 400, 36, points)
	if a, b := s.Rects["a"], s.Rects["b"]; a.W != b.W || a.W+b.W < 399 {
		t.Errorf("filled tabs %v and %v", a.W, b.W)
	}
}

func cellRects(r map[string]layout.Rect) map[string]layout.PointRect {
	out := map[string]layout.PointRect{}
	for k, v := range r {
		out[k] = layout.PointRect{X: float64(v.X), Y: float64(v.Y), W: float64(v.W), H: float64(v.H)}
	}
	return out
}

// Trailing nodes sit in the middle of the strip's height, as the tabs'
// labels do.
func TestTabsTrailingCentered(t *testing.T) {
	strip := Tabs{Tabs: []Tab{{ID: "a", Label: "main.go"}}, Trailing: []layout.Node{layout.Label{Text: "cmd › demo"}}}
	s := layout.LayoutScene(strip.Layout(), 600, 36, points)
	var ys []float64
	for _, op := range s.Ops {
		if text, ok := op.(layout.TextOp); ok {
			ys = append(ys, text.Rect.Y+text.Rect.H/2)
			if text.Rect.H >= 36 { // stretched: its text would draw at the top
				t.Errorf("%q stretched to the strip's height", text.Text)
			}
		}
	}
	if len(ys) != 2 || ys[0] != ys[1] {
		t.Errorf("label centers %v, want one line", ys)
	}
}
