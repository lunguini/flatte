package layout

import (
	"strings"
	"testing"
)

func TestMeasureString(t *testing.T) {
	w, h := measureString("hello")
	if w != 5 || h != 1 {
		t.Fatalf("single line: w=%d h=%d want 5,1", w, h)
	}

	w, h = measureString("line one\nline two")
	if w != 8 || h != 2 {
		t.Fatalf("two lines: w=%d h=%d want 8,2", w, h)
	}

	w, h = measureString("\x1b[1mbold\x1b[0m")
	if w != 4 || h != 1 {
		t.Fatalf("styled: w=%d h=%d want 4,1 (ANSI stripped)", w, h)
	}
}

func TestMeasurePassLeafAutoSizes(t *testing.T) {
	n := MeasurePass(Text("hello"))
	if n.W.Kind != SizeContent || n.W.Value != 5 {
		t.Fatalf("W=%+v want SizeContent(5)", n.W)
	}
	if n.H.Kind != SizeContent || n.H.Value != 1 {
		t.Fatalf("H=%+v want SizeContent(1)", n.H)
	}
}

func TestMeasurePassLeafRespectsExplicitSize(t *testing.T) {
	n := MeasurePass(Text("hello").Width(20).Height(3))
	if n.W.Kind != SizeFixed || n.W.Value != 20 {
		t.Fatalf("W=%+v want Fixed(20) (explicit)", n.W)
	}
	if n.H.Kind != SizeFixed || n.H.Value != 3 {
		t.Fatalf("H=%+v want Fixed(3) (explicit)", n.H)
	}
}

func TestMeasurePassLeafWithPadding(t *testing.T) {
	n := MeasurePass(Text("hello").Padding(1))
	if n.W.Kind != SizeContent || n.W.Value != 7 {
		t.Fatalf("W=%+v want SizeContent(7) (5+2*pad)", n.W)
	}
	if n.H.Kind != SizeContent || n.H.Value != 3 {
		t.Fatalf("H=%+v want SizeContent(3) (1+2*pad)", n.H)
	}
}

func TestMeasurePassRowAutoHeight(t *testing.T) {
	// Row with two content children: height = max(child heights)
	n := MeasurePass(Row(
		Text("x"),
		Text("y\nz"),
	))
	if n.H.Kind != SizeContent || n.H.Value != 2 {
		t.Fatalf("Row H=%+v want SizeContent(2) (max child height)", n.H)
	}
}

func TestMeasurePassColAutoHeight(t *testing.T) {
	n := MeasurePass(Col(
		Text("x"),
		Text("y"),
	))
	if n.H.Kind != SizeContent || n.H.Value != 2 {
		t.Fatalf("Col H=%+v want SizeContent(2) (sum child heights)", n.H)
	}
}

func TestMeasurePassSpacerDoesNotAffectHeight(t *testing.T) {
	n := MeasurePass(Row(
		Text("hello"),
		Spacer(),
		Text("tabs"),
	))
	if n.H.Kind != SizeContent || n.H.Value != 1 {
		t.Fatalf("Row with Spacer H=%+v want SizeContent(1)", n.H)
	}
}

func TestSpacerFillsRemainingWidth(t *testing.T) {
	rects := Solve(Row(
		Box("left").Width(10),
		Spacer(),
		Box("right").Width(10),
	), 80, 1)

	if rects["left"].W != 10 {
		t.Fatalf("left.W=%d want 10", rects["left"].W)
	}
	if rects["right"].X != 70 {
		t.Fatalf("right.X=%d want 70 (pushed by spacer)", rects["right"].X)
	}
}

func TestRenderBasicCol(t *testing.T) {
	tree := Col(
		Text("HEADER"),
		Text("BODY"),
		Text("FOOTER"),
	)
	result := Render(tree, 20, 3)

	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("Render produced %d lines, want 3: %q", len(lines), result)
	}
	if !strings.Contains(lines[0], "HEADER") {
		t.Fatalf("line 0 = %q want HEADER", lines[0])
	}
	if !strings.Contains(lines[1], "BODY") {
		t.Fatalf("line 1 = %q want BODY", lines[1])
	}
	if !strings.Contains(lines[2], "FOOTER") {
		t.Fatalf("line 2 = %q want FOOTER", lines[2])
	}
}

func TestRenderRowWithSpacer(t *testing.T) {
	tree := Row(
		Text("Title"),
		Spacer(),
		Text("Tabs"),
	)
	result := Render(tree, 20, 1)

	if !strings.Contains(result, "Title") {
		t.Fatalf("missing Title: %q", result)
	}
	if !strings.Contains(result, "Tabs") {
		t.Fatalf("missing Tabs: %q", result)
	}
	// Title should be at the left, Tabs at the right
	if strings.Index(result, "Title") > 0 {
		t.Fatalf("Title not at left: %q", result)
	}
	// Tabs should be at the far right (position 20-4=16)
	if !strings.Contains(result[16:], "Tabs") {
		t.Fatalf("Tabs not at right edge: %q", result)
	}
}

func TestRenderAutoHeightFromContent(t *testing.T) {
	// A Col with content children: height auto-sizes, no explicit Height needed.
	tree := Col(
		Text("AAA"),
		Text("BBB"),
	)
	result := Render(tree, 10, 5)

	lines := strings.Split(result, "\n")
	if len(lines) < 2 {
		t.Fatalf("Render produced %d lines, want >=2: %q", len(lines), result)
	}
	if !strings.Contains(lines[0], "AAA") {
		t.Fatalf("line 0 = %q want AAA", lines[0])
	}
	if !strings.Contains(lines[1], "BBB") {
		t.Fatalf("line 1 = %q want BBB", lines[1])
	}
}

func TestRenderGrowChildFillsRemaining(t *testing.T) {
	tree := Col(
		Text("H"),
		Row(
			Box("sidebar").Width(10),
			Box("content").Grow(1),
		).Grow(1),
		Text("F"),
	)
	result := Render(tree, 40, 5)

	lines := strings.Split(result, "\n")
	if len(lines) != 5 {
		t.Fatalf("Render produced %d lines, want 5", len(lines))
	}
	if !strings.Contains(lines[0], "H") {
		t.Fatalf("line 0 missing H: %q", lines[0])
	}
	if !strings.Contains(lines[4], "F") {
		t.Fatalf("line 4 missing F: %q", lines[4])
	}
}

func TestRenderEmptyContentFillsRect(t *testing.T) {
	tree := Col(
		Box("empty").Height(3),
	)
	result := Render(tree, 10, 3)

	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("Render produced %d lines, want 3", len(lines))
	}
}

func TestRenderWithBorder(t *testing.T) {
	tree := Col(
		Text("hello").Border(),
	)
	result := Render(tree, 10, 3)

	// Border should add visible characters
	if !strings.Contains(result, "╭") || !strings.Contains(result, "╮") {
		t.Fatalf("missing border corners: %q", result)
	}
}

func TestRenderNestedTree(t *testing.T) {
	tree := Col(
		Text("HEADER"),
		Row(
			Text("L"),
			Spacer(),
			Text("R"),
		),
		Text("FOOTER"),
	)
	result := Render(tree, 20, 3)

	lines := strings.Split(result, "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines want 3", len(lines))
	}
	if !strings.Contains(lines[0], "HEADER") {
		t.Fatalf("line 0: %q", lines[0])
	}
	if !strings.Contains(lines[1], "L") || !strings.Contains(lines[1], "R") {
		t.Fatalf("line 1 missing L/R: %q", lines[1])
	}
	if !strings.Contains(lines[2], "FOOTER") {
		t.Fatalf("line 2: %q", lines[2])
	}
	// L should be left-aligned, R right-aligned
	if strings.Index(lines[1], "L") > 0 {
		t.Fatalf("L not at left: %q", lines[1])
	}
}

func TestRenderMatchesSolve(t *testing.T) {
	// The rendered output should have the same dimensions that Solve computed.
	tree := Col(
		Box("header").Height(3),
		Row(
			Box("sidebar").Width(20),
			Box("content").Grow(1),
		).Grow(1),
		Box("footer").Height(1),
	)

	rects := Solve(tree, 80, 24)
	result := Render(tree, 80, 24)
	lines := strings.Split(result, "\n")

	// The rendered height should match the solved height.
	if len(lines) != 24 {
		t.Fatalf("Render height=%d lines, Solve says 24", len(lines))
	}

	// Each line should be 80 wide (or close — lipgloss may trim).
	_ = rects // verify rects exist for the IDs we expect
	if _, ok := rects["header"]; !ok {
		t.Fatal("missing header in rects")
	}
}
