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
func (t *TabBar) Next() {
	if len(t.Items) == 0 {
		return
	}
	t.active = (t.active + 1) % len(t.Items)
}
func (t *TabBar) Prev() {
	if len(t.Items) == 0 {
		return
	}
	t.active = (t.active - 1 + len(t.Items)) % len(t.Items)
}

// Layout builds the tab strip as a Row of tabs. Each tab is a Row of its
// glyph caps — blank Icons, so the terminal draws them and a GUI leaves them
// out — around its label on the tab's fill with two columns of padding: the
// string render's cells in the terminal, flat filled segments in a window.
// The engine positions and sizes it; the widget computes no coordinates.
// When WithID was set, the strip and each tab carry IDs so a solve records
// their rects. A bordered strip is one Text leaf.
func (t *TabBar) Layout() layout.Node {
	if t.border.enabled() {
		return layout.Text{NodeBase: layout.NodeBase{ID: t.id}, String: t.renderTabsWithBorder()}
	}
	tabs := make([]layout.Node, len(t.Items))
	for i, item := range t.Items {
		fill, text := t.tabColors(i == t.active)
		capStyle := layout.TextStyle{Fg: fill, Bg: t.barBg}
		tabs[i] = layout.Row{NodeBase: layout.NodeBase{ID: t.tabID(item.ID)}, Children: []layout.Node{
			layout.Icon{Name: layout.IconBlank, Text: t.glyphs.Left, Style: capStyle},
			layout.Label{
				NodeBase: layout.NodeBase{PadLeft: 2, Box: layout.Box{Background: fill}},
				Text:     item.Label,
				Style:    layout.TextStyle{Fg: text},
			},
			// The trailing padding is its own node so that a clipped strip
			// loses padding before label text, as the string render does.
			layout.Label{NodeBase: layout.NodeBase{W: layout.Fixed(2), Box: layout.Box{Background: fill}}},
			layout.Icon{Name: layout.IconBlank, Text: t.glyphs.Right, Style: capStyle},
		}}
	}
	return layout.Row{NodeBase: layout.NodeBase{ID: t.id}, Children: tabs}
}

// tabColors is a tab's fill and label color.
func (t *TabBar) tabColors(active bool) (fill, text color.Color) {
	if active {
		return t.fill, activeTabText
	}
	return t.inactiveFill, lipgloss.Color("245")
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
	f, text := t.tabColors(active)
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
	left, right := t.border.leadingWidth(), t.border.trailingWidth()
	top, bottom := t.border.topHeight(), t.border.bottomHeight()
	localX := x - strip.X - left
	localY := y - strip.Y - top
	contentWidth := strip.W - left - right
	contentHeight := strip.H - top - bottom
	if localX < 0 || localX >= contentWidth || localY < 0 || localY >= contentHeight {
		return -1, false
	}
	px := 0
	for i, item := range t.Items {
		w := t.tabWidth(item)
		if localX >= px && localX < px+w {
			return i, true
		}
		px += w
	}
	return -1, false
}

// tabWidth is measured from the rendered tab so arbitrary glyph widths and
// the label's actual Lip Gloss padding stay in sync with HitTest geometry.
func (t *TabBar) tabWidth(item TabItem) int {
	return lipgloss.Width(t.renderTab(item, false))
}

// tabLabelWidth is the default-glyph width retained for callers and tests that
// need to describe the natural width of a default tab.
func tabLabelWidth(label string) int {
	return lipgloss.Width(TabGlyphsPowerline.Left) + lipgloss.Width(label) + 4 + lipgloss.Width(TabGlyphsPowerline.Right)
}

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
	if !b.Left {
		return 0
	}
	return b.style().GetLeftSize()
}

func (b TabBarBorder) trailingWidth() int {
	if !b.Right {
		return 0
	}
	return b.style().GetRightSize()
}

func (b TabBarBorder) topHeight() int {
	if b.Top {
		return b.style().GetTopSize()
	}
	return 0
}

func (b TabBarBorder) bottomHeight() int {
	if b.Bottom {
		return b.style().GetBottomSize()
	}
	return 0
}
