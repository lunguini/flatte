package main

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// tabBar is a reusable tab-strip component with built-in Powerline rendering
// and mouse-click support. Owns: the active tab index, rendering, hit-testing.
// Does NOT own: what "active" means (the caller maps Active() → their domain).
//
// Same extraction pattern as paneLayout: Containers proved the pattern
// (hand-coded tabZones), the header proved it repeats (no click support at
// all). tabBar closes both gaps and proves the framework extraction
// candidate (flatui.TabBar with the same shape).
type tabBar struct {
	items  []tabItem
	active int
}

type tabItem struct {
	id    string
	label string
}

func newTabBar(items ...tabItem) *tabBar {
	return &tabBar{items: items}
}

func (t *tabBar) Active() int      { return t.active }
func (t *tabBar) ActiveID() string { return t.items[t.active].id }

func (t *tabBar) SetActive(i int) {
	if i >= 0 && i < len(t.items) {
		t.active = i
	}
}

func (t *tabBar) Next() {
	if len(t.items) > 0 {
		t.active = (t.active + 1) % len(t.items)
	}
}

func (t *tabBar) Prev() {
	if len(t.items) > 0 {
		t.active = (t.active - 1 + len(t.items)) % len(t.items)
	}
}

func tabLabelWidth(label string) int {
	return lipgloss.Width(label) + 6
}

// Render produces the tab strip with Powerline-style tabs that touch
// directly — no gap between them. The right slant of one tab meets the
// left slant of the next, creating a continuous edge.
func (t *tabBar) Render() string {
	var b strings.Builder
	for i, item := range t.items {
		rendered, _ := powerlineTab(item.label, i == t.active)
		b.WriteString(rendered)
	}
	return b.String()
}

// RenderWithBg wraps Render with a background color (for header-style
// usage where the bar sits on a colored strip).
func (t *tabBar) RenderWithBg(bg color.Color) string {
	return lipgloss.NewStyle().Background(bg).Render(t.Render())
}

// HandleMouseAt checks whether localX falls inside any tab. If so, sets
// that tab active and returns true. localX is relative to the start of
// the tab strip (the first tab's left slant is at localX=0).
func (t *tabBar) HandleMouseAt(localX int) bool {
	x := 0
	for i, item := range t.items {
		w := tabLabelWidth(item.label)
		if localX >= x && localX < x+w {
			t.SetActive(i)
			return true
		}
		x += w
	}
	return false
}

// TabStartX returns the frame X where tab i begins, given the strip
// starts at stripStartX. Useful for cursor placement or debugging.
func (t *tabBar) TabStartX(i, stripStartX int) int {
	x := stripStartX
	for j := 0; j < i && j < len(t.items); j++ {
		x += tabLabelWidth(t.items[j].label)
	}
	return x
}

// composeHeader places left content at the left edge and right content at
// the right edge of a width-bounded row, on a shared background. The gap
// between them absorbs the remaining space. This is the composition
// primitive for "title left, tabs right" or any two-element header layout.
//
// It deliberately does NOT aggregate title and tabs into one component —
// the caller owns which is which and which side each goes on. The helper
// just handles the alignment math so every caller gets it right without
// rewriting lipgloss.PlaceHorizontal / spacer arithmetic.
func composeHeader(left, right string, width int, bg color.Color) string {
	rightW := lipgloss.Width(right)
	if rightW >= width {
		return lipgloss.NewStyle().Background(bg).Width(width).MaxWidth(width).Render(right)
	}
	maxLeftW := width - rightW - 1
	leftW := lipgloss.Width(left)
	if leftW > maxLeftW {
		if maxLeftW > 1 {
			left = truncateStyled(left, maxLeftW-1) + "…"
		} else {
			left = ""
		}
		leftW = lipgloss.Width(left)
	}
	gap := width - leftW - rightW
	if gap < 1 {
		gap = 1
	}
	spacer := lipgloss.NewStyle().Background(bg).Render(strings.Repeat(" ", gap))
	return lipgloss.NewStyle().Background(bg).Width(width).MaxWidth(width).Render(left + spacer + right)
}

func truncateStyled(s string, width int) string {
	if lipgloss.Width(s) <= width {
		return s
	}
	// Strip ANSI, truncate, re-render as plain (simple — loses styling
	// but keeps the width correct)
	stripped := make([]rune, 0, width)
	inEscape := false
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if r == 'm' || (r >= '@' && r <= '~') {
				inEscape = false
			}
			continue
		}
		stripped = append(stripped, r)
		if len(stripped) >= width {
			break
		}
	}
	return string(stripped)
}
