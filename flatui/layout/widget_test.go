package layout

import (
	"strings"
	"testing"
)

// textWidget is a test widget producing a fixed string.
func textWidget(s string) Node {
	return Node{Layout: func(r Rect) Node {
		return Text(s)
	}}
}

// adaptWidget renders differently based on available width.
func adaptWidget(wide, narrow string) Node {
	return Node{Layout: func(r Rect) Node {
		if r.W < 20 {
			return Text(narrow)
		}
		return Text(wide)
	}}
}

// headerWidget returns a Row subtree with title + spacer + tabs.
func headerWidget(title, tabs string) Node {
	return Node{Layout: func(r Rect) Node {
		return Row(
			Text(title),
			Spacer(),
			Text(tabs),
		)
	}}
}

func TestWidgetAutoSizesFromSubtree(t *testing.T) {
	tree := Col(textWidget("hello"))
	result := Render(tree, 80, 24)
	if !strings.Contains(result, "hello") {
		t.Fatalf("missing 'hello': %q", result)
	}
}

func TestWidgetRendersAtSolvedSize(t *testing.T) {
	tree := Col(Node{Layout: func(r Rect) Node {
		return Text("body")
	}}.Grow(1))
	result := Render(tree, 20, 5)

	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines want 5", len(lines))
	}
	if !strings.Contains(lines[0], "body") {
		t.Fatalf("line 0 missing 'body': %q", lines[0])
	}
}

func TestWidgetAdaptsToWidth(t *testing.T) {
	narrow := Render(adaptWidget("FULL TITLE", "F.T."), 10, 1)
	if !strings.Contains(narrow, "F.T.") {
		t.Fatalf("narrow: expected 'F.T.' got %q", narrow)
	}
	wide := Render(adaptWidget("FULL TITLE", "F.T."), 40, 1)
	if !strings.Contains(wide, "FULL TITLE") {
		t.Fatalf("wide: expected 'FULL TITLE' got %q", wide)
	}
}

func TestWidgetReturnsSubtree(t *testing.T) {
	tree := Col(headerWidget("App", "[1] [2]"))
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

func TestNestedWidgets(t *testing.T) {
	inner := textWidget("inner content")
	outer := Node{Layout: func(r Rect) Node {
		return Col(
			Text("outer"),
			inner,
		)
	}}
	result := Render(outer, 30, 5)

	if !strings.Contains(result, "outer") {
		t.Fatalf("missing 'outer': %q", result)
	}
	if !strings.Contains(result, "inner content") {
		t.Fatalf("missing 'inner content': %q", result)
	}
}

func TestWidgetInRowWithSpacer(t *testing.T) {
	tree := Row(
		textWidget("LEFT"),
		Spacer(),
		textWidget("RIGHT"),
	)
	result := Render(tree, 30, 1)

	lines := strings.Split(result, "\n")
	line := lines[0]
	if !strings.Contains(line, "LEFT") {
		t.Fatalf("missing LEFT: %q", line)
	}
	if !strings.Contains(line[len(line)-10:], "RIGHT") {
		t.Fatalf("RIGHT not right-aligned: %q", line)
	}
}

func TestWidgetWithExplicitSize(t *testing.T) {
	tree := Col(
		textWidget("fixed").Width(20).Height(3),
		textWidget("grow").Grow(1),
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

func TestWidgetMeasureSkipsForGrowNodes(t *testing.T) {
	called := false
	tree := Col(
		Node{Layout: func(r Rect) Node {
			called = true
			return Text("should not render during measure")
		}}.Grow(1),
	)
	MeasurePass(tree)
	if called {
		t.Fatal("Layout should not be called during measure for Grow nodes")
	}
}
