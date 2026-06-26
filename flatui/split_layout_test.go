package flatui

import (
	"testing"

	"github.com/lunguini/flatte"
)

func TestSplitLayoutPaneWidths(t *testing.T) {
	l := NewSplitLayout(DefaultSplitLayoutStyle(),
		PaneEntry{ID: "a", MinWidth: 5, Render: func(w, h int) string { return "a" }},
		PaneEntry{ID: "b", MinWidth: 5, Render: func(w, h int) string { return "b" }},
	)
	l.Layout(50, 20, 0)
	// 50 - 1 divider = 49 available, split evenly = 24 + 25
	if l.PaneWidth(0) < 5 || l.PaneWidth(1) < 5 {
		t.Fatalf("pane widths too small: %d, %d", l.PaneWidth(0), l.PaneWidth(1))
	}
	if l.PaneWidth(0)+SplitDividerWidth+l.PaneWidth(1) != 50 {
		t.Fatalf("widths + divider != 50: %d + %d + %d",
			l.PaneWidth(0), SplitDividerWidth, l.PaneWidth(1))
	}
}

func TestSplitLayoutViewProducesContent(t *testing.T) {
	l := NewSplitLayout(DefaultSplitLayoutStyle(),
		PaneEntry{ID: "a", Render: func(w, h int) string { return "AAA" }},
		PaneEntry{ID: "b", Render: func(w, h int) string { return "BBB" }},
	)
	l.Layout(50, 10, 0)
	view := l.View()
	if view == "" {
		t.Fatal("View produced empty string")
	}
}

func TestSplitLayoutHandleMouseDividerDrag(t *testing.T) {
	l := NewSplitLayout(DefaultSplitLayoutStyle(),
		PaneEntry{ID: "a", MinWidth: 5, Render: func(w, h int) string { return "a" }},
		PaneEntry{ID: "b", MinWidth: 5, Render: func(w, h int) string { return "b" }},
	)
	l.Layout(50, 20, 0)
	divX := l.PaneWidth(0)
	startLeft := l.PaneWidth(0)
	startRight := l.PaneWidth(1)

	// Press on divider
	handled := l.HandleMouse(flatte.MouseEvent{
		Action: flatte.MousePress, Button: flatte.MouseLeft,
		X: divX, Y: 5,
	})
	if !handled {
		t.Fatal("divider press not handled")
	}

	// Drag right by 5
	l.HandleMouse(flatte.MouseEvent{
		Action: flatte.MouseMotion, Button: flatte.MouseLeft,
		X: divX + 5, Y: 5,
	})
	if l.PaneWidth(0) != startLeft+5 {
		t.Fatalf("after drag: left pane %d, want %d", l.PaneWidth(0), startLeft+5)
	}
	if l.PaneWidth(1) != startRight-5 {
		t.Fatalf("after drag: right pane %d, want %d", l.PaneWidth(1), startRight-5)
	}

	// Release
	l.HandleMouse(flatte.MouseEvent{
		Action: flatte.MouseRelease, Button: flatte.MouseLeft,
		X: divX + 5, Y: 5,
	})
}

func TestSplitLayoutHandleMouseRoutesToPane(t *testing.T) {
	var clicked bool
	l := NewSplitLayout(DefaultSplitLayoutStyle(),
		PaneEntry{
			ID: "a", Render: func(w, h int) string { return "a" },
			OnMouse: func(m flatte.MouseEvent, lx, ly int) { clicked = true },
		},
		PaneEntry{ID: "b", Render: func(w, h int) string { return "b" }},
	)
	l.Layout(50, 20, 0)

	// Click inside the first pane (not on the divider)
	l.HandleMouse(flatte.MouseEvent{
		Action: flatte.MousePress, Button: flatte.MouseLeft,
		X: 2, Y: 5,
	})
	if !clicked {
		t.Fatal("mouse press was not routed to pane OnMouse callback")
	}
}

func TestSplitLayoutMinWidthClamping(t *testing.T) {
	l := NewSplitLayout(DefaultSplitLayoutStyle(),
		PaneEntry{ID: "a", MinWidth: 20, Render: func(w, h int) string { return "a" }},
		PaneEntry{ID: "b", MinWidth: 20, Render: func(w, h int) string { return "b" }},
	)
	// Try to layout in a very narrow space
	l.Layout(15, 10, 0)
	if l.PaneWidth(0) < 20 {
		t.Fatalf("left pane width %d below min 20", l.PaneWidth(0))
	}
}

func TestSplitLayoutPaneWidthPersistsAcrossResize(t *testing.T) {
	l := NewSplitLayout(DefaultSplitLayoutStyle(),
		PaneEntry{ID: "a", MinWidth: 5, Render: func(w, h int) string { return "a" }},
		PaneEntry{ID: "b", MinWidth: 5, Render: func(w, h int) string { return "b" }},
	)
	l.Layout(50, 20, 0)

	// Drag divider
	divX := l.PaneWidth(0)
	l.HandleMouse(flatte.MouseEvent{Action: flatte.MousePress, Button: flatte.MouseLeft, X: divX, Y: 5})
	l.HandleMouse(flatte.MouseEvent{Action: flatte.MouseMotion, Button: flatte.MouseLeft, X: divX + 5, Y: 5})
	l.HandleMouse(flatte.MouseEvent{Action: flatte.MouseRelease, Button: flatte.MouseLeft, X: divX + 5, Y: 5})
	adjustedWidth := l.PaneWidth(0)

	// Resize taller
	l.Layout(50, 30, 0)
	if l.PaneWidth(0) != adjustedWidth {
		t.Fatalf("after resize: pane width %d, expected preserved %d", l.PaneWidth(0), adjustedWidth)
	}
}
