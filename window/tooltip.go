package window

import (
	"math"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

// tipDelay is how long the pointer rests on a node before its tooltip
// shows, about AppKit's own delay.
const tipDelay = 600 * time.Millisecond

// tipState is the tooltip under the pointer: its text, where the pointer
// came to it, and since when. off hides it after a press.
type tipState struct {
	text  string
	x, y  float64
	since time.Time
	off   bool
}

// shown reports whether the tooltip is drawn at now.
func (t tipState) shown(now time.Time) bool {
	return t.text != "" && !t.off && now.Sub(t.since) >= tipDelay
}

// Tooltip placement, in points: below the pointer (clear of the arrow),
// or above it when there is no room, and this far inside the window.
const (
	tipBelow  = 18
	tipAbove  = 6
	tipMargin = 4
	tipMaxW   = 320
)

// withTip is scene with p's tooltip drawn over it: a raised bubble near
// the pointer, laid out with the window's metrics so both painters draw it
// from the design's surfaces. The scene the app's hits are tested on is
// left as it is.
func (s *surface) withTip(scene layout.Scene, p presentation) layout.Scene {
	if p.tip == "" || s.width <= 0 || s.height <= 0 {
		return scene
	}
	m := s.h.metrics
	bubble := func() layout.Node {
		return layout.Col{
			NodeBase: layout.NodeBase{ID: "tip", NoPadding: true, PadLeft: 1, PadRight: 1, Box: layout.Box{Border: layout.BorderRounded, Fill: layout.FillRaised}},
			Children: []layout.Node{layout.Label{NodeBase: layout.NodeBase{W: layout.Points(0)}, Text: p.tip, Style: layout.TextStyle{Role: layout.TextMeta}}},
		}
	}
	// Measure the bubble at the text's width (bounded), then place it.
	w, _ := tipTextSize(m, p.tip)
	w = min(w, tipMaxW, s.width-2*tipMargin)
	label := func(n layout.Node) layout.Node {
		c := n.(layout.Col)
		l := c.Children[0].(layout.Label)
		l.W = layout.Points(int(math.Ceil(w)))
		l.Wrap = true
		c.Children[0] = l
		return c
	}
	probe := layout.LayoutScene(layout.Row{Align: layout.AlignStart, Children: []layout.Node{label(bubble())}}, s.width, s.height, m)
	r := probe.Rects["tip"]
	x := min(max(p.tipX, tipMargin), s.width-r.W-tipMargin)
	y := p.tipY + tipBelow
	if y+r.H > s.height-tipMargin {
		y = p.tipY - tipAbove - r.H
	}
	y = max(y, tipMargin)
	x = max(x, 0)
	placed := layout.LayoutScene(layout.Col{Align: layout.AlignStart, Children: []layout.Node{
		&layout.Spacer{NodeBase: layout.NodeBase{H: layout.Points(int(math.Round(y)))}},
		layout.Row{Align: layout.AlignStart, Children: []layout.Node{
			&layout.Spacer{NodeBase: layout.NodeBase{W: layout.Points(int(math.Round(x)))}},
			label(bubble()),
		}},
	}}, s.width, s.height, m)
	out := scene
	out.Ops = append(append([]layout.Op(nil), scene.Ops...), placed.Ops...)
	return out
}

// tipTextSize is the tooltip text's natural size in the window's metrics.
func tipTextSize(m layout.Metrics, text string) (float64, float64) {
	if m.MeasureText != nil {
		return m.MeasureText(text, layout.TextStyle{Role: layout.TextMeta}, 0)
	}
	return float64(len([]rune(text))) * m.Column, m.Line
}
