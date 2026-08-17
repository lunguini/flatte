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

func TestTabBarNextPrevEmptyIsInert(t *testing.T) {
	tb := NewTabBar()
	tb.Next()
	tb.Prev()
	if tb.Active() != 0 || tb.ActiveID() != "" {
		t.Fatalf("empty tabbar after Next and Prev = active %d id %q, want 0 and empty", tb.Active(), tb.ActiveID())
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

func TestTabBarHitTestStripFallbackUsesGlyphWidths(t *testing.T) {
	for _, tc := range []struct {
		name  string
		glyph TabGlyphs
	}{
		{name: "wide", glyph: TabGlyphs{Left: "界", Right: "界界"}},
		{name: "empty", glyph: TabGlyphs{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := NewTabBar(
				TabItem{ID: "a", Label: "alpha"},
				TabItem{ID: "b", Label: "beta"},
			).WithGlyphs(tc.glyph).WithID("strip")
			strip := layout.Rect{X: 10, Y: 2, W: layoutWidth(tb), H: 1}
			rects := map[string]layout.Rect{"strip": strip}

			firstWidth := lipgloss.Width(tc.glyph.Left) + lipgloss.Width("alpha") + 4 + lipgloss.Width(tc.glyph.Right)
			if got := strip.W; got < firstWidth {
				t.Fatalf("strip width = %d, want at least first tab width %d", got, firstWidth)
			}
			if i, ok := tb.HitTest(rects, strip.X+firstWidth-1, strip.Y); !ok || i != 0 {
				t.Fatalf("last cell of first tab = %d,%v want 0,true", i, ok)
			}
			if i, ok := tb.HitTest(rects, strip.X+firstWidth, strip.Y); !ok || i != 1 {
				t.Fatalf("first cell of second tab = %d,%v want 1,true", i, ok)
			}
		})
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

func TestTabBarHitTestFallbackRespectsBorderGeometry(t *testing.T) {
	border := lipgloss.Border{
		Top: "══", Bottom: "══", Left: "██", Right: "██",
		TopLeft: "██", TopRight: "██", BottomLeft: "██", BottomRight: "██",
	}
	tb := NewTabBar(TabItem{ID: "a", Label: "alpha"}).
		WithGlyphs(TabGlyphs{}).
		WithID("strip").
		WithBorder(TabBarBorder{Style: border, Top: true, Right: true, Bottom: true, Left: true})
	strip := layout.Rect{X: 10, Y: 2, W: layoutWidth(tb), H: 3}
	rects := map[string]layout.Rect{"strip": strip}
	left, right := border.GetLeftSize(), border.GetRightSize()

	if _, ok := tb.HitTest(rects, strip.X+left-1, strip.Y+1); ok {
		t.Fatal("left border cells should not hit the first tab")
	}
	if i, ok := tb.HitTest(rects, strip.X+left, strip.Y+1); !ok || i != 0 {
		t.Fatalf("first content cell after wide left border = %d,%v want 0,true", i, ok)
	}
	if _, ok := tb.HitTest(rects, strip.X+strip.W-right, strip.Y+1); ok {
		t.Fatal("right border cells should not hit the last tab")
	}
	if _, ok := tb.HitTest(rects, strip.X+left, strip.Y); ok {
		t.Fatal("top border cells should not hit a tab")
	}
	if _, ok := tb.HitTest(rects, strip.X+left, strip.Y+strip.H-1); ok {
		t.Fatal("bottom border cells should not hit a tab")
	}
}

func TestTabBarHitTestFallbackUsesThickTopAndBottomBorders(t *testing.T) {
	border := lipgloss.Border{
		Top: "界", Bottom: "界", Left: "│", Right: "│",
		TopLeft: "界", TopRight: "界", BottomLeft: "界", BottomRight: "界",
	}
	tb := NewTabBar(TabItem{ID: "a", Label: "alpha"}).
		WithGlyphs(TabGlyphs{}).
		WithID("strip").
		WithBorder(TabBarBorder{Style: border, Top: true, Right: true, Bottom: true, Left: true})
	top, bottom := border.GetTopSize(), border.GetBottomSize()
	strip := layout.Rect{X: 10, Y: 2, W: layoutWidth(tb), H: top + 1 + bottom}
	if top < 2 || bottom < 2 {
		t.Fatalf("test border sizes top=%d bottom=%d; want thick border", top, bottom)
	}
	rects := map[string]layout.Rect{"strip": strip}
	for row := 0; row < top; row++ {
		if _, ok := tb.HitTest(rects, strip.X+1, strip.Y+row); ok {
			t.Fatalf("top border row %d was treated as a tab hit", row)
		}
	}
	left := border.GetLeftSize()
	if i, ok := tb.HitTest(rects, strip.X+left, strip.Y+top); !ok || i != 0 {
		t.Fatalf("content row hit=%d,%v want 0,true", i, ok)
	}
	for row := 0; row < bottom; row++ {
		y := strip.Y + top + 1 + row
		if _, ok := tb.HitTest(rects, strip.X+left, y); ok {
			t.Fatalf("bottom border row %d was treated as a tab hit", row)
		}
	}
}
