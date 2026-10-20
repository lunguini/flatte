package layout

import (
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
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
type Input struct {
	NodeBase
	Value       string
	Caret       int
	Placeholder string
	Focused     bool
	Style       TextStyle
}

func (in Input) Size() (Size, Size) {
	w, h := in.NodeBase.Size()
	if w.Kind == SizeAuto {
		w = Grow(1)
	}
	if h.Kind == SizeAuto {
		h = Size{Kind: SizeContent, Value: satAdd(1, in.insetSides().vert())}
	}
	return w, h
}

func (in Input) Render(r Rect) string {
	inner := r.insetBy(in.insetSides())
	var text string
	if in.Value == "" {
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
	return satAdd(inner.X, in.caretColumn()-in.scroll(inner.W)), inner.Y, true
}
