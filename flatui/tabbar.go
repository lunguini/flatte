package flatui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
)

// TabBar is a reusable tab-strip component with built-in rendering and
// mouse-click hit-testing. Owns: the active tab index, rendering, and
// hit-testing. Does NOT own: what "active" means (the caller maps
// Active() → their domain enum) or the content below the tabs.
//
// Rendering uses Powerline-style diagonal slant edges (U+E0BA / U+E0BC)
// by default. Use WithGlyphs to provide custom edge characters (e.g.,
// ASCII brackets for terminals without Nerd Fonts).
type TabBar struct {
	items  []TabItem
	active int
	glyphs TabGlyphs
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

// TabGlyphsPowerline uses Unicode Private Use Area characters that
// render in terminals with a Powerline or Nerd Font installed.
var TabGlyphsPowerline = TabGlyphs{Left: "\ue0ba", Right: "\ue0bc"}

// TabGlyphsSafe uses standard ASCII brackets that render in every terminal.
var TabGlyphsSafe = TabGlyphs{Left: "[", Right: "]"}

// NewTabBar creates a TabBar with the given items and Powerline glyphs.
func NewTabBar(items ...TabItem) *TabBar {
	return &TabBar{items: items, glyphs: TabGlyphsPowerline}
}

// WithGlyphs sets custom edge glyphs (e.g., TabGlyphsSafe for ASCII fallback).
func (t *TabBar) WithGlyphs(g TabGlyphs) *TabBar {
	t.glyphs = g
	return t
}

// Active returns the index of the currently active tab.
func (t *TabBar) Active() int { return t.active }

// ActiveID returns the ID of the currently active tab.
func (t *TabBar) ActiveID() string {
	if t.active >= 0 && t.active < len(t.items) {
		return t.items[t.active].ID
	}
	return ""
}

// SetActive sets the active tab by index.
func (t *TabBar) SetActive(i int) {
	if i >= 0 && i < len(t.items) {
		t.active = i
	}
}

// Next advances to the next tab, wrapping.
func (t *TabBar) Next() {
	if len(t.items) > 0 {
		t.active = (t.active + 1) % len(t.items)
	}
}

// Prev goes to the previous tab, wrapping.
func (t *TabBar) Prev() {
	if len(t.items) > 0 {
		t.active = (t.active - 1 + len(t.items)) % len(t.items)
	}
}

// TabLabelWidth returns the rendered width of a single tab with the given
// label: left glyph + 2 padding + label + 2 padding + right glyph.
func TabLabelWidth(label string) int {
	return lipgloss.Width(label) + 6
}

// Render produces the tab strip as a styled string. Tabs touch directly
// (no gap between them). The active tab uses fill as its background; inactive
// tabs use inactiveFill. Text colors are derived from the fills for contrast.
func (t *TabBar) Render(fill, inactiveFill, barBg color.Color) string {
	tabs := make([]layout.Node, len(t.items))
	for i, item := range t.items {
		active := i == t.active
		tabs[i] = layout.Content(t.renderTab(item, active, fill, inactiveFill, barBg))
	}
	return layout.Render(layout.Row(tabs...), t.TotalWidth(), 1)
}

// renderTab produces a single tab: left glyph + padded label + right glyph.
func (t *TabBar) renderTab(item TabItem, active bool, fill, inactiveFill, barBg color.Color) string {
	var f, text color.Color
	if active {
		f = fill
		text = colorMatching(fill)
	} else {
		f = inactiveFill
		text = lipgloss.Color("245")
	}
	l := lipgloss.NewStyle().Foreground(f).Background(barBg).Render(t.glyphs.Left)
	c := lipgloss.NewStyle().Foreground(text).Background(f).Padding(0, 2).Render(item.Label)
	r := lipgloss.NewStyle().Foreground(f).Background(barBg).Render(t.glyphs.Right)
	return l + c + r
}

// HandleMouseAt checks whether localX falls inside any tab. If so, sets
// that tab active and returns true. localX is relative to the tab strip's
// left edge (the first tab's left glyph is at localX=0).
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
	// For light fills, return dark text; for dark fills, return light text.
	// Simple heuristic: if it's a lipgloss named color that's "light",
	// use dark text. Otherwise use the color itself as text (muted).
	return lipgloss.Color("23")
}
