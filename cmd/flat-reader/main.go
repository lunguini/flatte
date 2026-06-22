package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
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
	s.vp.SetSize(
		max(flatui.CardBodyWidth(width), 1),
		max(flatui.CardBodyHeight(height, pinnedRows), 1),
	)
}

// wheelLines is how many lines one mouse-wheel notch scrolls.
const wheelLines = 3

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.ResizeEvent:
		s.layout(e.Width, e.Height)
	case flatte.KeyEvent:
		handleKey(s, e, fx)
	case flatte.MouseEvent:
		handleMouse(s, e)
	}
}

func handleMouse(s *State, m flatte.MouseEvent) {
	switch m.Button {
	case flatte.MouseWheelUp:
		s.vp.LineUp(wheelLines)
	case flatte.MouseWheelDown:
		s.vp.LineDown(wheelLines)
	}
}

func handleKey(s *State, key flatte.KeyEvent, fx flatte.Effects[State]) {
	if key.Key != flatte.KeyCharacter {
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

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
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

	return flatte.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func main() {
	if err := flatte.Run(context.Background(), flatte.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flatte.WithMouse(flatte.MouseModeCellMotion)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
