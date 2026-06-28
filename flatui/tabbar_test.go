package flatui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestTabBarActiveAndSet(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "a", Label: "alpha"}, TabItem{ID: "b", Label: "beta"})
	if tb.Active() != 0 {
		t.Fatal("default active should be 0")
	}
	tb.SetActive(1)
	if tb.Active() != 1 || tb.ActiveID() != "b" {
		t.Fatalf("after SetActive(1): active=%d id=%q", tb.Active(), tb.ActiveID())
	}
}

func TestTabBarNextPrev(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "a"}, TabItem{ID: "b"}, TabItem{ID: "c"})
	tb.Next()
	if tb.Active() != 1 {
		t.Fatal("Next should advance to 1")
	}
	tb.Next()
	if tb.Active() != 2 {
		t.Fatal("Next should advance to 2")
	}
	tb.Next()
	if tb.Active() != 0 {
		t.Fatal("Next should wrap to 0")
	}
	tb.Prev()
	if tb.Active() != 2 {
		t.Fatal("Prev should wrap to 2")
	}
}

func TestTabBarHandleMouseAt(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "a", Label: "alpha"}, TabItem{ID: "b", Label: "beta"})
	if !tb.HandleMouseAt(0) {
		t.Fatal("click at 0 should hit first tab")
	}
	if tb.Active() != 0 {
		t.Fatal("first tab should be active")
	}
	if !tb.HandleMouseAt(TabLabelWidth("alpha")) {
		t.Fatal("click at second tab start should hit")
	}
	if tb.Active() != 1 {
		t.Fatal("second tab should be active")
	}
}

func TestTabBarLayoutProducesNonEmpty(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
	).WithColors(lipgloss.Color("117"), lipgloss.Color("238"), lipgloss.Color("236"))
	rendered := layout.Render(tb.Layout(), tb.TotalWidth(), 1)
	if rendered == "" {
		t.Fatal("Render produced empty string")
	}
	if !strings.Contains(rendered, "alpha") || !strings.Contains(rendered, "beta") {
		t.Fatalf("Layout missing labels: %q", rendered)
	}
}

func TestTabBarWithSafeGlyphs(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "x", Label: "x"}).
		WithGlyphs(TabGlyphsSafe).
		WithColors(lipgloss.Color("117"), lipgloss.Color("238"), lipgloss.Color("236"))
	rendered := layout.Render(tb.Layout(), tb.TotalWidth(), 1)
	if !strings.Contains(rendered, "[") || !strings.Contains(rendered, "]") {
		t.Fatalf("safe glyphs should produce brackets: %q", rendered)
	}
}

func TestTabBarTotalWidth(t *testing.T) {
	tb := NewTabBar(TabItem{Label: "alpha"}, TabItem{Label: "beta"})
	want := TabLabelWidth("alpha") + TabLabelWidth("beta")
	if tb.TotalWidth() != want {
		t.Fatalf("TotalWidth = %d, want %d", tb.TotalWidth(), want)
	}
}

func TestTabBarTabStartX(t *testing.T) {
	tb := NewTabBar(TabItem{Label: "alpha"}, TabItem{Label: "beta"})
	if tb.TabStartX(0) != 0 {
		t.Fatalf("TabStartX(0) = %d, want 0", tb.TabStartX(0))
	}
	if tb.TabStartX(1) != TabLabelWidth("alpha") {
		t.Fatalf("TabStartX(1) = %d, want %d", tb.TabStartX(1), TabLabelWidth("alpha"))
	}
}
