package window

import (
	"strings"
	"testing"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestTextScrollOffset(t *testing.T) {
	start := time.Unix(1000, 0)
	op := layout.TextOp{Text: "long", Truncate: true, Rect: layout.PointRect{W: 10, H: 5}}
	s := textScroll{text: "long", rect: op.Rect, since: start}
	for _, tc := range []struct {
		after time.Duration
		want  float64
		more  bool
	}{
		{0, 0, true},
		{scrollRest, 0, true},
		{scrollRest + time.Second, scrollSpeed, true},
		{scrollRest + time.Minute, 90, false}, // at its end, it stays
	} {
		if x, more := s.offset(op, 90, start.Add(tc.after)); x != tc.want || more != tc.more {
			t.Errorf("after %v: %v %v, want %v %v", tc.after, x, more, tc.want, tc.more)
		}
	}
	later := start.Add(time.Minute)
	other := op
	other.Rect.Y = 7
	for name, o := range map[string]layout.TextOp{"another place": other, "another text": {Text: "x", Truncate: true, Rect: op.Rect}, "not truncated": {Text: "long", Rect: op.Rect}} {
		if x, more := s.offset(o, 90, later); x != 0 || more {
			t.Errorf("%s scrolls: %v %v", name, x, more)
		}
	}
	if x, more := s.offset(op, 0, later); x != 0 || more {
		t.Errorf("text that fits scrolls: %v %v", x, more)
	}
}

func TestHoverScrollsCutOffText(t *testing.T) {
	st := controlsState()
	h, _ := newTestHost(t, st)
	now := time.Unix(1000, 0)
	h.clock = func() time.Time { return now }
	h.start(400, 300)
	h.render()
	long := strings.Repeat("word ", 40)
	rect := layout.PointRect{X: 10, Y: 10, W: 60, H: 20}
	fits := layout.PointRect{X: 10, Y: 40, W: 60, H: 20}
	s := h.surface
	s.scene.Ops = []layout.Op{
		layout.TextOp{Text: long, Truncate: true, Rect: rect, Clip: rect},
		layout.TextOp{Text: "a", Truncate: true, Rect: fits, Clip: fits},
	}
	if tip, changed := s.scrollAt(20, 15, "Row"); tip != long+"\nRow" || !changed || s.pres.scroll.text != long {
		t.Fatalf("over cut-off text: tip %q changed %v scroll %+v", tip, changed, s.pres.scroll)
	}
	since := s.pres.scroll.since
	now = now.Add(time.Second)
	if _, changed := s.scrollAt(30, 15, ""); changed || s.pres.scroll.since != since {
		t.Error("moving within it starts over")
	}
	if tip, _ := s.scrollAt(30, 15, "About "+long); tip != "About "+long {
		t.Errorf("the text twice: %q", tip)
	}
	if tip, changed := s.scrollAt(12, 45, ""); tip != "" || !changed || s.pres.scroll.text != "" {
		t.Errorf("over text that fits: %q %v %+v", tip, changed, s.pres.scroll)
	}
	s.scrollAt(20, 15, "")
	if _, changed := s.scrollAt(-1, -1, ""); !changed || s.pres.scroll.text != "" {
		t.Error("leaving the view does not stop it")
	}
	// A layer over it takes the pointer.
	s.scene.Ops = append(s.scene.Ops, layout.LayerOp{Rect: rect})
	if tip, _ := s.scrollAt(20, 15, ""); tip != "" {
		t.Errorf("through an overlay: %q", tip)
	}
}
