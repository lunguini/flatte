package flatui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
)

// TabBar is a tab-strip Element with built-in mouse hit-testing.
// Implements layout.Element — use El(tabBar) in layout trees.
//
// Colors are set once via WithColors, not passed per-render. This keeps
// the Element interface clean: Layout(w, h) Node takes only dimensions.
type TabBar struct {
	items        []TabItem
	active       int
	glyphs       TabGlyphs
	fill         color.Color
	inactiveFill color.Color
	barBg        color.Color
}

// TabItem is one tab in a TabBar.
type TabItem struct {
	ID    string
	Label string
}

// TabGlyphs holds the left/right edge characters for tab rendering.
type TabGlyphs struct {
	Left  string
	Right string
}

var TabGlyphsPowerline = TabGlyphs{Left: "\ue0ba", Right: "\ue0bc"}
var TabGlyphsSafe = TabGlyphs{Left: "[", Right: "]"}

// NewTabBar creates a TabBar with the given items and Powerline glyphs.
func NewTabBar(items ...TabItem) *TabBar {
	return &TabBar{items: items, glyphs: TabGlyphsPowerline}
}

// WithGlyphs sets custom edge glyphs.
func (t *TabBar) WithGlyphs(g TabGlyphs) *TabBar {
	t.glyphs = g
	return t
}

// WithColors sets the fill, inactiveFill, and background colors used by
// Layout. Call this once during initialization — colors stay fixed for
// the TabBar's lifetime.
func (t *TabBar) WithColors(fill, inactiveFill, barBg color.Color) *TabBar {
	t.fill, t.inactiveFill, t.barBg = fill, inactiveFill, barBg
	return t
}

func (t *TabBar) Active() int { return t.active }
func (t *TabBar) ActiveID() string {
	if t.active >= 0 && t.active < len(t.items) {
		return t.items[t.active].ID
	}
	return ""
}
func (t *TabBar) SetActive(i int) {
	if i >= 0 && i < len(t.items) {
		t.active = i
	}
}
func (t *TabBar) Next() {
	if len(t.items) > 0 {
		t.active = (t.active + 1) % len(t.items)
	}
}
func (t *TabBar) Prev() {
	if len(t.items) > 0 {
		t.active = (t.active - 1 + len(t.items)) % len(t.items)
	}
}

// Layout implements layout.Element. Returns a Row of tab content nodes.
func (t *TabBar) Layout(_, _ int) layout.Node {
	tabs := make([]layout.Node, len(t.items))
	for i, item := range t.items {
		tabs[i] = layout.Content(t.renderTab(item, i == t.active))
	}
	return layout.Row(tabs...)
}

func (t *TabBar) renderTab(item TabItem, active bool) string {
	var f, text color.Color
	if active {
		f, text = t.fill, colorMatching(t.fill)
	} else {
		f, text = t.inactiveFill, lipgloss.Color("245")
	}
	l := lipgloss.NewStyle().Foreground(f).Background(t.barBg).Render(t.glyphs.Left)
	c := lipgloss.NewStyle().Foreground(text).Background(f).Padding(0, 2).Render(item.Label)
	r := lipgloss.NewStyle().Foreground(f).Background(t.barBg).Render(t.glyphs.Right)
	return l + c + r
}

// HandleMouseAt checks whether localX falls inside any tab. If so, sets
// that tab active and returns true. localX is relative to the tab strip.
func (t *TabBar) HandleMouseAt(localX int) bool {
	x := 0
	for i, item := range t.items {
		w := TabLabelWidth(item.Label)
		if localX >= x && localX < x+w {
			t.SetActive(i)
			return true
		}
		x += w
	}
	return false
}

// TabLabelWidth returns the rendered width of a single tab.
func TabLabelWidth(label string) int {
	return lipgloss.Width(label) + 6
}

// TotalWidth returns the combined rendered width of all tabs.
func (t *TabBar) TotalWidth() int {
	w := 0
	for _, item := range t.items {
		w += TabLabelWidth(item.Label)
	}
	return w
}

// TabStartX returns the X offset where tab i begins within the strip.
func (t *TabBar) TabStartX(i int) int {
	x := 0
	for j := 0; j < i && j < len(t.items); j++ {
		x += TabLabelWidth(t.items[j].Label)
	}
	return x
}

func colorMatching(c color.Color) color.Color {
	return lipgloss.Color("23")
}
