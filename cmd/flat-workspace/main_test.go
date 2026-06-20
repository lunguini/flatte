package main

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatest"
	"github.com/lunguini/flat/flatui"
)

func ready() *State {
	s := NewState()
	s.layout(86, 24)
	return s
}

func key(r rune) flat.KeyEvent {
	return flat.KeyEvent{Key: flat.KeyCharacter, Rune: r}
}

func TestFocusCyclesBetweenWorkspaceSections(t *testing.T) {
	s := ready()
	if !s.focus.Focused(int(focusTree)) {
		t.Fatalf("initial focus = %d, want tree", s.focus.Index())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	if !s.focus.Focused(int(focusSearch)) {
		t.Fatalf("after tab focus = %d, want search", s.focus.Index())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	if !s.focus.Focused(int(focusDetails)) {
		t.Fatalf("second tab focus = %d, want details", s.focus.Index())
	}
	Handle(s, flat.KeyEvent{Key: flat.KeyTab, Mod: flat.ModShift}, flat.Effects[State]{})
	if !s.focus.Focused(int(focusSearch)) {
		t.Fatalf("shift-tab focus = %d, want search", s.focus.Index())
	}
}

func TestFocusTabsSpanFullWidth(t *testing.T) {
	s := ready()
	frame := flatest.CleanFrame(View(s, flat.RenderContext{Width: 86}).Content)
	line, _ := cleanLineContaining(frame, "Tree")
	for _, want := range []string{"Tree", "Search", "Details"} {
		if !strings.Contains(line, want) {
			t.Fatalf("tab line missing %q:\n%s", want, frame)
		}
	}
	if strings.Contains(line, "Work") {
		t.Fatalf("tab line includes redundant Work focus target:\n%s", frame)
	}
	if width := lipgloss.Width(line); width != 86 {
		t.Fatalf("tab line width = %d, want 86: %q", width, line)
	}
}

func TestHelpLinePinnedToBottom(t *testing.T) {
	s := ready()
	frame := flatest.CleanFrame(View(s, flat.RenderContext{Width: 86}).Content)
	lines := strings.Split(frame, "\n")
	if len(lines) != 24 {
		t.Fatalf("frame line count = %d, want terminal height 24:\n%s", len(lines), frame)
	}
	if !strings.Contains(lines[len(lines)-1], "tab focus") {
		t.Fatalf("last line is not help: %q", lines[len(lines)-1])
	}
}

func TestProgressUsesWorkPanelWidth(t *testing.T) {
	s := ready()
	_, centerOuter, _ := layoutWidths(86)
	if got, want := s.progress.Width(), centerOuter-12; got != want {
		t.Fatalf("progress width = %d, want work content width minus label = %d", got, want)
	}
}

func TestTreeExpansionChangesVisibleRows(t *testing.T) {
	s := ready()
	before := len(s.tree.VisibleRows())

	Handle(s, flat.KeyEvent{Key: flat.KeyEnter}, flat.Effects[State]{})

	if after := len(s.tree.VisibleRows()); after >= before {
		t.Fatalf("visible rows after collapsing root = %d, want less than %d", after, before)
	}
}

func TestTreeLeftRightExpandAndCollapseSelectedBranch(t *testing.T) {
	s := ready()
	Handle(s, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})
	if got := s.tree.CursorID(); got != "services" {
		t.Fatalf("selected tree node = %q, want services", got)
	}

	Handle(s, flat.KeyEvent{Key: flat.KeyRight}, flat.Effects[State]{})
	if got := treeLabels(s.tree.VisibleRows()); strings.Join(got, ",") != "workspace,services,API,Billing,Schema,ops" {
		t.Fatalf("right expanded labels = %v, want services children visible", got)
	}

	Handle(s, flat.KeyEvent{Key: flat.KeyLeft}, flat.Effects[State]{})
	if got := treeLabels(s.tree.VisibleRows()); strings.Join(got, ",") != "workspace,services,ops" {
		t.Fatalf("left collapsed labels = %v, want services children hidden", got)
	}
}

func TestSearchFiltersTableOnlyWhenFocused(t *testing.T) {
	s := ready()
	Handle(s, key('a'), flat.Effects[State]{})
	if s.search.Value != "" {
		t.Fatalf("tree-focused character edited search: %q", s.search.Value)
	}

	Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	Handle(s, key('a'), flat.Effects[State]{})
	Handle(s, key('p'), flat.Effects[State]{})
	Handle(s, key('i'), flat.Effects[State]{})

	if s.search.Value != "api" {
		t.Fatalf("search = %q, want api", s.search.Value)
	}
	if got := resultTitles(s.visibleResults()); strings.Join(got, ",") != "API gateway,API schema" {
		t.Fatalf("filtered results = %v, want API rows", got)
	}
}

func TestSearchFocusCanMoveWorkRowsAndKeepEditing(t *testing.T) {
	s := ready()
	Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})

	Handle(s, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})
	if s.table.Cursor() != 1 {
		t.Fatalf("table cursor after search-down = %d, want 1", s.table.Cursor())
	}
	if got := s.selectedResult().Title; got != "Billing sync" {
		t.Fatalf("selected title = %q, want Billing sync", got)
	}
	if !strings.Contains(s.details.View(), "Billing sync") {
		t.Fatalf("details missing selected title:\n%s", s.details.View())
	}

	Handle(s, key('a'), flat.Effects[State]{})
	if s.search.Value != "a" {
		t.Fatalf("search after row move = %q, want a", s.search.Value)
	}
	if !s.focus.Focused(int(focusSearch)) {
		t.Fatalf("focus after search row move = %d, want search", s.focus.Index())
	}
}

func TestSearchSelectionUpdatesDetailsAndProgress(t *testing.T) {
	s := ready()
	Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	Handle(s, flat.KeyEvent{Key: flat.KeyDown}, flat.Effects[State]{})

	if s.table.Cursor() != 1 {
		t.Fatalf("table cursor = %d, want 1", s.table.Cursor())
	}
	if got := s.selectedResult().Title; got != "Billing sync" {
		t.Fatalf("selected title = %q, want Billing sync", got)
	}
	if s.progress.Percent() != 42 {
		t.Fatalf("progress = %.0f, want 42", s.progress.Percent())
	}
	if !strings.Contains(s.details.View(), "Billing sync") {
		t.Fatalf("details missing selected title:\n%s", s.details.View())
	}
}

func TestSearchCursorTracksTypedText(t *testing.T) {
	s := ready()
	Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	Handle(s, key('a'), flat.Effects[State]{})
	Handle(s, key('p'), flat.Effects[State]{})
	Handle(s, key('i'), flat.Effects[State]{})

	frame := View(s, flat.RenderContext{Width: 86})
	if frame.Cursor == nil {
		t.Fatal("search-focused frame has no cursor")
	}
	line, y := cleanLineContaining(frame.Content, "search: api")
	idx := strings.Index(line, "search: api")
	wantX := lipgloss.Width(line[:idx]) + len("search: api")
	if frame.Cursor.X != wantX || frame.Cursor.Y != y {
		t.Fatalf("cursor = %+v, want (%d,%d) on line %q", *frame.Cursor, wantX, y, line)
	}
}

func TestFocusDetailsIsVisibleAndDescribesScroll(t *testing.T) {
	s := ready()
	for range 2 {
		Handle(s, flat.KeyEvent{Key: flat.KeyTab}, flat.Effects[State]{})
	}

	frame := flatest.CleanFrame(View(s, flat.RenderContext{Width: 86}).Content)
	for _, want := range []string{"focus details", "[details scroll]", "j/k scroll"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("details-focused frame missing %q:\n%s", want, frame)
		}
	}
}

func TestTreeRowsShowExpansionMarkers(t *testing.T) {
	s := ready()
	frame := flatest.CleanFrame(View(s, flat.RenderContext{Width: 86}).Content)
	for _, want := range []string{"▾ workspace", " ▸ services", " ▸ ops"} {
		if !strings.Contains(frame, want) {
			t.Fatalf("tree frame missing %q:\n%s", want, frame)
		}
	}
}

func TestStyledProgressRendersANSI(t *testing.T) {
	s := ready()
	frame := View(s, flat.RenderContext{Width: 86}).Content
	if !strings.Contains(frame, "\x1b[") {
		t.Fatalf("view has no ANSI styling:\n%s", frame)
	}
	if !strings.Contains(flatest.CleanFrame(frame), "68%") {
		t.Fatalf("view missing progress label:\n%s", flatest.CleanFrame(frame))
	}
}

func TestNarrowFrameFitsWidth(t *testing.T) {
	s := NewState()
	s.layout(70, 22)
	frame := flatest.CleanFrame(View(s, flat.RenderContext{Width: 70}).Content)
	for i, line := range strings.Split(frame, "\n") {
		if width := lipgloss.Width(line); width > 70 {
			t.Fatalf("line %d width = %d, want <= 70:\n%s", i+1, width, frame)
		}
	}
}

func TestWorkspaceSnapshot(t *testing.T) {
	s := ready()
	flatest.AssertGoldenFrame(t, "testdata/workspace.golden", View(s, flat.RenderContext{Width: 86}))
}

func resultTitles(results []WorkItem) []string {
	out := make([]string, 0, len(results))
	for _, result := range results {
		out = append(out, result.Title)
	}
	return out
}

func treeLabels(rows []flatui.TreeRow) []string {
	out := make([]string, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.Label)
	}
	return out
}

func cleanLineContaining(content, needle string) (line string, y int) {
	for i, line := range strings.Split(flatest.CleanFrame(content), "\n") {
		if strings.Contains(line, needle) {
			return line, i
		}
	}
	return "", -1
}
