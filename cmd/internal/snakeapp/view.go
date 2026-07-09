package snakeapp

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// Frame geometry. The board pane is fixed on BOTH axes: its border is the
// collision wall, so it must hug the grid exactly — a pane that stretched
// with the terminal would draw walls the snake dies before reaching. The
// side panel matches the board's height for a flush layout.
const (
	boardPaneW = gridW*cellW + 2 // +2 for the pane border
	boardPaneH = gridH + 2       // +2 for the pane border
	frameW     = 80
	frameH     = 24
	overlayW   = 34
)

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	w := ctx.Width
	if s.width > 0 {
		w = s.width
	}
	if w <= 0 {
		w = frameW
	}
	h := s.height
	if h <= 0 {
		h = frameH
	}

	boardTitle := fmt.Sprintf("SNAKE  score %d  lvl %d", s.score, s.level)
	boardPane := layout.Col{
		NodeBase: layout.NodeBase{
			W:        layout.Fixed(boardPaneW),
			H:        layout.Fixed(boardPaneH),
			Bordered: true,
			Chrome:   borderChrome(boardTitle, pal.accent),
		},
		Children: []layout.Node{layout.Text{String: s.renderBoard()}},
	}

	panel := layout.Col{
		NodeBase: layout.NodeBase{
			W:        layout.Grow(1),
			H:        layout.Fixed(boardPaneH),
			Bordered: true,
			Chrome:   borderChrome("STATS", pal.panel),
		},
		Children: s.panelLines(),
	}

	children := []layout.Node{
		layout.Row{
			NodeBase: layout.NodeBase{H: layout.Fixed(boardPaneH)},
			Children: []layout.Node{boardPane, panel},
		},
	}
	if s.paused {
		children = append(children, pauseOverlay())
	}
	if s.over {
		children = append(children, gameOverOverlay(s))
	}

	content, _ := layout.SolveAndCompose(layout.Col{Children: children}, w, h)

	title := fmt.Sprintf("flat-game — level %d", s.level)
	if s.paused {
		title = "flat-game — paused"
	}
	if s.over {
		title = "flat-game — game over"
	}
	return flatte.Frame{Content: content, Title: title}
}

// renderBoard draws the grid as one styled Text leaf: gridH lines, each gridW
// cells of cellW columns. View stays a pure function of state — no randomness,
// no wall clock.
func (s *State) renderBoard() string {
	occupied := make(map[point]int, len(s.snake)) // point -> index in snake
	for i, p := range s.snake {
		occupied[p] = i
	}
	head := s.snake[0]

	var b strings.Builder
	for y := 0; y < gridH; y++ {
		for x := 0; x < gridW; x++ {
			p := point{x, y}
			switch {
			case p == head:
				b.WriteString(headCell)
			case occupiedContains(occupied, p):
				b.WriteString(bodyCell)
			case p == s.food:
				b.WriteString(foodCell)
			default:
				b.WriteString(emptyCell)
			}
		}
		if y < gridH-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func occupiedContains(m map[point]int, p point) bool {
	_, ok := m[p]
	return ok
}

// panelLines is the side panel body: live stats then a control legend.
func (s *State) panelLines() []layout.Node {
	label := func(k, v string) layout.Node {
		line := " " + lipgloss.NewStyle().Foreground(pal.muted).Render(k) +
			lipgloss.NewStyle().Foreground(pal.text).Render(v)
		return layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, String: line}
	}
	blank := func() layout.Node {
		return layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}}
	}
	help := func(keys, what string) layout.Node {
		line := " " + lipgloss.NewStyle().Foreground(pal.accent).Render(keys) + " " +
			lipgloss.NewStyle().Foreground(pal.muted).Render(what)
		return layout.Text{NodeBase: layout.NodeBase{H: layout.Fixed(1)}, String: line}
	}

	status := "playing"
	switch {
	case s.over:
		status = "game over"
	case s.paused:
		status = "paused"
	}

	return []layout.Node{
		blank(),
		label("score  ", fmt.Sprintf("%d", s.score)),
		label("high   ", fmt.Sprintf("%d", s.HighScore)),
		blank(),
		label("level  ", fmt.Sprintf("%d / %d", s.level, maxLevel)),
		label("speed  ", fmt.Sprintf("%dms", s.moveIntervalMs())),
		label("length ", fmt.Sprintf("%d", len(s.snake))),
		label("state  ", status),
		blank(),
		help("←↑↓→", "move"),
		help("wasd", "move"),
		help("p", "pause"),
		help("r", "restart"),
		help("q", "quit"),
	}
}

// pauseOverlay and gameOverOverlay are centered Overlay nodes with a styled
// Chrome frame — the single SolveAndCompose pass clears their rect and composes
// them over the board (flat-docker's modalNode pattern).
func pauseOverlay() layout.Node {
	return overlayBox("PAUSED", pal.accent, []string{
		"",
		center("game paused"),
		"",
		center("p resume   q quit"),
		"",
	})
}

func gameOverOverlay(s *State) layout.Node {
	final := "new high score!"
	if s.score < s.HighScore {
		final = fmt.Sprintf("high score %d", s.HighScore)
	}
	return overlayBox("GAME OVER", pal.bad, []string{
		"",
		center(fmt.Sprintf("final score %d", s.score)),
		center(final),
		"",
		center("r restart   q quit"),
		"",
	})
}

func center(s string) string {
	return lipgloss.NewStyle().Width(overlayW - 2).Align(lipgloss.Center).Render(s)
}

func overlayBox(title string, fg color.Color, lines []string) layout.Node {
	children := make([]layout.Node, len(lines))
	for i, ln := range lines {
		children[i] = layout.Text{
			NodeBase: layout.NodeBase{H: layout.Fixed(1)},
			String:   lipgloss.NewStyle().Foreground(pal.text).Render(ln),
		}
	}
	return layout.Col{
		NodeBase: layout.NodeBase{
			Overlay:  true,
			Bordered: true,
			W:        layout.Fixed(overlayW),
			Chrome:   borderChrome(title, fg),
		},
		Children: children,
	}
}
