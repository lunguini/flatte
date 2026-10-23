package layout

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"

	"github.com/lunguini/flatte/flatui/internal/textedit"
)

// Input is a one-line text field: the value, the caret, and whether it has
// focus. It holds no editing state of its own — the app's widget (for
// example flatui.TextField, via its Layout method) owns the value and caret,
// and Input shows them. Backends draw it as a native-feeling field; the
// terminal draws the text and places the hardware cursor at the caret of the
// topmost focused Input (see Composition.Cursor).
//
// Caret is a byte offset into Value; it is clamped into range and backed off
// to the start of a rune. A focused Input scrolls horizontally to keep the
// caret visible. When Value is empty the Placeholder shows in TextSubtle.
//
// On Auto axes an Input fills the available width and is one line tall.
//
// A Bare Input draws no field of its own in a GUI — no fill, no outline,
// no focus ring — only its text and caret, so it can sit inside a box the
// app draws (a search field with an icon and a clear button, say); the app
// shows focus on that box. The terminal draws an Input bare either way.
//
// A Multiline Input is a text area (flatui.Textarea, via its Layout method,
// fills one in): "\n" breaks lines, Wrap soft-wraps them at the field's
// width, and its Auto height grows. Its selection runs from byte SelStart to
// SelEnd (either order; equal: none) and shows in SelectionStyle in the
// terminal (zero: reverse video) and as a native highlight in a GUI. OffsetX
// and OffsetY are the widget's scroll — the first visible display column,
// and the first visible line (visual row when wrapping) — clamped so the
// caret stays visible.
type Input struct {
	NodeBase
	Value       string
	Caret       int
	Placeholder string
	Focused     bool
	Style       TextStyle

	Multiline, Wrap  bool
	SelStart, SelEnd int
	SelectionStyle   TextStyle
	OffsetX, OffsetY int
	Bare             bool
}

func (in Input) Size() (Size, Size) {
	w, h := in.NodeBase.Size()
	if w.Kind == SizeAuto {
		w = Grow(1)
	}
	if h.Kind == SizeAuto && in.Multiline {
		h = Grow(1)
	}
	if h.Kind == SizeAuto {
		h = Size{Kind: SizeContent, Value: satAdd(1, in.insetSides().vert())}
	}
	return w, h
}

func (in Input) Render(r Rect) string {
	inner := r.insetBy(in.insetSides())
	var text string
	if in.Multiline && in.Value != "" {
		text = in.renderArea(inner)
	} else if in.Value == "" {
		text = TextStyle{Role: TextSubtle}.Render(ansi.Truncate(in.Placeholder, inner.W, ""))
	} else {
		start := in.scroll(inner.W)
		text = in.Style.Render(ansi.Cut(in.Value, start, start+inner.W))
	}
	return fillBox(text, r, in.padSides(), in.box())
}

// caretColumn is the display column of the caret within Value.
func (in Input) caretColumn() int {
	c := min(max(in.Caret, 0), len(in.Value))
	for c > 0 && c < len(in.Value) && !utf8.RuneStart(in.Value[c]) {
		c--
	}
	return ansi.StringWidth(in.Value[:c])
}

// scroll is the first visible display column for a field width cells wide:
// zero unless the focused caret would fall past the right edge, in which case
// the text shifts so the caret sits on the last cell.
func (in Input) scroll(width int) int {
	if !in.Focused || width <= 0 {
		return 0
	}
	return max(in.caretColumn()-width+1, 0)
}

// caret reports where the hardware cursor goes for this Input solved at r:
// on the caret cell inside the inset, if focused and the field has room.
func (in Input) caret(r Rect) (x, y int, ok bool) {
	inner := r.insetBy(in.insetSides())
	if !in.Focused || inner.W <= 0 || inner.H <= 0 {
		return 0, 0, false
	}
	if in.Multiline {
		a, s := in.area(inner)
		x, y := a.CursorCell(s)
		return satAdd(inner.X, x), satAdd(inner.Y, y), true
	}
	return satAdd(inner.X, in.caretColumn()-in.scroll(inner.W)), inner.Y, true
}

// caretAt is the byte offset into Value of the caret position at cell
// (x, y) for the Input solved at r: the boundary before the grapheme under
// the cell, or the end of the row past its text. Points outside the field
// clamp to its nearest row and column.
func (in Input) caretAt(r Rect, x, y int) int {
	inner := r.insetBy(in.insetSides())
	lx, ly := max(x-inner.X, 0), min(max(y-inner.Y, 0), max(inner.H-1, 0))
	if !in.Multiline {
		return textedit.ByteOffsetForColumn(in.Value, in.scroll(inner.W)+lx)
	}
	a, s := in.area(inner)
	var row, col int
	if a.Wrap {
		row, col = textedit.PositionForVisualCell(a.Lines, s.WrapOffset+ly, lx, a.Width)
	} else {
		row = min(s.Offset+ly, len(a.Lines)-1)
		col = textedit.ByteOffsetForColumn(a.Lines[row], s.XOffset+lx)
	}
	at := col
	for _, line := range a.Lines[:row] {
		at += len(line) + 1
	}
	return at
}

// area is a Multiline Input laid out in inner, scrolled from its offsets to
// keep the caret visible.
func (in Input) area(inner Rect) (textedit.Area, textedit.Scroll) {
	lines := strings.Split(in.Value, "\n")
	row, col := lineOf(lines, in.Caret)
	a := textedit.Area{Lines: lines, Row: row, Col: col, Width: max(inner.W, 0), Height: max(inner.H, 0), Wrap: in.Wrap}
	s := textedit.Scroll{Offset: in.OffsetY, XOffset: in.OffsetX}
	if in.Wrap {
		s = textedit.Scroll{WrapOffset: in.OffsetY}
	}
	return a, a.KeepVisible(s)
}

// lineOf is byte offset at of lines joined by "\n" as a line and a byte
// offset in it, clamped into range and backed off to the start of a rune.
func lineOf(lines []string, at int) (row, col int) {
	at = max(at, 0)
	for row < len(lines)-1 && at > len(lines[row]) {
		at -= len(lines[row]) + 1
		row++
	}
	line := lines[row]
	col = min(at, len(line))
	for col > 0 && col < len(line) && !utf8.RuneStart(line[col]) {
		col--
	}
	return row, col
}

func (in Input) renderArea(inner Rect) string {
	a, s := in.area(inner)
	selStart, selEnd := min(in.SelStart, in.SelEnd), max(in.SelStart, in.SelEnd)
	starts := make([]int, len(a.Lines))
	for i := 1; i < len(a.Lines); i++ {
		starts[i] = starts[i-1] + len(a.Lines[i-1]) + 1
	}
	selection := func(row int) (int, int, bool) {
		from, to := max(selStart-starts[row], 0), min(selEnd-starts[row], len(a.Lines[row]))
		return from, to, selStart < selEnd && from < to
	}
	sel := in.SelectionStyle.lipgloss()
	if in.SelectionStyle.zero() {
		sel = sel.Reverse(true)
	}
	return a.Render(s, selection, func(text string, selected bool) string {
		switch {
		case selected:
			return sel.Render(text)
		case in.Style.zero():
			return text
		}
		return in.Style.Render(text)
	})
}
