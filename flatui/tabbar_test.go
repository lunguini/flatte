package flatui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/layout"
)

// layoutWidth is the tab strip's natural width, straight from its layout node —
// the same measurement the engine distributes with.
func layoutWidth(tb *TabBar) int {
	w, _ := tb.Layout().Size()
	return w.Value
}

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

func TestTabBarHitTestPerTabRects(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
	).WithID("tabs")

	_, rects := layout.SolveAndCompose(tb.Layout(), layoutWidth(tb), 1)

	ra, ok := rects["tabs:a"]
	if !ok {
		t.Fatal("per-tab rect tabs:a not recorded")
	}
	rb, ok := rects["tabs:b"]
	if !ok {
		t.Fatal("per-tab rect tabs:b not recorded")
	}

	if i, ok := tb.HitTest(rects, ra.X+1, ra.Y); !ok || i != 0 {
		t.Fatalf("hit in tab a rect = %d,%v want 0,true", i, ok)
	}
	if i, ok := tb.HitTest(rects, rb.X+1, rb.Y); !ok || i != 1 {
		t.Fatalf("hit in tab b rect = %d,%v want 1,true", i, ok)
	}
}

func TestTabBarHitTestOutsideStrip(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "a", Label: "alpha"}).WithID("tabs")
	_, rects := layout.SolveAndCompose(tb.Layout(), layoutWidth(tb), 1)
	if _, ok := tb.HitTest(rects, 999, 999); ok {
		t.Fatal("point outside every tab rect should miss")
	}
}

func TestTabBarHitTestStripFallback(t *testing.T) {
	// When only the strip rect is registered (no per-tab rects), HitTest falls
	// back to the strip rect plus internal label-width math.
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
	).WithID("strip")
	strip := layout.Rect{X: 10, Y: 0, W: layoutWidth(tb), H: 1}
	rects := map[string]layout.Rect{"strip": strip}

	// A point inside the first tab's width band.
	if i, ok := tb.HitTest(rects, 10+1, 0); !ok || i != 0 {
		t.Fatalf("fallback hit tab a = %d,%v want 0,true", i, ok)
	}
	// A point past alpha's band lands on beta.
	if i, ok := tb.HitTest(rects, 10+tabLabelWidth("alpha")+1, 0); !ok || i != 1 {
		t.Fatalf("fallback hit tab b = %d,%v want 1,true", i, ok)
	}
}

func TestTabBarLayoutProducesNonEmpty(t *testing.T) {
	tb := NewTabBar(
		TabItem{ID: "a", Label: "alpha"},
		TabItem{ID: "b", Label: "beta"},
	).WithColors(lipgloss.Color("117"), lipgloss.Color("238"), lipgloss.Color("236"))
	rendered, _ := layout.SolveAndCompose(tb.Layout(), layoutWidth(tb), 1)
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
	rendered, _ := layout.SolveAndCompose(tb.Layout(), layoutWidth(tb), 1)
	if !strings.Contains(rendered, "[") || !strings.Contains(rendered, "]") {
		t.Fatalf("safe glyphs should produce brackets: %q", rendered)
	}
}

func TestTabBarWithBottomBorder(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "x", Label: "x"}).
		WithGlyphs(TabGlyphsSafe).
		WithColors(lipgloss.Color("117"), lipgloss.Color("238"), lipgloss.Color("236")).
		WithBorder(TabBarBorder{
			Color:  lipgloss.Color("5"),
			Bottom: true,
		})

	rendered, _ := layout.SolveAndCompose(tb.Layout(), layoutWidth(tb), 2)
	clean := ansi.Strip(rendered)
	lines := strings.Split(clean, "\n")
	if len(lines) != 2 {
		t.Fatalf("bordered tabbar lines = %d, want 2:\n%q", len(lines), clean)
	}
	if !strings.Contains(lines[0], "x") {
		t.Fatalf("tab label missing from first line: %q", lines[0])
	}
	if !strings.Contains(lines[1], "▀") {
		t.Fatalf("default bottom border should use InnerHalfBlockBorder: %q", lines[1])
	}
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("border color should add ANSI styling: %q", rendered)
	}
}

func TestTabBarBorderSpansAllocatedWidth(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "x", Label: "x"}).
		WithGlyphs(TabGlyphsSafe).
		WithBorder(TabBarBorder{Bottom: true})

	contentW := layoutWidth(tb)
	rendered, _ := layout.SolveAndCompose(tb.Layout(), contentW+10, 2)
	lines := strings.Split(ansi.Strip(rendered), "\n")
	// The border row's trailing padding beyond the tab bar's own content
	// width is unstyled, so SolveAndCompose's compositor correctly trims it
	// from the composed frame; the border glyphs themselves are unaffected.
	if got, want := lipgloss.Width(lines[1]), contentW; got != want {
		t.Fatalf("bottom border width = %d, want content width %d; line=%q", got, want, lines[1])
	}
}

func TestTabBarWithCustomBorderStyle(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "x", Label: "x"}).
		WithGlyphs(TabGlyphsSafe).
		WithBorder(TabBarBorder{
			Style:  lipgloss.NormalBorder(),
			Bottom: true,
		})

	rendered, _ := layout.SolveAndCompose(tb.Layout(), layoutWidth(tb), 2)
	clean := ansi.Strip(rendered)
	if !strings.Contains(clean, "─") {
		t.Fatalf("custom border style missing normal horizontal rule: %q", clean)
	}
	if strings.Contains(clean, "▀") {
		t.Fatalf("custom border style should replace default half-block rule: %q", clean)
	}
}

func TestTabBarSideBordersAffectWidthAndHitTesting(t *testing.T) {
	tb := NewTabBar(TabItem{ID: "a", Label: "alpha"}).
		WithGlyphs(TabGlyphsSafe).
		WithID("strip").
		WithBorder(TabBarBorder{
			Left:  true,
			Right: true,
		})

	if got, want := layoutWidth(tb), tabLabelWidth("alpha")+2; got != want {
		t.Fatalf("bordered strip width = %d, want %d", got, want)
	}

	// Bordered mode renders one leaf, so HitTest uses the strip rect fallback.
	strip := layout.Rect{X: 0, Y: 0, W: layoutWidth(tb), H: 1}
	rects := map[string]layout.Rect{"strip": strip}
	if _, ok := tb.HitTest(rects, 0, 0); ok {
		t.Fatal("left border cell should not hit the first tab")
	}
	if i, ok := tb.HitTest(rects, 1, 0); !ok || i != 0 {
		t.Fatalf("first content cell after left border = %d,%v want 0,true", i, ok)
	}
}
