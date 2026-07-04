package main

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui/layout"
)

// palette is the single place colors are defined (flat-docker convention).
type palette struct {
	accent  color.Color // board frame + titles
	panel   color.Color // side panel frame
	text    color.Color
	muted   color.Color
	bad     color.Color // game-over frame
	boardBg color.Color // empty board cell
	body    color.Color // snake body
	head    color.Color // snake head
	food    color.Color // food glyph
}

func defaultPalette() palette {
	return palette{
		accent:  lipgloss.Color("45"),  // cyan
		panel:   lipgloss.Color("240"), // grey
		text:    lipgloss.Color("252"),
		muted:   lipgloss.Color("245"),
		bad:     lipgloss.Color("203"),
		boardBg: lipgloss.Color("234"), // near-black board
		body:    lipgloss.Color("35"),  // green
		head:    lipgloss.Color("47"),  // bright green
		food:    lipgloss.Color("203"), // coral
	}
}

var pal = defaultPalette()

// Cell strings are built once from the palette; a board render just picks the
// right one per grid cell. Each cell is cellW columns wide so it reads square.
// Glyphs (not just background colors) distinguish head/body/food so the cells
// survive ANSI stripping in goldens — an empty cell is spaces, everything else
// is a visible character.
var (
	emptyCell = lipgloss.NewStyle().Background(pal.boardBg).Render(strings.Repeat(" ", cellW))
	bodyCell  = lipgloss.NewStyle().Background(pal.boardBg).Foreground(pal.body).Render("▓▓")
	headCell  = lipgloss.NewStyle().Background(pal.boardBg).Foreground(pal.head).Render("██")
	foodCell  = lipgloss.NewStyle().Background(pal.boardBg).Foreground(pal.food).Bold(true).Render("◆ ")
)

// borderChrome paints a rounded border in fg with an optional bold title
// embedded in the top rule, mirroring flat-docker's modalChrome. It fills only
// the border ring (the interior stays as spaces the children paint over), so it
// works for the board pane, the side panel, and the overlays alike.
func borderChrome(title string, fg color.Color) func(layout.Rect) string {
	return func(r layout.Rect) string {
		if r.W < 4 || r.H < 2 {
			return ""
		}
		b := lipgloss.NewStyle().Foreground(fg)
		var top string
		if title == "" {
			top = b.Render("╭" + strings.Repeat("─", r.W-2) + "╮")
		} else {
			t := lipgloss.NewStyle().Bold(true).Foreground(fg).Render(" " + title + " ")
			trail := r.W - 2 - lipgloss.Width(t) - 1 // after "╭─" + title, before "╮"
			if trail < 0 {
				trail = 0
			}
			top = b.Render("╭─") + t + b.Render(strings.Repeat("─", trail)+"╮")
		}
		side := b.Render("│") + strings.Repeat(" ", r.W-2) + b.Render("│")
		bottom := b.Render("╰" + strings.Repeat("─", r.W-2) + "╯")
		rows := make([]string, r.H)
		rows[0] = top
		for i := 1; i < r.H-1; i++ {
			rows[i] = side
		}
		rows[r.H-1] = bottom
		return strings.Join(rows, "\n")
	}
}
