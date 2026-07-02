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
	id           string
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

// WithID gives the tab strip a layout ID. When set, Layout tags the strip node
// with id and each tab leaf with id+":"+item.ID, so a full-frame solve records
// per-tab rects that HitTest can look up. Leave unset for decorative strips.
func (t *TabBar) WithID(id string) *TabBar { t.id = id; return t }

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
// positions and sizes it; the widget computes no coordinates. When WithID was
// set, the strip and each tab carry IDs so a solve records their rects.
func (t *TabBar) Layout() layout.Node {
	if t.border.enabled() {
		return layout.Text{NodeBase: layout.NodeBase{ID: t.id}, String: t.renderTabsWithBorder()}
	}
	tabs := make([]layout.Node, len(t.Items))
	for i, item := range t.Items {
		tabs[i] = layout.Text{
			NodeBase: layout.NodeBase{ID: t.tabID(item.ID)},
			String:   t.renderTab(item, i == t.active),
		}
	}
	return layout.Row{NodeBase: layout.NodeBase{ID: t.id}, Children: tabs}
}

// tabID is the per-tab layout ID, or "" when the strip has no ID (so tabs stay
// un-ID'd and record no rects).
func (t *TabBar) tabID(itemID string) string {
	if t.id == "" {
		return ""
	}
	return t.id + ":" + itemID
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
		f, text = t.fill, activeTabText
	} else {
		f, text = t.inactiveFill, lipgloss.Color("245")
	}
	l := lipgloss.NewStyle().Foreground(f).Background(t.barBg).Render(t.glyphs.Left)
	c := lipgloss.NewStyle().Foreground(text).Background(f).Padding(0, 2).Render(item.Label)
	r := lipgloss.NewStyle().Foreground(f).Background(t.barBg).Render(t.glyphs.Right)
	return l + c + r
}

// HitTest maps an absolute frame point to a tab index using solved geometry.
// In the normal (unbordered) mode it looks up each tab's own rect (recorded by
// a full-frame solve when WithID was set). When those per-tab rects are absent
// — the bordered mode renders one pre-composed leaf, and some callers only
// register the strip rect — it falls back to the strip rect plus the widget's
// private per-label width math. It does not mutate the bar; callers apply the
// returned index.
func (t *TabBar) HitTest(rects map[string]layout.Rect, x, y int) (int, bool) {
	if t.id != "" && !t.border.enabled() {
		for i, item := range t.Items {
			if r, ok := rects[t.tabID(item.ID)]; ok && r.Contains(x, y) {
				return i, true
			}
		}
	}
	strip, ok := rects[t.id]
	if !ok || !strip.Contains(x, y) {
		return -1, false
	}
	localX := x - strip.X - t.border.leadingWidth()
	px := 0
	for i, item := range t.Items {
		w := tabLabelWidth(item.Label)
		if localX >= px && localX < px+w {
			return i, true
		}
		px += w
	}
	return -1, false
}

// tabLabelWidth is the rendered width of one tab: the label plus the two-cell
// side padding and the two edge glyphs.
func tabLabelWidth(label string) int { return lipgloss.Width(label) + 6 }

// activeTabText is the label color on an active tab's fill. A computed
// contrast color is the eventual fix if themes need it; today every palette
// pairs the accent fill with this dark teal.
var activeTabText color.Color = lipgloss.Color("23")

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
