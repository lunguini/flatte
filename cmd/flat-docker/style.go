package main

import (
	"image/color"
	"os"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lunguini/flatte/flatui"
)

type palette struct {
	accent color.Color
	panel  color.Color
	muted  color.Color
	text   color.Color
	good   color.Color
	bad    color.Color
	bg     color.Color
	tabBg  color.Color // inactive tab fill (slightly lighter than bg)
	dark   color.Color // high-contrast text for accent backgrounds
}

func defaultPalette() palette {
	return palette{
		accent: lipgloss.Color("117"),
		panel:  lipgloss.Color("240"),
		muted:  lipgloss.Color("245"),
		text:   lipgloss.Color("252"),
		good:   lipgloss.Color("114"),
		bad:    lipgloss.Color("203"),
		bg:     lipgloss.Color("236"),
		tabBg:  lipgloss.Color("238"),
		dark:   lipgloss.Color("23"),
	}
}

var pal = defaultPalette()

// pickGlyphs reads FLAT_DOCKER_GLYPHS and returns the framework glyph preset.
func pickGlyphs() flatui.TabGlyphs {
	switch strings.ToLower(os.Getenv("FLAT_DOCKER_GLYPHS")) {
	case "safe", "ascii", "off", "0", "false":
		return flatui.TabGlyphsSafe
	default:
		return flatui.TabGlyphsPowerline
	}
}

// tipForGlyphs returns a one-time startup hint about glyph choice.
func tipForGlyphs(g flatui.TabGlyphs) string {
	if g == flatui.TabGlyphsPowerline {
		return "tip   if tab edges look like boxes, install a Nerd Font or set FLAT_DOCKER_GLYPHS=safe"
	}
	return "tip   using safe (ASCII) glyphs; set FLAT_DOCKER_GLYPHS=powerline with a Nerd Font for nicer tabs"
}

const paneBorderRows = 1 // bottom padding only (top=0 so headers are flush)
const paneBorderCols = 2 // left + right padding
const panePadding = 1    // cells of padding per side

func paneStyle(width, height int, focused bool, border bool) lipgloss.Style {
	s := lipgloss.NewStyle().
		Width(width).
		Height(height).
		MaxHeight(height).
		MaxWidth(width).
		Padding(0, panePadding, panePadding, panePadding) // top=0, sides=1, bottom=1
	if border {
		borderFg := pal.panel
		if focused {
			borderFg = pal.accent
		}
		s = s.Border(lipgloss.RoundedBorder()).BorderForeground(borderFg)
	}
	return s
}

func truncateToWidth(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= width {
		return s
	}
	return ansiTruncate(s, width-1) + "…"
}

func ansiTruncate(s string, width int) string {
	return lipgloss.NewStyle().MaxWidth(width).Render(s)
}

func ansiStrip(s string) string {
	out := make([]rune, 0, len(s))
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
		out = append(out, r)
	}
	return string(out)
}

func truncateForActivity(s string) string {
	if len(s) > 60 {
		return s[:60] + "…"
	}
	return s
}

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

func sparkline(history []float64, c color.Color) string {
	if len(history) == 0 {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Render("(no history)")
	}
	runes := make([]rune, len(history))
	for i, v := range history {
		idx := int(v/100.0*float64(len(sparkBlocks))) % len(sparkBlocks)
		if idx < 0 {
			idx = 0
		}
		runes[i] = sparkBlocks[idx]
	}
	return lipgloss.NewStyle().Foreground(c).Render(string(runes))
}
