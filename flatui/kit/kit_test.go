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
