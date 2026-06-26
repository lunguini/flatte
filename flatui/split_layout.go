package flatui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
)

// SplitLayout manages a horizontal row of panes separated by drag-to-resize
// dividers. Each pane provides a render callback (returns fully-styled
// content) and an optional mouse callback. The layout owns geometry,
// divider rendering, drag handling, and mouse routing.
//
// Promoted from cmd/flat-docker/pane_layout.go where the dogfood proved
// the pattern across three screens (Containers hand-coded, Images + Volumes
// via the abstraction).
type SplitLayout struct {
	width  int
	height int
	startY int
	panes  []*PaneEntry
	drag   *splitDrag
	style  SplitLayoutStyle
}

// PaneEntry describes one pane in a SplitLayout.
type PaneEntry struct {
	ID       string
	Width    int // current width (user-adjustable; initialized to 0 = even split)
	MinWidth int
	Render   PaneRenderFunc
	OnMouse  PaneMouseFunc
}

// PaneRenderFunc returns a fully-styled string for the pane's content at the
// given dimensions. The caller owns borders, padding, background — the
// layout only handles placement and dividers.
type PaneRenderFunc func(paneWidth, paneHeight int) string

// PaneMouseFunc receives a mouse event with coordinates converted to
// pane-local (0-based, relative to the pane's top-left corner).
type PaneMouseFunc func(m flatte.MouseEvent, localX, localY int)

// SplitLayoutStyle configures divider appearance.
type SplitLayoutStyle struct {
	DividerBg       color.Color // normal divider background
	DividerActiveBg color.Color // divider background while being dragged
	DividerGripFg   color.Color // the centered │ grip indicator (normal)
	DividerActiveFg color.Color // the centered │ grip indicator (dragging)
}

// DefaultSplitLayoutStyle uses common gray/blue defaults that match
// typical Flatte apps.
func DefaultSplitLayoutStyle() SplitLayoutStyle {
	return SplitLayoutStyle{
		DividerBg:       lipgloss.Color("240"),
		DividerActiveBg: lipgloss.Color("117"),
		DividerGripFg:   lipgloss.Color("236"),
		DividerActiveFg: lipgloss.Color("23"),
	}
}

type splitDrag struct {
	divider     int
	startX      int
	startWidths []int
}

const SplitDividerWidth = 1

// NewSplitLayout creates a layout with the given panes and style.
func NewSplitLayout(style SplitLayoutStyle, panes ...PaneEntry) *SplitLayout {
	l := &SplitLayout{style: style}
	for i := range panes {
		l.panes = append(l.panes, &panes[i])
	}
	return l
}

// Layout sets the layout dimensions. Panes with Width=0 get an even split
// on first call; thereafter the user owns the widths.
func (l *SplitLayout) Layout(width, height, startY int) {
	l.width, l.height, l.startY = width, height, startY
	dividers := (len(l.panes) - 1) * SplitDividerWidth
	available := width - dividers
	for i, p := range l.panes {
		if p.Width == 0 {
			share := available / len(l.panes)
			if i == 0 {
				share += available % len(l.panes) // remainder to first pane
			}
			p.Width = max(share, p.MinWidth)
		}
	}
	l.clampWidths()
}

func (l *SplitLayout) clampWidths() {
	dividers := (len(l.panes) - 1) * SplitDividerWidth
	available := l.width - dividers
	total := 0
	for _, p := range l.panes {
		total += p.Width
	}
	if total > available {
		overflow := total - available
		for _, p := range l.panes {
			shrink := min(overflow, p.Width-p.MinWidth)
			p.Width -= shrink
			overflow -= shrink
			if overflow <= 0 {
				break
			}
		}
	}
	for _, p := range l.panes {
		p.Width = max(p.Width, p.MinWidth)
	}
}

// View renders all panes + dividers as a horizontal join.
func (l *SplitLayout) View() string {
	parts := make([]string, 0, len(l.panes)*2-1)
	for i, p := range l.panes {
		rendered := p.Render(p.Width, l.height)
		parts = append(parts, rendered)
		if i < len(l.panes)-1 {
			parts = append(parts, l.renderDivider(i))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (l *SplitLayout) renderDivider(idx int) string {
	dragging := l.drag != nil && l.drag.divider == idx
	bg := l.style.DividerBg
	gripFg := l.style.DividerGripFg
	if dragging {
		bg = l.style.DividerActiveBg
		gripFg = l.style.DividerActiveFg
	}
	midY := l.height / 2
	rows := make([]string, l.height)
	for i := 0; i < l.height; i++ {
		if i == midY {
			rows[i] = "│"
		} else {
			rows[i] = " "
		}
	}
	content := strings.Join(rows, "\n")
	return lipgloss.NewStyle().
		Width(SplitDividerWidth).
		Height(l.height).
		MaxHeight(l.height).
		Foreground(gripFg).
		Background(bg).
		Render(content)
}

// HandleMouse routes a mouse event. Returns true if the event was consumed.
// Priority: ongoing drag → divider press (start drag) → pane callback.
func (l *SplitLayout) HandleMouse(m flatte.MouseEvent) bool {
	if l.drag != nil {
		if m.Action == flatte.MouseRelease || (m.Button == flatte.MouseNone && m.Action != flatte.MouseMotion) {
			l.drag = nil
			return true
		}
		if m.Action == flatte.MouseMotion {
			l.applyDrag(m.X)
			return true
		}
		return true
	}
	if m.Action == flatte.MousePress {
		if div := l.dividerAt(m.X, m.Y); div >= 0 {
			l.drag = &splitDrag{divider: div, startX: m.X, startWidths: l.snapshotWidths()}
			return true
		}
	}
	idx, localX, localY := l.paneAt(m.X, m.Y)
	if idx >= 0 && l.panes[idx].OnMouse != nil {
		l.panes[idx].OnMouse(m, localX, localY)
		return true
	}
	return false
}

func (l *SplitLayout) dividerAt(x, y int) int {
	if y < l.startY || y >= l.startY+l.height {
		return -1
	}
	paneX := 0
	for i := 0; i < len(l.panes)-1; i++ {
		paneX += l.panes[i].Width
		if x == paneX {
			return i
		}
		paneX += SplitDividerWidth
	}
	return -1
}

func (l *SplitLayout) paneAt(x, y int) (int, int, int) {
	if y < l.startY || y >= l.startY+l.height {
		return -1, 0, 0
	}
	paneX := 0
	for i, p := range l.panes {
		if x >= paneX && x < paneX+p.Width {
			return i, x - paneX, y - l.startY
		}
		paneX += p.Width + SplitDividerWidth
	}
	return -1, 0, 0
}

func (l *SplitLayout) applyDrag(currentX int) {
	if l.drag == nil {
		return
	}
	delta := currentX - l.drag.startX
	leftIdx := l.drag.divider
	rightIdx := l.drag.divider + 1
	minLeft := l.panes[leftIdx].MinWidth
	minRight := l.panes[rightIdx].MinWidth
	pool := l.drag.startWidths[leftIdx] + l.drag.startWidths[rightIdx]
	newLeft := l.drag.startWidths[leftIdx] + delta
	newLeft = max(minLeft, min(newLeft, pool-minRight))
	l.panes[leftIdx].Width = newLeft
	l.panes[rightIdx].Width = pool - newLeft
}

func (l *SplitLayout) snapshotWidths() []int {
	w := make([]int, len(l.panes))
	for i, p := range l.panes {
		w[i] = p.Width
	}
	return w
}

// PaneWidth returns the current width of pane i.
func (l *SplitLayout) PaneWidth(i int) int {
	if i < 0 || i >= len(l.panes) {
		return 0
	}
	return l.panes[i].Width
}

// PaneCount returns the number of panes.
func (l *SplitLayout) PaneCount() int { return len(l.panes) }
