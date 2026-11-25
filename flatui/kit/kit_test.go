package kit

import (
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

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
	a, b := row.Children[0].(layout.Row).Box, row.Children[2].(layout.Row).Box // the gap between
	if a.Edge != layout.SideNone || !a.Interactive {
		t.Errorf("inactive tab %+v", a)
	}
	if b.Edge != layout.SideTop || b.Fill != layout.FillContent {
		t.Errorf("active tab %+v", b)
	}
	// Tabs and the strip stand on the panel below: top corners round.
	for _, box := range []layout.Box{a, b, row.Box} {
		if !box.Round || box.Flat != layout.SideBottom {
			t.Errorf("not rounded on top only: %+v", box)
		}
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
	if got := icons(row.Children[2]); len(got) != 1 || got[0] != layout.IconDot {
		t.Errorf("modified tab icons %v", got)
	}
}

// Fill shares the strip: equal tab widths, no spacer.
func TestTabsFill(t *testing.T) {
	s := layout.LayoutScene(Tabs{Fill: true, Tabs: []Tab{{ID: "a", Label: "a"}, {ID: "b", Label: "a much longer name"}}}.Layout(), 400, 36, points)
	if a, b := s.Rects["a"], s.Rects["b"]; a.W != b.W || a.W+b.W != 400-TabGap {
		t.Errorf("filled tabs %v and %v", a.W, b.W)
	}
}

// A hairline stands between tabs, seen on the strip.
func TestTabsStandApart(t *testing.T) {
	strip := Tabs{Tabs: []Tab{{ID: "a", Label: "a.go"}, {ID: "b", Label: "b.go"}, {ID: "c", Label: "c.go"}}, Active: 0}
	s := layout.LayoutScene(strip.Layout(), 600, 36, points).Rects
	if d := s["b"].X - s["a"].X - s["a"].W; d != TabGap {
		t.Errorf("%v points between tabs", d)
	}
	c := layout.Compose(strip.Layout(), 60, 1).Rects
	if d := c["c"].X - c["b"].X - c["b"].W; d != 1 {
		t.Errorf("%v columns between tabs", d)
	}
	if b := strip.Layout().(layout.Row).Children[1].(layout.Col).Box; b.Fill != layout.FillSeparator {
		t.Errorf("between tabs: %+v", b)
	}
	strip.Surface = layout.FillField
	if b := strip.Layout().(layout.Row).Children[0].(layout.Row).Box; b.Fill != layout.FillField {
		t.Errorf("the active tab on a field: %+v", b)
	}
}

// A scrolling strip never squeezes its tabs: they keep their names' width
// (or, sharing the room, MinTabCols) and the rest is scrolled to.
func TestTabsScroll(t *testing.T) {
	var tabs []Tab
	for i := range 10 {
		tabs = append(tabs, Tab{ID: string(rune('a' + i)), Label: "some_file_name.go", Close: string(rune('a'+i)) + "x"})
	}
	plus := Action{ID: "new", Icon: layout.IconPlus}.Layout()
	one := layout.LayoutScene(Tabs{ID: "s", Scroll: true, Tabs: tabs[:1], Trailing: []layout.Node{plus}}.Layout(), 400, 36, points).Rects
	if n, a := one["new"], one["a"]; n.W == 0 || n.X != a.X+a.W {
		t.Errorf("+ at %v, the tab at %v: want it right after", n, a)
	}
	for _, fill := range []bool{false, true} {
		strip := Tabs{ID: "s", Scroll: true, Fill: fill, Tabs: tabs, Trailing: []layout.Node{plus}}
		r := layout.LayoutScene(strip.Layout(), 400, 36, points).Rects
		view, content := r[strip.ViewID()], r[strip.ContentID()]
		if view.W > 400 || content.W <= view.W {
			t.Fatalf("fill %v: view %v content %v", fill, view, content)
		}
		want := float64(MinTabCols) * points.Column
		if !fill {
			want = one["a"].W
		}
		if d := r["j"].W - want; !fill && d != 0 || d < -TabGap || d > 0 {
			t.Errorf("fill %v: the last tab is %v wide, want %v", fill, r["j"].W, want)
		}
		if fill && (r["new"].X+r["new"].W > 400 || r["new"].W == 0) {
			t.Errorf("crowded, + is out of sight: %v", r["new"])
		}
		strip.Offset = 1000
		r = layout.LayoutScene(strip.Layout(), 400, 36, points).Rects
		if j := r["j"]; j.X+j.W > view.X+view.W+.01 {
			t.Errorf("fill %v: scrolled to the end, the last tab ends at %v of %v", fill, j.X+j.W, view.X+view.W)
		}
		scene := layout.LayoutScene(strip.Layout(), 400, 36, points)
		if id, _ := scene.HitTest(view.X+1, 10); id == "a" || id == "" {
			t.Errorf("fill %v: the first point hits %q", fill, id)
		}
	}
	// In a terminal the window clips whole cells.
	strip := Tabs{ID: "s", Scroll: true, Tabs: tabs, Offset: 3}
	c := layout.Compose(strip.Layout(), 40, 1)
	if r := c.Rects["a"]; r.X != -3 {
		t.Errorf("scrolled by 3: the first tab at %d", r.X)
	}
}

// Closing a tab by its close action leaves the next tab's right there.
func TestHoldTabs(t *testing.T) {
	widths := []float64{100, 150, 120}
	for _, tc := range []struct {
		i    int
		fill bool
		want []float64
	}{
		{0, false, []float64{100, 120}}, {1, false, []float64{100, 150}}, {2, false, []float64{100, 271}},
		{2, true, []float64{371.0 / 2, 371.0 / 2}}, {3, false, nil}, {-1, false, nil},
	} {
		if got := HoldTabs(widths, tc.i, 1, 300, tc.fill); !slices.Equal(got, tc.want) {
			t.Errorf("%d fill %v: %v, want %v", tc.i, tc.fill, got, tc.want)
		}
	}
	if got := HoldTabs([]float64{100, 250}, 1, 1, 300, false); got[0] != 300 {
		t.Errorf("past the limit: %v", got)
	}
	if got := HoldTabs([]float64{100}, 0, 1, 300, false); len(got) != 0 {
		t.Errorf("the only tab: %v", got)
	}
	if !slices.Equal(widths, []float64{100, 150, 120}) {
		t.Errorf("the caller's widths changed: %v", widths)
	}
	// Laid out so, the close actions coincide, scrolled to the end too.
	strip := Tabs{ID: "s", Scroll: true, Offset: 1000}
	for i := range 8 {
		id := string(rune('a' + i))
		strip.Tabs = append(strip.Tabs, Tab{ID: id, Label: strings.Repeat(id, 4+3*i), Close: id + "x"})
	}
	before := layout.LayoutScene(strip.Layout(), 400, 36, points).Rects
	var ws []float64
	for _, tab := range strip.Tabs {
		ws = append(ws, before[tab.ID].W)
	}
	content := before[strip.ContentID()]
	after := strip
	after.Tabs = slices.Delete(slices.Clone(strip.Tabs), 5, 6)
	for j, w := range HoldTabs(ws, 5, TabGap, 300, false) {
		after.Tabs[j].W = layout.Points(int(math.Round(w)))
	}
	after.Held = int(math.Ceil(content.W / points.Column))
	after.Offset = int((before[strip.ViewID()].X - content.X) / points.Column)
	s := layout.LayoutScene(after.Layout(), 400, 36, points)
	if a, b := before["fx"], s.Rects["gx"]; math.Abs(a.X-b.X) > points.Column {
		t.Errorf("closed at %v, the next close action is at %v", a.X, b.X)
	}
	if b := after.Layout().(layout.Row).Box; !b.ReportLeave {
		t.Error("a held strip does not report the pointer leaving")
	}
}

// A divider's title is between two equal lines, away from the rows around.
func TestDivider(t *testing.T) {
	col := layout.Col{Children: []layout.Node{
		ListRow{ID: "above", Title: "a"}.Layout(), Divider{ID: "d", Title: "Remote"}.Layout(), ListRow{ID: "below", Title: "b"}.Layout(), layout.NewSpacer(),
	}}
	s := layout.LayoutScene(col, 300, 200, points)
	var lines []layout.BoxOp
	var title layout.TextOp
	for _, op := range s.Ops {
		switch o := op.(type) {
		case layout.BoxOp:
			if o.Box.Fill == layout.FillPressed {
				lines = append(lines, o)
			}
		case layout.TextOp:
			if strings.EqualFold(o.Text, "Remote") {
				title = o
			}
		}
	}
	d := s.Rects["d"]
	if len(lines) != 2 || lines[0].Rect.W != lines[1].Rect.W || lines[0].Rect.H != 1 || lines[0].Rect.W <= 0 {
		t.Fatalf("lines %+v", lines)
	}
	if mid := lines[0].Rect.Y; math.Abs(mid-(d.Y+d.H/2)) > 1 || title.Rect.X < lines[0].Rect.X+lines[0].Rect.W {
		t.Errorf("line at %v in %+v, title %+v", mid, d, title.Rect)
	}
	if above := title.Rect.Y - d.Y; above < 4 {
		t.Errorf("%v above the title", above)
	}
	if first := layout.LayoutScene(Divider{ID: "d", Title: "x", First: true}.Layout(), 300, 200, points).Rects["d"]; first.H > 200 {
		t.Errorf("first %+v", first)
	}
	frame := layout.Compose(Divider{Title: "Remote"}.Layout(), 20, 1).Frame
	if !strings.Contains(frame, "───") || !strings.Contains(strings.ToLower(frame), "remote") {
		t.Errorf("terminal: %q", frame)
	}
	if plain := layout.Compose(Divider{}.Layout(), 10, 1).Frame; strings.Count(plain, "─") != 8 {
		t.Errorf("untitled: %q", plain)
	}
}

// End nodes sit at the strip's far end.
func TestTabsEnd(t *testing.T) {
	end := []layout.Node{layout.Label{NodeBase: layout.NodeBase{ID: "where"}, Text: "internal"}}
	for _, scroll := range []bool{false, true} {
		r := layout.LayoutScene(Tabs{ID: "s", Scroll: scroll, Tabs: []Tab{{ID: "a", Label: "a"}}, End: end}.Layout(), 400, 36, points).Rects["where"]
		if r.X+r.W != 400-points.Step {
			t.Errorf("scroll %v: ends at %v", scroll, r.X+r.W)
		}
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

// A tree row's chevron sits inside the row's left padding, not on its edge,
// and a leaf keeps the chevron's room so its icon lines up with a folder's.
func TestTreeRowDisclosure(t *testing.T) {
	col := layout.Col{Children: []layout.Node{
		ListRow{ID: "dir", Icon: layout.IconFolder, Title: "cmd", Disclosure: DisclosureOpen}.Layout(),
		ListRow{ID: "file", Icon: layout.IconFile, Title: "main.go", Disclosure: DisclosureLeaf, TitleColor: layout.ColorSecondary}.Layout(),
	}}
	s := layout.LayoutScene(col, 300, 100, points)
	var icons []layout.IconOp
	for _, op := range s.Ops {
		if i, ok := op.(layout.IconOp); ok {
			icons = append(icons, i)
		}
	}
	// dir: chevron, folder; file: blank, file.
	if len(icons) < 3 {
		t.Fatalf("%d icons drawn", len(icons))
	}
	row := s.Rects["dir"]
	if chev := icons[0]; chev.Name != layout.IconDisclosureOpen || chev.Rect.X < row.X+points.Step {
		t.Errorf("chevron %+v at the row's edge (row x %v)", chev, row.X)
	}
	var folder, file layout.IconOp
	for _, i := range icons {
		switch i.Name {
		case layout.IconFolder:
			folder = i
		case layout.IconFile:
			file = i
		}
	}
	if folder.Rect.X != file.Rect.X {
		t.Errorf("leaf icon at %v, folder icon at %v", file.Rect.X, folder.Rect.X)
	}
	// Still clickable with a dimmed title.
	if !(ListRow{TitleColor: layout.ColorDisabled}.Layout().(layout.Row).Box.Interactive) {
		t.Error("a dimmed row stopped hovering")
	}
	// Terminal: "▾" one cell in, after the padding.
	c := layout.Compose(col, 30, 2)
	if !strings.HasPrefix(stripANSI(c.Frame), " ▾") {
		t.Errorf("cells: %q", c.Frame)
	}
}

func stripANSI(s string) string { return ansi.Strip(s) }

// A pane header's Body sits between the title and the actions and takes
// the room; Active brightens the caption.
func TestPaneHeaderBodyAndActive(t *testing.T) {
	h := PaneHeader{ID: "h", Title: "Terminal", Active: true,
		Body:    layout.Label{NodeBase: layout.NodeBase{ID: "body"}, Text: "zsh"},
		Actions: []layout.Node{Action{ID: "hide", Icon: layout.IconChevronDown}.Layout()}}
	s := layout.LayoutScene(h.Layout(), 400, 36, points)
	body, hide := s.Rects["body"], s.Rects["hide"]
	if body.X <= 0 || hide.X <= body.X || hide.X+hide.W > 400 {
		t.Errorf("body %+v, action %+v", body, hide)
	}
	cap := h.Layout().(layout.Row).Children[0].(layout.Label)
	if cap.Style.Color != layout.ColorPrimary {
		t.Errorf("active caption %+v", cap.Style)
	}
}

// Inline empty states sit at the top left and wrap their hint.
func TestInlineEmptyState(t *testing.T) {
	e := EmptyState{ID: "e", Title: "No agent sessions", Hint: "Run claude in a terminal; its sessions show here with their state.", Inline: true}
	s := layout.LayoutScene(layout.Col{Children: []layout.Node{e.Layout(), layout.NewSpacer()}}, 200, 400, points)
	var hint layout.TextOp
	for _, op := range s.Ops {
		if t, ok := op.(layout.TextOp); ok && t.Wrap {
			hint = t
		}
	}
	if hint.Text == "" || hint.Rect.W > 200 {
		t.Fatalf("hint not wrapped: %+v", hint)
	}
	if r := s.Rects["e"]; r.Y != 0 || r.H > 200 {
		t.Errorf("inline state %+v, want at the top, content-tall", r)
	}
}

// A dimmed row reads normally while it is the focused selection.
func TestDimmedRowSelected(t *testing.T) {
	r := ListRow{Title: "build", TitleColor: layout.ColorDisabled, Selected: true, Active: true}.Layout().(layout.Row)
	for _, k := range r.Children {
		if l, ok := k.(layout.Label); ok && l.Style.Color == layout.ColorDisabled {
			t.Errorf("selected title still disabled-grey: %+v", l.Style)
		}
	}
}

// A pane header stands on its pane like a tab bar: top corners round.
func TestPaneHeaderRoundsItsTop(t *testing.T) {
	b := PaneHeader{Title: "Files"}.Layout().(layout.Row).Box
	if !b.Round || b.Flat != layout.SideBottom || b.Fill != layout.FillChrome {
		t.Fatalf("pane header box %+v", b)
	}
}

// An inline empty state sits in a pane body, which owns the inset
// (layout.NodeBase.PanePadding): it adds none of its own.
func TestInlineEmptyStateLeavesTheInsetToItsPane(t *testing.T) {
	col := EmptyState{Title: "Nothing", Hint: "Here", Inline: true}.Layout().(layout.Col)
	if nb := col.NodeBase; nb.Pad != 0 || nb.PadTop != 0 || nb.PadLeft != 0 || nb.PadRight != 0 || nb.PadBottom != 0 {
		t.Errorf("inline empty state pads itself: %+v", nb)
	}
}

// An unfocused strip marks its active tab with a neutral edge, so the
// focused pane's tab is the only accented one.
func TestUnfocusedTabsMarkTheActiveTabQuietly(t *testing.T) {
	edge := func(tabs Tabs) layout.Box {
		row := tabs.Layout().(layout.Row)
		for _, k := range row.Children {
			if r, ok := k.(layout.Row); ok && r.ID == "t0" {
				return r.Box
			}
		}
		t.Fatalf("no tab t0 in %+v", row)
		return layout.Box{}
	}
	tabs := Tabs{ID: "tabs", Active: 0, Tabs: []Tab{{ID: "t0", Label: "a"}, {ID: "t1", Label: "b"}}}
	if b := edge(tabs); b.Edge != layout.SideTop || b.EdgeFill != layout.FillNone {
		t.Errorf("focused: edge %v fill %v, want the accent", b.Edge, b.EdgeFill)
	}
	tabs.Unfocused = true
	if b := edge(tabs); b.Edge != layout.SideTop || b.EdgeFill != layout.FillBorder {
		t.Errorf("unfocused: edge %v fill %v, want FillBorder", b.Edge, b.EdgeFill)
	}
}
