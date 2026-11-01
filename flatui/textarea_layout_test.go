package flatui

import (
	"image/color"
	"math/rand/v2"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"

	"github.com/lunguini/flatte/flatui/layout"
)

// cellGrid is s drawn into a w×h buffer: each cell's content and whether
// its background is the selection's.
func cellGrid(s string, w, h int, selBg color.Color) []string {
	buf := uv.NewScreenBuffer(w, h)
	uv.NewStyledString(s).Draw(buf, buf.Bounds())
	rows := make([]string, h)
	for y := range h {
		var b strings.Builder
		for x := range w {
			c := buf.CellAt(x, y)
			switch {
			case c == nil || c.Width == 0:
				continue
			case c.Style.Bg != nil && colorEq(c.Style.Bg, selBg):
				b.WriteString("[" + c.Content + "]")
			case c.Content == "":
				b.WriteByte(' ')
			default:
				b.WriteString(c.Content)
			}
		}
		rows[y] = strings.TrimRight(b.String(), " ")
	}
	return rows
}

func colorEq(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// TestTextareaLayoutMatchesView drives random edits, moves, selections,
// sizes, and wrap modes, and checks the node composes the cells and cursor
// ViewWithSelection and CursorCell give.
func TestTextareaLayoutMatchesView(t *testing.T) {
	selBg := lipgloss.Color("#123456")
	sel := lipgloss.NewStyle().Background(selBg)
	render := func(text string, selected bool) string {
		if selected {
			return sel.Render(text)
		}
		return text
	}
	runes := []rune("ab  cd界e🙂́ xyz")
	for seed := range 300 {
		rng := rand.New(rand.NewPCG(uint64(seed), 7))
		var ta Textarea
		w, h := 1+rng.IntN(12), 1+rng.IntN(6)
		ta.SetSize(w, h)
		ta.SetSoftWrap(rng.IntN(2) == 0)
		ta.SetValue("")
		for range 40 + rng.IntN(80) {
			switch rng.IntN(14) {
			case 0, 1, 2, 3, 4:
				ta.Insert(runes[rng.IntN(len(runes))])
			case 5:
				ta.InsertNewline()
			case 6:
				ta.Backspace()
			case 7:
				ta.MoveLeft()
			case 8:
				ta.MoveUp()
			case 9:
				ta.MoveDown()
			case 10:
				ta.MoveLeftSelecting()
			case 11:
				ta.MoveUpSelecting()
			case 12:
				ta.MoveRightSelecting()
			case 13:
				ta.MoveLineStart()
			}
		}
		in := ta.Layout("ta", true)
		in.W, in.H = layout.Fixed(w), layout.Fixed(h)
		in.SelectionStyle = layout.TextStyle{Bg: selBg}
		comp := layout.Compose(layout.Col{Children: []layout.Node{in}}, w, h)
		got := cellGrid(comp.Frame, w, h, selBg)
		want := cellGrid(ta.ViewWithSelection(render), w, h, selBg)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("seed %d (%dx%d wrap=%v value %q):\nnode:\n%s\nview:\n%s", seed, w, h, ta.SoftWrap(), ta.Value(),
				strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
		x, y, ok := comp.Cursor()
		cx, cy := ta.CursorCell()
		// A cursor past a full row's end sits just outside the window, which
		// the frame clips; inside it, the node agrees with the widget.
		inside := cx >= 0 && cx < w && cy >= 0 && cy < h
		if ok != inside || (ok && (x != cx || y != cy)) {
			t.Fatalf("seed %d (%dx%d wrap=%v value %q row %d col %d): cursor %d,%d (%v), want %d,%d", seed, w, h, ta.SoftWrap(), ta.Value(), ta.Row(), ta.Col(), x, y, ok, cx, cy)
		}
	}
}

func TestTextareaLayoutFields(t *testing.T) {
	var ta Textarea
	ta.SetSize(10, 2)
	ta.SetValue("one\ntwo\nthree")
	ta.MoveDown()
	ta.MoveDown()
	ta.MoveRightSelecting()
	ta.MoveRightSelecting()
	in := ta.Layout("ed", false)
	if in.ID != "ed" || in.Focused || !in.Multiline || in.Wrap || in.Value != "one\ntwo\nthree" {
		t.Fatalf("layout = %+v", in)
	}
	// "three" starts at byte 8; the cursor is two right of it.
	if in.Caret != 10 || in.SelStart != 8 || in.SelEnd != 10 || in.OffsetY != 1 {
		t.Fatalf("caret %d sel %d..%d offset %d, want 10, 8..10, 1", in.Caret, in.SelStart, in.SelEnd, in.OffsetY)
	}
	ta.SetSoftWrap(true)
	if in := ta.Layout("ed", true); !in.Wrap {
		t.Fatal("soft wrap not carried")
	}
}

func TestTextareaInsertText(t *testing.T) {
	var ta Textarea
	ta.SetValue("head tail")
	ta.MoveTo(5)
	ta.InsertText("one\r\ntwo\nthree ")
	if got := ta.Value(); got != "head one\ntwo\nthree tail" {
		t.Fatalf("value %q", got)
	}
	if ta.Row() != 2 || ta.Col() != len("three ") {
		t.Fatalf("cursor %d:%d, want after the paste", ta.Row(), ta.Col())
	}
	// A selection is replaced; text without breaks stays on its line.
	ta.MoveTo(0)
	ta.MoveToSelecting(4)
	ta.InsertText("HEAD")
	if got := ta.Value(); got != "HEAD one\ntwo\nthree tail" || ta.Col() != 4 {
		t.Fatalf("replace: %q col %d", got, ta.Col())
	}
}

func TestTextareaMoveToAndSelectAll(t *testing.T) {
	var ta Textarea
	ta.SetValue("ab\néx\n界")
	for _, tc := range []struct{ to, row, col int }{
		{0, 0, 0}, {2, 0, 2}, {3, 1, 0},
		{4, 1, 0}, // inside "é": backs off to its start
		{6, 1, 3}, {7, 1, 4}, {8, 2, 0}, {99, 2, 3}, {-5, 0, 0},
	} {
		ta.MoveTo(tc.to)
		if ta.Row() != tc.row || ta.Col() != tc.col {
			t.Errorf("MoveTo(%d) = %d:%d, want %d:%d", tc.to, ta.Row(), ta.Col(), tc.row, tc.col)
		}
		if _, sel := ta.Selection(); sel {
			t.Errorf("MoveTo(%d) left a selection", tc.to)
		}
	}
	ta.MoveTo(1)
	ta.MoveToSelecting(6)
	if got := ta.SelectedText(); got != "b\ne\u0301" {
		t.Fatalf("dragged selection %q", got)
	}
	// The node's caret is the offset MoveTo took.
	if in := ta.Layout("ed", true); in.Caret != 6 || in.SelStart != 1 || in.SelEnd != 6 {
		t.Fatalf("layout caret %d sel %d..%d", in.Caret, in.SelStart, in.SelEnd)
	}
	ta.SelectAll()
	if got := ta.SelectedText(); got != ta.Value() || ta.Row() != 2 || ta.Col() != len("界") {
		t.Fatalf("SelectAll selected %q, cursor %d:%d", got, ta.Row(), ta.Col())
	}
}
