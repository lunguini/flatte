package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lunguini/flat/internal/flatcore"
	"github.com/lunguini/flat/internal/flatui"
)

// document is fixed so goldens stay deterministic. The long line near the top
// exercises soft-wrapping; the numbered lines make the visible window obvious.
const document = `This first paragraph is intentionally longer than the viewport width so that soft wrapping splits it across several rows inside the card.
line 02
line 03
line 04
line 05
line 06
line 07
line 08
line 09
line 10
line 11
line 12
line 13
line 14
line 15
line 16
line 17
line 18
line 19
line 20
line 21
line 22
line 23
line 24
line 25`

type State struct {
	vp flatui.Viewport
}

func NewState() *State {
	s := &State{}
	s.vp.SetWrappedContent(document) // wrap deferred until first ResizeEvent
	return s
}

// layout sizes the viewport to the space left after the pinned chrome. The
// card adds a 1-col border + 2-col padding on each side (6 columns total) and
// a top+bottom border (2 rows); the pinned content rows are title, subtitle, a
// blank, a blank, and the footer (5 rows). Manual arithmetic on purpose —
// decision-gate evidence for whether a flatui height-budgeting helper is
// warranted (see .docs/evaluation.md).
func (s *State) layout(width, height int) {
	const pinnedRows = 5 // title, subtitle, blank, blank, footer
	const vChrome = 2    // card top+bottom border
	const hChrome = 6    // card border (2) + padding (4)
	vpHeight := max(height-pinnedRows-vChrome, 1)
	vpWidth := max(width-hChrome, 1)
	s.vp.SetSize(vpWidth, vpHeight)
}

// wheelLines is how many lines one mouse-wheel notch scrolls.
const wheelLines = 3

func Handle(s *State, ev flatcore.Event, fx flatcore.Effects[State]) {
	switch e := ev.(type) {
	case flatcore.ResizeEvent:
		s.layout(e.Width, e.Height)
	case flatcore.KeyEvent:
		handleKey(s, e, fx)
	case flatcore.MouseEvent:
		handleMouse(s, e)
	}
}

func handleMouse(s *State, m flatcore.MouseEvent) {
	switch m.Button {
	case flatcore.MouseWheelUp:
		s.vp.LineUp(wheelLines)
	case flatcore.MouseWheelDown:
		s.vp.LineDown(wheelLines)
	}
}

func handleKey(s *State, key flatcore.KeyEvent, fx flatcore.Effects[State]) {
	if key.Key != flatcore.KeyCharacter {
		return
	}
	switch key.Rune {
	case 'j':
		s.vp.LineDown(1)
	case 'k':
		s.vp.LineUp(1)
	case 'd':
		s.vp.HalfPageDown()
	case 'u':
		s.vp.HalfPageUp()
	case 'f':
		s.vp.PageDown()
	case 'b':
		s.vp.PageUp()
	case 'g':
		s.vp.GotoTop()
	case 'G':
		s.vp.GotoBottom()
	case 'q':
		fx.Quit()
	}
}

func View(s *State, ctx flatcore.RenderContext) flatcore.Frame {
	footer := flatui.Subtle(fmt.Sprintf(
		"j/k line  d/u half  f/b page  g/G ends  q quit   %3.0f%%",
		s.vp.ScrollPercent()*100))

	lines := []string{
		flatui.Title("Flat Reader"),
		flatui.Subtle("scrollable viewport sample"),
		"",
	}
	lines = append(lines, strings.Split(s.vp.View(), "\n")...)
	lines = append(lines, "", footer)

	return flatcore.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func main() {
	if err := flatcore.Run(context.Background(), flatcore.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flatcore.WithMouse(flatcore.MouseModeCellMotion)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
