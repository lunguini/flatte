package main

import (
	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
)

// paneLayout manages a horizontal row of bordered panes separated by
// drag-to-resize dividers. Each pane provides a render callback (returns
// inner content; the layout wraps with border + dimensions) and an
// optional mouse callback (receives events with content-relative coords).
//
// The layout owns:
//   - pane widths (user-adjustable, persisted across resize)
//   - divider rendering and hit-testing
//   - mouse routing (press→divider-start-drag or pane-callback;
//     motion→drag-resize; wheel→pane-callback)
//
// Each pane owns:
//   - its content (render callback)
//   - its mouse behavior within its content area (onMouse callback)
//
// This is the abstraction the dogfood found: Containers proved the pattern
// (hand-coded), Images/Volumes prove it generalizes (via paneLayout), and
// the framework extraction candidate is flatui.SplitLayout with the same
// shape but public API.

type paneRenderFunc func(contentWidth, contentHeight int) string
type paneMouseFunc func(m flatte.MouseEvent, localX, localY int)

type paneEntry struct {
	id       string
	width    int
	minWidth int
	render   paneRenderFunc
	onMouse  paneMouseFunc
}

type paneLayout struct {
	width  int
	height int
	startY int // frame Y of the layout's top edge
	panes  []*paneEntry
	drag   *paneDragState
}

type paneDragState struct {
	divider     int
	startX      int
	startWidths []int
}

func newPaneLayout(entries ...paneEntry) *paneLayout {
	l := &paneLayout{}
	for i := range entries {
		l.panes = append(l.panes, &entries[i])
	}
	return l
}

func (l *paneLayout) Layout(width, height, startY int) {
	l.width, l.height, l.startY = width, height, startY
	dividers := (len(l.panes) - 1) * dividerWidth
	available := width - dividers

	for _, p := range l.panes {
		if p.width == 0 {
			p.width = max(available/len(l.panes), p.minWidth)
		}
	}
	l.clampWidths()
}

func (l *paneLayout) clampWidths() {
	dividers := (len(l.panes) - 1) * dividerWidth
	available := l.width - dividers
	total := 0
	for _, p := range l.panes {
		total += p.width
	}
	if total > available {
		overflow := total - available
		for _, p := range l.panes {
			shrink := min(overflow, p.width-p.minWidth)
			p.width -= shrink
			overflow -= shrink
			if overflow <= 0 {
				break
			}
		}
	}
	for _, p := range l.panes {
		p.width = max(p.width, p.minWidth)
	}
}

func (l *paneLayout) View() string {
	parts := make([]string, 0, len(l.panes)*2-1)
	for i, p := range l.panes {
		contentWidth := max(p.width-paneBorderCols, 1)
		contentHeight := max(l.height-paneBorderRows, 0)
		inner := p.render(contentWidth, contentHeight)
		outer := paneStyle(p.width, l.height, false).Render(inner)
		parts = append(parts, outer)
		if i < len(l.panes)-1 {
			parts = append(parts, l.renderDivider(i))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

func (l *paneLayout) renderDivider(idx int) string {
	return renderDragDivider(l.height, l.drag != nil && l.drag.divider == idx)
}

func (l *paneLayout) HandleMouse(m flatte.MouseEvent) bool {
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
			l.drag = &paneDragState{divider: div, startX: m.X, startWidths: l.snapshotWidths()}
			return true
		}
	}

	idx, localX, localY := l.paneAt(m.X, m.Y)
	if idx >= 0 && l.panes[idx].onMouse != nil {
		l.panes[idx].onMouse(m, localX, localY)
		return true
	}
	return false
}

func (l *paneLayout) dividerAt(x, y int) int {
	if y < l.startY || y >= l.startY+l.height {
		return -1
	}
	paneX := 0
	for i := 0; i < len(l.panes)-1; i++ {
		paneX += l.panes[i].width
		if x == paneX {
			return i
		}
		paneX += dividerWidth
	}
	return -1
}

func (l *paneLayout) paneAt(x, y int) (int, int, int) {
	if y < l.startY || y >= l.startY+l.height {
		return -1, 0, 0
	}
	paneX := 0
	for i, p := range l.panes {
		if x >= paneX && x < paneX+p.width {
			localX := x - paneX - 1
			localY := y - l.startY - 1
			if localX < 0 {
				localX = 0
			}
			if localY < 0 {
				localY = 0
			}
			return i, localX, localY
		}
		paneX += p.width + dividerWidth
	}
	return -1, 0, 0
}

func (l *paneLayout) applyDrag(currentX int) {
	if l.drag == nil {
		return
	}
	delta := currentX - l.drag.startX
	leftIdx := l.drag.divider
	rightIdx := l.drag.divider + 1

	minLeft := l.panes[leftIdx].minWidth
	minRight := l.panes[rightIdx].minWidth
	pool := l.drag.startWidths[leftIdx] + l.drag.startWidths[rightIdx]

	newLeft := l.drag.startWidths[leftIdx] + delta
	newLeft = max(minLeft, min(newLeft, pool-minRight))
	newRight := pool - newLeft

	l.panes[leftIdx].width = newLeft
	l.panes[rightIdx].width = newRight
}

func (l *paneLayout) snapshotWidths() []int {
	w := make([]int, len(l.panes))
	for i, p := range l.panes {
		w[i] = p.width
	}
	return w
}

// PaneWidth returns the current width of pane i (including border).
func (l *paneLayout) PaneWidth(i int) int {
	if i < 0 || i >= len(l.panes) {
		return 0
	}
	return l.panes[i].width
}
