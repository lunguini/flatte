package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui"
	"github.com/lunguini/flatte/flatui/layout"
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
	s.vp.SetWrappedContent(document) // wrap deferred until the first layout
	return s
}

// bodyID names the node the viewport fills: the card's space left after
// the pinned rows, as the frame solved it.
const bodyID = "reader.body"

// wheelLines is how many lines one mouse-wheel notch scrolls.
const wheelLines = 3

func Handle(s *State, ev flatte.Event, fx flatte.Effects[State]) {
	switch e := ev.(type) {
	case flatte.LayoutEvent:
		if r, ok := e.Rects[bodyID]; ok {
			s.vp.SetSize(max(r.W, 1), max(r.H, 1))
		}
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

// View shows the viewport's visible lines as a Text leaf: Viewport wraps
// and scrolls in cells.
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	footer := fmt.Sprintf("j/k line  d/u half  f/b page  g/G ends  q quit   %3.0f%%", s.vp.ScrollPercent()*100)
	return flatte.Frame{Root: flatui.CardLayout(
		layout.Label{Text: "Flat Reader", Style: layout.TextStyle{Role: layout.TextTitle}},
		layout.Label{Text: "scrollable viewport sample", Style: layout.TextStyle{Role: layout.TextSubtle}},
		layout.Label{},
		layout.Col{NodeBase: layout.NodeBase{ID: bodyID, H: layout.Grow(1)}, Children: []layout.Node{
			layout.Text{String: s.vp.View()},
		}},
		layout.Label{},
		layout.Label{Text: footer, Style: layout.TextStyle{Role: layout.TextSubtle}},
	)}
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
