package layout

import (
	"reflect"
	"testing"
)

func TestBuilderColWithChildren(t *testing.T) {
	tree := Col(
		Box("header").Height(3).Border(),
		Row(
			Box("sidebar").Width(20),
			Box("content").Grow(1),
		).Grow(1),
		Box("footer").Height(1),
	)

	if tree.Dir != ColDir {
		t.Fatalf("root.Dir=%v want ColDir", tree.Dir)
	}
	if len(tree.Children) != 3 {
		t.Fatalf("root has %d children, want 3", len(tree.Children))
	}

	header := tree.Children[0]
	if header.ID != "header" || header.Dir != LeafDir {
		t.Fatalf("header: %+v", header)
	}
	if header.H != Fixed(3) {
		t.Fatalf("header.H=%+v want Fixed(3)", header.H)
	}
	if !header.Bordered {
		t.Fatalf("header should have border")
	}

	main := tree.Children[1]
	if main.Dir != RowDir || main.H.Kind != SizeGrow {
		t.Fatalf("main: Dir=%v H=%+v want RowDir/Grow", main.Dir, main.H)
	}
	if len(main.Children) != 2 {
		t.Fatalf("main has %d children, want 2", len(main.Children))
	}

	sidebar := main.Children[0]
	if sidebar.W != Fixed(20) {
		t.Fatalf("sidebar.W=%+v want Fixed(20)", sidebar.W)
	}

	content := main.Children[1]
	if content.W.Kind != SizeGrow || content.W.Weight != 1 {
		t.Fatalf("content.W=%+v want Grow(1)", content.W)
	}

	footer := tree.Children[2]
	if footer.H != Fixed(1) {
		t.Fatalf("footer.H=%+v want Fixed(1)", footer.H)
	}
}

func TestBuilderGrowSetsBothAxes(t *testing.T) {
	n := Box("test").Grow(2)
	if n.W.Kind != SizeGrow || n.W.Weight != 2 {
		t.Fatalf("W=%+v want Grow(2)", n.W)
	}
	if n.H.Kind != SizeGrow || n.H.Weight != 2 {
		t.Fatalf("H=%+v want Grow(2)", n.H)
	}
}

func TestBuilderGrowThenHeightOverrides(t *testing.T) {
	n := Box("toolbar").Grow(1).Height(3)
	if n.W.Kind != SizeGrow {
		t.Fatalf("W=%+v want Grow", n.W)
	}
	if n.H != Fixed(3) {
		t.Fatalf("H=%+v want Fixed(3) (Height overrides Grow)", n.H)
	}
}

func TestBuilderGrowThenWidthOverrides(t *testing.T) {
	n := Box("panel").Grow(1).Width(30)
	if n.H.Kind != SizeGrow {
		t.Fatalf("H=%+v want Grow", n.H)
	}
	if n.W != Fixed(30) {
		t.Fatalf("W=%+v want Fixed(30) (Width overrides Grow)", n.W)
	}
}

func TestBuilderPaddingAndSpacing(t *testing.T) {
	n := Col(
		Box("a"),
		Box("b"),
	).Padding(2).Spacing(1)

	if n.Pad != 2 {
		t.Fatalf("Pad=%d want 2", n.Pad)
	}
	if n.Gap != 1 {
		t.Fatalf("Gap=%d want 1", n.Gap)
	}
}

func TestBuilderBorder(t *testing.T) {
	n := Box("card").Border()
	if !n.Bordered {
		t.Fatalf("Bordered=false want true")
	}
}

func TestBuilderLayer(t *testing.T) {
	n := Box("modal").Width(40).Height(10).Layer()
	if !n.Overlay {
		t.Fatalf("Overlay=false want true")
	}
}

func TestBuilderProducesSameSolveAsDirect(t *testing.T) {
	// Build via builder API.
	tree1 := Col(
		Box("header").Height(3),
		Row(
			Box("sidebar").Width(20),
			Box("content").Grow(1),
		).Grow(1),
		Box("footer").Height(1),
	)

	// Build via direct construction.
	tree2 := Node{
		ID:  "",
		Dir: ColDir,
		Children: []Node{
			{ID: "header", Dir: LeafDir, H: Fixed(3)},
			{ID: "", Dir: RowDir, H: Grow(1), W: Grow(1), Children: []Node{
				{ID: "sidebar", Dir: LeafDir, W: Fixed(20)},
				{ID: "content", Dir: LeafDir, W: Grow(1), H: Grow(1)},
			}},
			{ID: "footer", Dir: LeafDir, H: Fixed(1)},
		},
	}

	r1 := Solve(tree1, 80, 24)
	r2 := Solve(tree2, 80, 24)

	// Both should produce identical geometry (ignoring the container IDs
	// that the builder doesn't set).
	for _, id := range []string{"header", "sidebar", "content", "footer"} {
		if !reflect.DeepEqual(r1[id], r2[id]) {
			t.Fatalf("node %q: builder=%+v direct=%+v", id, r1[id], r2[id])
		}
	}
}

func TestSlotAndInjectBasic(t *testing.T) {
	chrome := Col(
		Box("header").Height(3),
		Slot("content"),
		Box("footer").Height(1),
	)

	view := Row(
		Box("sidebar").Width(20),
		Box("detail").Grow(1),
	)

	filled := Inject(chrome, "content", view)

	if filled.Dir != ColDir {
		t.Fatalf("filled.Dir=%v want ColDir", filled.Dir)
	}
	if len(filled.Children) != 3 {
		t.Fatalf("filled has %d children, want 3", len(filled.Children))
	}

	middle := filled.Children[1]
	if middle.Dir != RowDir {
		t.Fatalf("middle.Dir=%v want RowDir (injected)", middle.Dir)
	}
	if len(middle.Children) != 2 {
		t.Fatalf("injected view has %d children, want 2", len(middle.Children))
	}
}

func TestSlotUnfilledBehavesAsEmpty(t *testing.T) {
	chrome := Col(
		Box("header").Height(3),
		Slot("content"),
		Box("footer").Height(1),
	)

	rects := Solve(chrome, 80, 24)

	// Unfilled slot gets 0 height (Auto on main axis = take nothing).
	header := rects["header"]
	footer := rects["footer"]
	if header.H != 3 {
		t.Fatalf("header.H=%d want 3", header.H)
	}
	if footer.H != 1 {
		t.Fatalf("footer.H=%d want 1", footer.H)
	}
	// Header + footer = 4px. The remaining 20px is just empty (slot has no
	// Grow constraint, so it doesn't claim the space).
	if footer.Y != 3 {
		t.Fatalf("footer.Y=%d want 3 (slot takes 0)", footer.Y)
	}
}

func TestInjectNested(t *testing.T) {
	root := Col(
		Row(
			Box("a"),
			Slot("dynamic"),
		),
	)

	injected := Inject(root, "dynamic", Box("injected-item"))

	if len(injected.Children) != 1 {
		t.Fatalf("root has %d children, want 1", len(injected.Children))
	}
	row := injected.Children[0]
	if len(row.Children) != 2 {
		t.Fatalf("row has %d children, want 2", len(row.Children))
	}
	if row.Children[1].ID != "injected-item" {
		t.Fatalf("second child ID=%q want 'injected-item'", row.Children[1].ID)
	}
}

func TestInjectNoMatchReturnsOriginal(t *testing.T) {
	root := Col(
		Box("a"),
		Box("b"),
	)

	result := Inject(root, "nonexistent", Box("c"))

	if len(result.Children) != 2 {
		t.Fatalf("result has %d children, want 2 (unchanged)", len(result.Children))
	}
}

func TestInjectMultipleSlots(t *testing.T) {
	root := Col(
		Slot("a"),
		Box("middle"),
		Slot("a"),
	)

	result := Inject(root, "a", Box("filled"))

	count := 0
	for _, c := range result.Children {
		if c.ID == "filled" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("found %d 'filled' children, want 2", count)
	}
}

func TestBuilderEndToEndSolve(t *testing.T) {
	tree := Col(
		Box("header").Height(3).Border(),
		Row(
			Box("sidebar").Width(20),
			Box("content").Grow(1),
		).Grow(1),
		Box("footer").Height(1),
	)

	rects := Solve(tree, 80, 24)

	assertRects(t, rects, map[string]Rect{
		"header":  {0, 0, 80, 3},
		"sidebar": {0, 3, 20, 20},
		"content": {20, 3, 60, 20},
		"footer":  {0, 23, 80, 1},
	})
}
