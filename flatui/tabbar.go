package flatui

import (
	"image/color"
	"strings"

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
	border       TabBarBorder
}

type TabItem struct {
	ID    string
	Label string
}

type TabGlyphs struct{ Left, Right string }

var TabGlyphsPowerline = TabGlyphs{Left: "\ue0ba", Right: "\ue0bc"}
var TabGlyphsSafe = TabGlyphs{Left: "[", Right: "]"}

// TabBarBorder customizes the border around a TabBar. Set any side to true to
// enable that side; Style defaults to lipgloss.InnerHalfBlockBorder().
type TabBarBorder struct {
	Style  lipgloss.Border
	Color  color.Color
	Top    bool
	Right  bool
	Bottom bool
	Left   bool
}

func NewTabBar(items ...TabItem) *TabBar {
	return &TabBar{Items: items, glyphs: TabGlyphsPowerline}
}

func (t *TabBar) WithGlyphs(g TabGlyphs) *TabBar { t.glyphs = g; return t }
func (t *TabBar) WithBorder(b TabBarBorder) *TabBar {
	t.border = b
	return t
}
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
	if t.border.enabled() {
		return layout.Text{String: t.renderTabsWithBorder()}
	}
	tabs := make([]layout.Node, len(t.Items))
	for i, item := range t.Items {
		tabs[i] = layout.Text{String: t.renderTab(item, i == t.active)}
	}
	return layout.Row{Children: tabs}
}

func (t *TabBar) renderTabs() string {
	var b strings.Builder
	for i, item := range t.Items {
		b.WriteString(t.renderTab(item, i == t.active))
	}
	return b.String()
}

func (t *TabBar) renderTabsWithBorder() string {
	style := lipgloss.NewStyle().
		BorderStyle(t.border.style()).
		BorderTop(t.border.Top).
		BorderRight(t.border.Right).
		BorderBottom(t.border.Bottom).
		BorderLeft(t.border.Left)
	if t.border.Color != nil {
		style = style.BorderForeground(t.border.Color)
	}
	return style.Render(t.renderTabs())
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
	x := t.border.leadingWidth()
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
	w := t.border.horizontalWidth()
	for _, item := range t.Items {
		w += TabLabelWidth(item.Label)
	}
	return w
}

func (t *TabBar) TabStartX(i int) int {
	x := t.border.leadingWidth()
	for j := 0; j < i && j < len(t.Items); j++ {
		x += TabLabelWidth(t.Items[j].Label)
	}
	return x
}

func colorMatching(c color.Color) color.Color { return lipgloss.Color("23") }

func (b TabBarBorder) enabled() bool {
	return b.Top || b.Right || b.Bottom || b.Left
}

func (b TabBarBorder) style() lipgloss.Border {
	if b.Style == (lipgloss.Border{}) {
		return lipgloss.InnerHalfBlockBorder()
	}
	return b.Style
}

func (b TabBarBorder) leadingWidth() int {
	if b.Left {
		return 1
	}
	return 0
}

func (b TabBarBorder) horizontalWidth() int {
	w := 0
	if b.Left {
		w++
	}
	if b.Right {
		w++
	}
	return w
}
