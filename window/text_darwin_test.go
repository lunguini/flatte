package window

import (
	"testing"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestPasteboardRoundTrip(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	pb := pasteboard{name: "dev.flatte.window.test"}
	defer withPool(func() { pb.board().Send(sel("releaseGlobally")) })
	pb.set("héllo\n界")
	got := pb.get()
	if got == "" {
		// The pasteboard server is a system service; a sandbox can deny it.
		t.Skip("pasteboard service unavailable (sandboxed?)")
	}
	if got != "héllo\n界" {
		t.Fatalf("pasteboard = %q", got)
	}
}

// TestWindowTextCarets: carets the window reports are where it paints
// them — every boundary's painted x maps back to it — and row steps move
// between the painter's rows.
func TestWindowTextCarets(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	text := windowText{th: func() theme { return darkTheme }}
	op := layout.InputOp{
		ID: "ed", Rect: layout.PointRect{X: 10, Y: 20, W: 14 * m.Column, H: 6 * m.Line},
		Value: "the quick brown fox jumps over the lazy dog\nnext 界 line", Multiline: true, Wrap: true, Focused: true,
	}
	fl := darkTheme.layoutField(op, "")
	if len(fl.rows) < 4 {
		t.Fatalf("want a wrapped value, got %d rows", len(fl.rows))
	}
	for i, row := range fl.rows {
		y := fl.y0 + float64(i-fl.first)*fl.pitch + fl.pitch/2
		for b := row.start; b <= row.end; b++ {
			if !runeStart(op.Value, b) {
				continue
			}
			x := fl.x0 - fl.scroll + fl.x(i, b)
			if got := text.caretAt(op, x+0.1, y); got != b {
				t.Fatalf("row %d byte %d at x %.1f: caretAt = %d", i, b, x, got)
			}
		}
	}
	// Clicks off the field clamp to its rows.
	if got := text.caretAt(op, 0, 0); got != 0 {
		t.Errorf("above-left caret %d, want 0", got)
	}
	if got := text.caretAt(op, 1e4, 1e4); got != len(op.Value) {
		t.Errorf("below-right caret %d, want the end", got)
	}
	// Down from row 0 lands in row 1 near the same x; up from row 0 stays.
	x := text.caretX(op, 4) // after "the "
	down := text.caretStep(op, 4, 1, x)
	if caretRow(fl.rows, down) != 1 {
		t.Fatalf("down from row 0 lands on row %d", caretRow(fl.rows, down))
	}
	if got := fl.x(1, down); got < x-m.Column || got > x+m.Column {
		t.Errorf("down moved x from %.1f to %.1f", x, got)
	}
	if got := text.caretStep(op, 4, -1, x); got != 4 {
		t.Errorf("up from the first row = %d, want unchanged 4", got)
	}
	last := len(op.Value) - 2
	if got := text.caretStep(op, last, 1, text.caretX(op, last)); got != last {
		t.Errorf("down from the last row = %d, want unchanged %d", got, last)
	}
}

func TestCaretBlinkPhases(t *testing.T) {
	for _, tc := range []struct {
		since time.Duration
		on    bool
	}{
		{0, true}, {caretBlink - 1, true}, {caretBlink, false}, {2*caretBlink - 1, false},
		{2 * caretBlink, true}, {-time.Second, true},
	} {
		if got := caretOn(tc.since); got != tc.on {
			t.Errorf("caretOn(%v) = %v, want %v", tc.since, got, tc.on)
		}
	}
}

// TestSnapshotCaretStaysInItsField: a caret on a row of a field that a
// Scroll hides is clipped away with the row, not drawn over what is below.
func TestSnapshotCaretStaysInItsField(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{
			layout.Scroll{NodeBase: layout.NodeBase{ID: "sc", H: layout.Fixed(2)}, Child: layout.Input{
				NodeBase: layout.NodeBase{ID: "in", H: layout.Fixed(4)}, Value: "a\nb\nc\nd", Caret: 7, Focused: true, Multiline: true,
			}},
			layout.Label{NodeBase: layout.NodeBase{ID: "below", H: layout.Fixed(3)}},
		}}
	}
	m := metrics()
	img, err := Snapshot(snapApp(root), Options{Width: 20, Height: 5}, 2)
	if err != nil {
		t.Fatal(err)
	}
	s := layout.LayoutScene(root(), 20*m.Column, 5*m.Line, m)
	below := s.Rects["below"]
	if region(img, below, 2, toColor(darkTheme.caret), 40) {
		t.Fatal("caret of a scrolled-out row drawn below the scroll")
	}
}
