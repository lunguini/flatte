package window

import (
	"bytes"
	"testing"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestScrolledTextIsDrawnMoved(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	r := layout.PointRect{X: 4, Y: 4, W: 60, H: m.Line}
	op := layout.TextOp{Text: "the quick brown fox jumps over", Truncate: true, Rect: r, Clip: r}
	scene := layout.Scene{Ops: []layout.Op{op}}
	start := time.Unix(1000, 0)
	draw := func(p presentation) []byte {
		return renderImage(scene, darkTheme, m, 100, 40, 2, nil, p).Pix
	}
	still := draw(presentation{})
	at := func(d time.Duration) []byte {
		return draw(presentation{now: start.Add(d), scroll: textScroll{text: op.Text, rect: r, since: start}})
	}
	if !bytes.Equal(at(0), still) {
		t.Error("it moves at once")
	}
	mid, end := at(scrollRest+500*time.Millisecond), at(scrollRest+time.Minute)
	if bytes.Equal(mid, still) || bytes.Equal(end, mid) {
		t.Error("it does not scroll")
	}
	if !bytes.Equal(at(scrollRest+2*time.Minute), end) {
		t.Error("it does not stay at its end")
	}
	// Nothing is drawn outside its rect.
	blank := renderImage(layout.Scene{}, darkTheme, m, 100, 40, 2, nil, presentation{}).Pix
	stride := 100 * 2 * 4
	for y := range 40 * 2 {
		row := end[y*stride : (y+1)*stride]
		if !bytes.Equal(row[:4*2*4], blank[y*stride:y*stride+4*2*4]) || !bytes.Equal(row[65*2*4:], blank[y*stride+65*2*4:(y+1)*stride]) {
			t.Fatalf("drawn outside its place in row %d", y)
		}
	}
}
