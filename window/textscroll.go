package window

import (
	"strings"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

// Text cut off with "…" (Label.Truncate) scrolls while the pointer rests on
// it: after a moment to its end, where it stays, so nothing keeps moving
// under a resting pointer. Its whole text is in the tooltip too.
const (
	scrollRest  = 600 * time.Millisecond
	scrollSpeed = 45.0 // points a second
)

// textScroll is the cut-off text under the pointer (text "": none) and
// since when. It is known by its text and place: scenes are rebuilt under a
// resting pointer.
type textScroll struct {
	text  string
	rect  layout.PointRect
	since time.Time
}

// offset is how far op, overflow points wider than its rect, has scrolled
// at now; more reports that it goes on.
func (t textScroll) offset(op layout.TextOp, overflow float64, now time.Time) (x float64, more bool) {
	if t.text == "" || !op.Truncate || op.Text != t.text || op.Rect != t.rect || overflow <= 0 {
		return 0, false
	}
	x = min((now.Sub(t.since)-scrollRest).Seconds()*scrollSpeed, overflow)
	return max(x, 0), x < overflow
}

// cutOffAt is the topmost text at (x, y) points that is cut off there.
func (s *surface) cutOffAt(x, y float64) (layout.TextOp, bool) {
	in := func(r layout.PointRect) bool { return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H }
	for i := len(s.scene.Ops) - 1; i >= 0; i-- {
		switch op := s.scene.Ops[i].(type) {
		case layout.TextOp:
			if in(op.Rect) && in(op.Clip) {
				return op, op.Truncate && s.textWidth(op) > op.Rect.W
			}
		case layout.LayerOp:
			if in(op.Rect) {
				return layout.TextOp{}, false
			}
		}
	}
	return layout.TextOp{}, false
}

// textWidth is op's width in the window's fonts.
func (s *surface) textWidth(op layout.TextOp) float64 {
	m := s.h.metrics
	var w float64
	switch {
	case len(op.Runs) > 0 && m.MeasureRuns != nil:
		w, _ = m.MeasureRuns(op.Runs, 0)
	case m.MeasureText != nil:
		w, _ = m.MeasureText(op.Text, op.Style, 0)
	default:
		w, _ = tipTextSize(m, op.Text)
	}
	return w
}

// scrollAt follows the pointer to the cut-off text under it (outside the
// view: x < 0). It returns tooltip with that text in it, and whether what
// to draw changed.
func (s *surface) scrollAt(x, y float64, tooltip string) (string, bool) {
	op, ok := layout.TextOp{}, false
	if x >= 0 && y >= 0 {
		op, ok = s.cutOffAt(x, y)
	}
	if !ok {
		changed := s.pres.scroll.text != ""
		s.pres.scroll = textScroll{}
		return tooltip, changed
	}
	switch {
	case tooltip == "":
		tooltip = op.Text
	case !strings.Contains(tooltip, op.Text):
		tooltip = op.Text + "\n" + tooltip
	}
	if op.Selectable { // a drag selects it: it stays put
		s.pres.scroll = textScroll{}
		return tooltip, false
	}
	if t := s.pres.scroll; t.text == op.Text && t.rect == op.Rect {
		return tooltip, false
	}
	s.pres.scroll = textScroll{text: op.Text, rect: op.Rect, since: s.now()}
	return tooltip, true
}
