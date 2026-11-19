package flatte

import "testing"

func TestMouseEventScrollLinesAndPoint(t *testing.T) {
	for _, c := range []struct {
		m    MouseEvent
		want float64
	}{
		{MouseEvent{Button: MouseWheelDown}, 1},
		{MouseEvent{Button: MouseWheelUp}, -1},
		{MouseEvent{Button: MouseWheelRight}, 1},
		{MouseEvent{Button: MouseWheelLeft}, -1},
		{MouseEvent{Button: MouseWheelDown, Scroll: 0.25}, 0.25},
		{MouseEvent{Button: MouseWheelUp, Scroll: -2.5}, -2.5},
		{MouseEvent{Button: MouseLeft}, 0},
	} {
		if got := c.m.ScrollLines(); got != c.want {
			t.Errorf("%+v: ScrollLines %v, want %v", c.m, got, c.want)
		}
	}
	if x, y := (MouseEvent{X: 3, Y: 4}).Point(); x != 3 || y != 4 {
		t.Errorf("whole-cell Point = %v, %v", x, y)
	}
	if x, y := (MouseEvent{X: 3, Y: 4, PreciseX: 3.5, PreciseY: 4.75, HasPrecise: true}).Point(); x != 3.5 || y != 4.75 {
		t.Errorf("precise Point = %v, %v", x, y)
	}
	if !applyRunOptions(nativeRunConfig(), WithPreciseScrolling()).config.preciseScrolling {
		t.Error("WithPreciseScrolling not applied")
	}
}
