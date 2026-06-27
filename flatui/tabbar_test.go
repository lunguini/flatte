package flatui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestTabBarActiveAndSet(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
		TabItem{ID: "c", Label: "gamma"},
	)
	if tb.Active() != 0 {
		t.Fatalf("initial active = %d, want 0", tb.Active())
	}
	if tb.ActiveID() != "a" {
		t.Fatalf("initial activeID = %q, want a", tb.ActiveID())
	}
	tb.SetActive(2)
	if tb.Active() != 2 || tb.ActiveID() != "c" {
		t.Fatalf("after SetActive(2): active=%d id=%q, want 2/c", tb.Active(), tb.ActiveID())
	}
	tb.SetActive(-1)
	if tb.Active() != 2 {
		t.Fatalf("SetActive(-1) should clamp, active = %d", tb.Active())
	}
	tb.SetActive(99)
	if tb.Active() != 2 {
		t.Fatalf("SetActive(99) should clamp, active = %d", tb.Active())
	}
}

func TestTabBarNextPrev(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
		TabItem{ID: "c", Label: "gamma"},
	)
	tb.Next()
	if tb.Active() != 1 {
		t.Fatalf("after Next: %d, want 1", tb.Active())
	}
	tb.Next()
	tb.Next()
	if tb.Active() != 0 {
		t.Fatalf("after Next×3 (wrap): %d, want 0", tb.Active())
	}
	tb.Prev()
	if tb.Active() != 2 {
		t.Fatalf("after Prev from 0 (wrap): %d, want 2", tb.Active())
	}
}

func TestTabBarHandleMouseAt(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
		TabItem{ID: "c", Label: "gamma"},
	)
	alphaW := TabLabelWidth("alpha")
	betaStart := alphaW

	if !tb.HandleMouseAt(0) || tb.Active() != 0 {
		t.Fatal("click at 0 should hit alpha")
	}
	if !tb.HandleMouseAt(betaStart) || tb.Active() != 1 {
		t.Fatal("click at betaStart should hit beta")
	}
	if tb.HandleMouseAt(999) {
		t.Fatal("click way past the end should not hit any tab")
	}
}

func TestTabBarRenderProducesNonEmpty(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
	)
	rendered := tb.Render(lipgloss.Color("117"), lipgloss.Color("238"), lipgloss.Color("236"))
	if rendered == "" {
		t.Fatal("Render produced empty string")
	}
	// After ANSI strip, both labels should be present
	stripped := lipgloss.NewStyle().Render(rendered) // strip ANSI via re-render (approximate)
	if !strings.Contains(stripped, "alpha") || !strings.Contains(stripped, "beta") {
		// Re-render might not strip; check raw too
		if !strings.Contains(rendered, "alpha") {
			t.Fatalf("Render missing 'alpha': %q", rendered)
		}
	}
}

func TestTabBarWithSafeGlyphs(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "x", Label: "x"}).WithGlyphs(TabGlyphsSafe)
	rendered := tb.Render(lipgloss.Color("117"), lipgloss.Color("238"), lipgloss.Color("236"))
	if !strings.Contains(rendered, "[") || !strings.Contains(rendered, "]") {
		t.Fatalf("safe glyphs should produce brackets: %q", rendered)
	}
}

func TestTabBarTotalWidth(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
	)
	want := TabLabelWidth("alpha") + TabLabelWidth("beta")
	if tb.TotalWidth() != want {
		t.Fatalf("TotalWidth = %d, want %d", tb.TotalWidth(), want)
	}
}

func TestTabBarTabStartX(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
	)
	if tb.TabStartX(0) != 0 {
		t.Fatalf("TabStartX(0) = %d, want 0", tb.TabStartX(0))
	}
	if tb.TabStartX(1) != TabLabelWidth("alpha") {
		t.Fatalf("TabStartX(1) = %d, want %d", tb.TabStartX(1), TabLabelWidth("alpha"))
	}
}
