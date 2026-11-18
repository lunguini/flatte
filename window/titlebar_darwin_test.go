package window

import "testing"

func TestTitleBarPoints(t *testing.T) {
	for _, c := range []struct {
		h    int
		want float64
	}{{0, TitleBarHeight}, {20, TitleBarHeight}, {TitleBarHeight, TitleBarHeight}, {38, 38}, {52, 52}} {
		if got := titleBarPoints(Options{HideTitleBar: true, TitleBarHeight: c.h}); got != c.want {
			t.Errorf("TitleBarHeight %d: strip %v, want %v", c.h, got, c.want)
		}
	}
}

func TestSameRect(t *testing.T) {
	if !sameRect(cgRect(1, 2, 3, 4), cgRect(1, 2, 3, 4)) || sameRect(cgRect(1, 2, 3, 4), cgRect(1, 2, 3, 5)) {
		t.Fatal("sameRect")
	}
}
