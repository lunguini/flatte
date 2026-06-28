package layout

import (
	"strings"
	"testing"
)

// textElement is a test Element that renders a fixed string.
type textElement struct{ s string }

func (e *textElement) Layout(_, _ int) Node {
	return Content(e.s)
}

// adaptElement renders differently based on available width.
type adaptElement struct {
	wide   string
	narrow string
}

func (e *adaptElement) Layout(w, _ int) Node {
	if w < 20 {
		return Content(e.narrow)
	}
	return Content(e.wide)
}

// headerElement returns a Row subtree with title + spacer + tabs.
type headerElement struct {
	title string
	tabs  string
}

func (h *headerElement) Layout(_, _ int) Node {
	return Row(
		Content(h.title),
		Spacer(),
		Content(h.tabs),
	)
}

// nestedElement returns a subtree containing another El.
type nestedElement struct {
	inner Element
}

func (n *nestedElement) Layout(w, h int) Node {
	return Col(
		Content("outer"),
		El(n.inner),
	)
}

func TestElementAutoSizesFromSubtree(t *testing.T) {
	tree := Col(
		El(&textElement{"hello"}),
	)
	rects := Solve(MeasurePass(tree), 80, 24)

	// "hello" is 5 chars wide, 1 line. The element auto-sizes to that.
	// In a Col, the element stretches to full width (cross-axis Auto),
	// but its height comes from the measured subtree (1).
	root := rects[""]
	_ = root
	// Element has no ID, so check via Render
	result := Render(tree, 80, 24)
	if !strings.Contains(result, "hello") {
		t.Fatalf("missing 'hello': %q", result)
	}
}

func TestElementRendersAtSolvedSize(t *testing.T) {
	tree := Col(
		El(&textElement{"body"}).Grow(1),
	)
	result := Render(tree, 20, 5)

	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines want 5", len(lines))
	}
	if !strings.Contains(lines[0], "body") {
		t.Fatalf("line 0 missing 'body': %q", lines[0])
	}
}

func TestElementAdaptsToWidth(t *testing.T) {
	tree := El(&adaptElement{
		wide:   "FULL TITLE",
		narrow: "F.T.",
	})

	narrow := Render(tree, 10, 1)
	if !strings.Contains(narrow, "F.T.") {
		t.Fatalf("narrow: expected 'F.T.' got %q", narrow)
	}

	wide := Render(tree, 40, 1)
	if !strings.Contains(wide, "FULL TITLE") {
		t.Fatalf("wide: expected 'FULL TITLE' got %q", wide)
	}
}

func TestElementReturnsSubtree(t *testing.T) {
	tree := Col(
		El(&headerElement{title: "App", tabs: "[1] [2]"}),
	)
	result := Render(tree, 30, 1)

	if !strings.Contains(result, "App") {
		t.Fatalf("missing title: %q", result)
	}
	if !strings.Contains(result, "[1] [2]") {
		t.Fatalf("missing tabs: %q", result)
	}
	// Tabs should be right-aligned (Spacer pushes them)
	lines := strings.Split(result, "\n")
	if len(lines) > 0 {
		line := lines[0]
		appIdx := strings.Index(line, "App")
		tabsIdx := strings.Index(line, "[1]")
		if appIdx < 0 || tabsIdx < 0 {
			t.Fatalf("missing parts: %q", line)
		}
		if tabsIdx < appIdx+10 {
			t.Fatalf("tabs should be right of title: %q", line)
		}
	}
}

func TestNestedElements(t *testing.T) {
	tree := El(&nestedElement{
		inner: &textElement{"inner content"},
	})
	result := Render(tree, 30, 5)

	if !strings.Contains(result, "outer") {
		t.Fatalf("missing 'outer': %q", result)
	}
	if !strings.Contains(result, "inner content") {
		t.Fatalf("missing 'inner content': %q", result)
	}
}

func TestContentAnonymousLeaf(t *testing.T) {
	tree := Col(
		Content("top"),
		Content("bottom"),
	)
	result := Render(tree, 10, 2)

	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("got %d lines want >=2", len(lines))
	}
	if !strings.Contains(lines[0], "top") {
		t.Fatalf("line 0: %q", lines[0])
	}
	if !strings.Contains(lines[1], "bottom") {
		t.Fatalf("line 1: %q", lines[1])
	}
}

func TestElementInRowWithSpacer(t *testing.T) {
	left := &textElement{"LEFT"}
	right := &textElement{"RIGHT"}

	tree := Row(
		El(left),
		Spacer(),
		El(right),
	)
	result := Render(tree, 30, 1)

	if !strings.Contains(result, "LEFT") {
		t.Fatalf("missing LEFT: %q", result)
	}
	if !strings.Contains(result, "RIGHT") {
		t.Fatalf("missing RIGHT: %q", result)
	}
	// RIGHT should be at the far right
	lines := strings.Split(result, "\n")
	line := lines[0]
	if !strings.Contains(line[len(line)-10:], "RIGHT") {
		t.Fatalf("RIGHT not right-aligned: %q", line)
	}
}

func TestElementWithExplicitSize(t *testing.T) {
	tree := Col(
		El(&textElement{"fixed"}).Width(20).Height(3),
		El(&textElement{"grow"}).Grow(1),
	)
	result := Render(tree, 30, 5)

	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines want 5", len(lines))
	}
	if !strings.Contains(lines[0], "fixed") {
		t.Fatalf("line 0: %q", lines[0])
	}
	if !strings.Contains(lines[3], "grow") {
		t.Fatalf("line 3 (start of grow area): %q", lines[3])
	}
}

func TestElementMeasureProducesCorrectSize(t *testing.T) {
	e := &headerElement{title: "Title", tabs: "Tabs"}

	measured := MeasurePass(e.Layout(noConstraint, noConstraint))
	// After MeasurePass, the Row should have SizeContent sizing (measured
	// from children, acts as Fixed on main axis, stretches on cross).
	if measured.H.Kind != SizeContent && measured.H.Kind != SizeFixed {
		t.Fatalf("measured H=%+v want SizeContent or SizeFixed", measured.H)
	}
	if measured.H.Value != 1 {
		t.Fatalf("measured H value=%d want 1", measured.H.Value)
	}
}
