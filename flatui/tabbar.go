package flatui

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
)

// TabBar is a tab-strip component. Set colors once via WithColors, then
// compose it into a layout tree via Layout().
type TabBar struct {
	Items        []TabItem
	active       int
	glyphs       TabGlyphs
	fill         color.Color
	inactiveFill color.Color
	barBg        color.Color
}

type TabItem struct {
	ID    string
	Label string
}

type TabGlyphs struct{ Left, Right string }

var TabGlyphsPowerline = TabGlyphs{Left: "\ue0ba", Right: "\ue0bc"}
var TabGlyphsSafe = TabGlyphs{Left: "[", Right: "]"}

func NewTabBar(items ...TabItem) *TabBar {
	return &TabBar{Items: items, glyphs: TabGlyphsPowerline}
}

func (t *TabBar) WithGlyphs(g TabGlyphs) *TabBar { t.glyphs = g; return t }
func (t *TabBar) WithColors(f, i, bg color.Color) *TabBar {
	t.fill, t.inactiveFill, t.barBg = f, i, bg
	return t
}

func (t *TabBar) Active() int { return t.active }
func (t *TabBar) ActiveID() string {
	if t.active >= 0 && t.active < len(t.Items) {
		return t.Items[t.active].ID
	}
	return ""
}
func (t *TabBar) SetActive(i int) {
	if i >= 0 && i < len(t.Items) {
		t.active = i
	}
}
func (t *TabBar) Next() { t.active = (t.active + 1) % len(t.Items) }
func (t *TabBar) Prev() { t.active = (t.active - 1 + len(t.Items)) % len(t.Items) }

// Layout builds the tab strip as a Row of styled Text leaves. The engine
// positions and sizes it; the widget computes no coordinates.
func (t *TabBar) Layout() layout.Node {
	tabs := make([]layout.Node, len(t.Items))
	for i, item := range t.Items {
		tabs[i] = layout.Text{String: t.renderTab(item, i == t.active)}
	}
	return layout.Row{Children: tabs}
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

func (t *TabBar) HandleMouseAt(localX int) bool {
	x := 0
	for i, item := range t.Items {
		w := TabLabelWidth(item.Label)
		if localX >= x && localX < x+w {
			t.SetActive(i)
			return true
		}
		x += w
	}
	return false
}

func TabLabelWidth(label string) int { return lipgloss.Width(label) + 6 }

func (t *TabBar) TotalWidth() int {
	w := 0
	for _, item := range t.Items {
		w += TabLabelWidth(item.Label)
	}
	return w
}

func (t *TabBar) TabStartX(i int) int {
	x := 0
	for j := 0; j < i && j < len(t.Items); j++ {
		x += TabLabelWidth(t.Items[j].Label)
	}
	return x
}

func colorMatching(c color.Color) color.Color { return lipgloss.Color("23") }
