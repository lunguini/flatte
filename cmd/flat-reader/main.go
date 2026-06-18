package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lunguini/flat"
	"github.com/lunguini/flat/flatui"
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

func Handle(s *State, ev flat.Event, fx flat.Effects[State]) {
	switch e := ev.(type) {
	case flat.ResizeEvent:
		s.layout(e.Width, e.Height)
	case flat.KeyEvent:
		handleKey(s, e, fx)
	case flat.MouseEvent:
		handleMouse(s, e)
	}
}

func handleMouse(s *State, m flat.MouseEvent) {
	switch m.Button {
	case flat.MouseWheelUp:
		s.vp.LineUp(wheelLines)
	case flat.MouseWheelDown:
		s.vp.LineDown(wheelLines)
	}
}

func handleKey(s *State, key flat.KeyEvent, fx flat.Effects[State]) {
	if key.Key != flat.KeyCharacter {
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

func View(s *State, ctx flat.RenderContext) flat.Frame {
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

	return flat.Frame{Content: flatui.Card(lines, ctx.Width)}
}

func main() {
	if err := flat.Run(context.Background(), flat.App[State]{
		State:  NewState(),
		Handle: Handle,
		View:   View,
	}, flat.WithMouse(flat.MouseModeCellMotion)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
