package window

import (
	"testing"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

// tipText is the drawn tooltip's text in scene, if any.
func tipText(s layout.Scene, from int) (layout.TextOp, bool) {
	for _, op := range s.Ops[from:] {
		if t, ok := op.(layout.TextOp); ok {
			return t, true
		}
	}
	return layout.TextOp{}, false
}

// The window draws a node's tooltip itself, near the pointer, once the
// pointer has rested on the node a moment (AppKit's own view tooltip never
// showed: the whole frame is one view). A press or leaving hides it.
func TestHostDrawsTooltipAfterARest(t *testing.T) {
	s := controlsState()
	h, b := newTestHost(t, s)
	now := time.Unix(1000, 0)
	h.clock = func() time.Time { return now }
	h.start(400, 300)
	h.render()
	r := h.scene.Rects["ok"]
	x, y := r.X+2, r.Y+2
	h.hover(x, y)
	p := h.presentation()
	if p.tip != "" || p.tipWait != tipDelay {
		t.Fatalf("at once: tip %q wait %v, want none yet and a wait of %v", p.tip, p.tipWait, tipDelay)
	}
	now = now.Add(tipDelay / 2)
	h.hover(x+1, y) // moving within the node keeps the rest going
	if p = h.presentation(); p.tip != "" || p.tipWait != tipDelay/2 {
		t.Fatalf("half way: tip %q wait %v", p.tip, p.tipWait)
	}
	before := b.invalidates
	now = now.Add(tipDelay / 2)
	p = h.presentation()
	if p.tip != "Confirm" || p.tipWait != 0 {
		t.Fatalf("after the rest: tip %q wait %v", p.tip, p.tipWait)
	}
	n := len(h.scene.Ops)
	drawn := h.withTip(h.scene, p)
	op, ok := tipText(drawn, n)
	if !ok || op.Text != "Confirm" {
		t.Fatalf("no tooltip text drawn: %#v", drawn.Ops[n:])
	}
	if op.Rect.Y <= y || op.Rect.X < 0 || op.Rect.X+op.Rect.W > 400 {
		t.Errorf("tooltip at %+v, want below the pointer and inside the window", op.Rect)
	}
	if _, ok := drawn.Ops[n].(layout.BoxOp); !ok {
		t.Errorf("tooltip has no surface under its text: %T", drawn.Ops[n])
	}
	if len(h.scene.Ops) != n {
		t.Error("withTip changed the hit-tested scene")
	}

	h.mouse(x, y, flatte.MouseLeft, flatte.MousePress, 0)
	if p = h.presentation(); p.tip != "" || b.invalidates == before {
		t.Errorf("a press left the tooltip %q (redraws %d)", p.tip, b.invalidates-before)
	}
	h.mouse(x, y, flatte.MouseLeft, flatte.MouseRelease, 0)
	h.hover(-1, -1)
	h.hover(x, y)
	now = now.Add(tipDelay)
	if p = h.presentation(); p.tip != "Confirm" {
		t.Fatalf("back on the button after leaving: tip %q", p.tip)
	}
	h.hover(-1, -1)
	if p = h.presentation(); p.tip != "" {
		t.Errorf("left the view, tip %q", p.tip)
	}

	// Near the bottom-right corner the bubble flips above and stays inside.
	h.hover(399, 299)
	h.pres.hovered = ""
	h.tip = tipState{text: "A longer tooltip near the corner", x: 399, y: 299, since: now.Add(-tipDelay)}
	p = h.presentation()
	op, ok = tipText(h.withTip(h.scene, p), n)
	if !ok || op.Rect.X+op.Rect.W > 400 || op.Rect.Y+op.Rect.H > 300 || op.Rect.Y >= 299 {
		t.Errorf("corner tooltip at %+v, want inside the window above the pointer", op.Rect)
	}
}
