package layout

import (
	"image/color"
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TextRole is what a piece of text is for. Backends map roles to their own
// look — the terminal to colors and emphasis, a GUI to font size and weight —
// so neither bakes the other's look into the app. The set is deliberately
// small; unknown roles read as TextBody.
type TextRole int

const (
	TextBody TextRole = iota
	TextTitle
	TextSubtle
	// TextCaption is a pane or section caption: small, semibold, upper
	// case, muted (theme.TypeScale.Caption).
	TextCaption
	// TextMeta is secondary information beside or under a name: paths,
	// counts, a row's second line (Meta, secondary color).
	TextMeta
	// TextStrong is body text that needs weight: the active tab, a
	// selected row's name, a setting's name.
	TextStrong
	// TextPageTitle is a page's one heading (PageTitle).
	TextPageTitle
)

// TextStyle is a role plus optional overrides applied on top of it. Nil
// colors and false flags leave the role's look unchanged.
type TextStyle struct {
	Role TextRole
	// Color is the text color by theme role (ColorDefault: the role's
	// own); Fg wins when set. A themed host resolves it per appearance.
	Color  TextColor
	Fg, Bg color.Color
	// Size is the font size in points (0: the role's size). A GUI sets the
	// text at it, and its lines are that much taller; terminals ignore it.
	Size float64
	// Leading is a line's height over its font's in a GUI (0: the theme's
	// line spacing): below it to set lines close, as a title over a
	// subtitle; terminals ignore it.
	Leading float64

	Bold, Italic, Underline, Strikethrough bool
	// Font is a font family to set the text in ("": the role's), as a
	// font picker shows each family in itself. A GUI falls back to its
	// own choice for a family it does not have; terminals ignore it.
	Font string
}

// Render returns text styled for a terminal. It is the single definition of
// each role's terminal look.
func (s TextStyle) Render(text string) string {
	return s.lipgloss().Render(text)
}

// zero reports whether s sets nothing. (Colors are interfaces, which ==
// would panic on for an uncomparable implementation.)
func (s TextStyle) zero() bool {
	return s.Role == TextBody && s.Color == ColorDefault && s.Size == 0 && s.Leading == 0 && s.Font == "" && s.Fg == nil && s.Bg == nil && !s.Bold && !s.Italic && !s.Underline && !s.Strikethrough
}

func (s TextStyle) lipgloss() lipgloss.Style {
	st := lipgloss.NewStyle()
	if p := cellPalette.Load(); p != nil {
		return s.themed(st, p)
	}
	switch s.Role {
	case TextTitle:
		st = st.Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("235")).Padding(0, 1)
	case TextSubtle:
		st = st.Foreground(lipgloss.Color("244"))
	}
	if c := ansiRole(s.Color); c != nil {
		st = st.Foreground(c)
	}
	if s.Fg != nil {
		st = st.Foreground(s.Fg)
	}
	if s.Bg != nil {
		st = st.Background(s.Bg)
	}
	if s.Bold {
		st = st.Bold(true)
	}
	if s.Italic {
		st = st.Italic(true)
	}
	if s.Underline {
		st = st.Underline(true)
	}
	if s.Strikethrough {
		st = st.Strikethrough(true)
	}
	return st
}

// Label is a leaf of plain text with a semantic style. Text holds no escape
// sequences (use Text for pre-styled strings). Like Text, it auto-sizes to
// its content on Auto axes, and lines break only at "\n" — unless Wrap is
// set: then it also wraps by words at its width (a word wider than the
// width breaks), in cells in the terminal and in the font in a GUI. A
// wrapping Label fills the width on an Auto width, and its Auto height is
// its wrapped line count at the width it gets.
//
// Runs makes it rich text: when set, the label's text is the runs' text in
// order, each in its own Style (a run with a zero Style takes the Label's),
// and Text is ignored. Runs wrap as one text, so a line can break inside a
// run or between two.
//
// Selectable lets a GUI user select the text with the pointer and copy it
// (⌘C): the window draws the selection itself, and the app still gets the
// clicks. Terminals ignore it — their own selection copies any text.
type Label struct {
	NodeBase
	Text       string
	Style      TextStyle
	Wrap       bool
	Runs       []Run
	Selectable bool
	// Truncate ends a line too long for the label's box in "…", in the
	// terminal and a GUI (instead of clipping it); not with Wrap. In a
	// window, text cut off scrolls while the pointer rests on it, and its
	// tooltip has it whole.
	Truncate bool
	// KeepEnd is how many bytes at the end of the text a cut-off line keeps,
	// after the "…" (a file's extension: "agentli….go"); in a box too narrow
	// for that the text is cut at its end as usual. One line only.
	KeepEnd int
	// HoverFg is the native foreground color while the pointer rests over
	// this named label. It colors the whole label, including Runs; nil keeps
	// the label's normal and per-run colors. Terminals ignore it.
	HoverFg color.Color
}

// Run is a span of a Label's text in a style of its own.
type Run struct {
	Text  string
	Style TextStyle
}

// runs is the label's text as runs with their styles resolved: Runs, or
// one run of Text in Style.
func (l Label) runs() []Run {
	if len(l.Runs) == 0 {
		return []Run{{Text: cased(l.Text, l.Style), Style: l.Style}}
	}
	out := make([]Run, len(l.Runs))
	for i, r := range l.Runs {
		if r.Style.zero() {
			r.Style = l.Style
		}
		r.Text = cased(r.Text, r.Style)
		out[i] = r
	}
	return out
}

// cased is text as its role shows it: captions in upper case, in every
// renderer (so a GUI measures what it draws).
func cased(text string, s TextStyle) string {
	if s.Role == TextCaption {
		return strings.ToUpper(text)
	}
	return text
}

// plain is the label's unstyled text, as its roles show it.
func (l Label) plain() string {
	if len(l.Runs) == 0 {
		return cased(l.Text, l.Style)
	}
	var b strings.Builder
	for _, r := range l.runs() {
		b.WriteString(r.Text)
	}
	return b.String()
}

// styled is the label's text for a terminal: text is its plain text, with
// line breaks a wrap inserted (and spaces it dropped at them).
func (l Label) styled(text string) string {
	if len(l.Runs) == 0 {
		return l.Style.Render(text)
	}
	return renderRuns(l.runs(), text)
}

// renderRuns styles text — the runs' text, with line breaks inserted and
// spaces dropped at them by a wrap — run by run. Each line's pieces are
// rendered apart, so no style spans a line break.
func renderRuns(runs []Run, text string) string {
	type at struct{ run, i int } // the next rune of the runs' text
	pos := at{}
	next := func() (rune, bool) {
		for pos.run < len(runs) && pos.i >= len(runs[pos.run].Text) {
			pos = at{run: pos.run + 1}
		}
		if pos.run == len(runs) {
			return 0, false
		}
		r, _ := utf8.DecodeRuneInString(runs[pos.run].Text[pos.i:])
		return r, true
	}
	var out, piece strings.Builder
	cur := -1 // the run piece is in
	flush := func() {
		if piece.Len() > 0 {
			out.WriteString(runs[cur].Style.Render(piece.String()))
			piece.Reset()
		}
	}
	for _, r := range text {
		o, ok := next()
		for ok && o != r && o == ' ' { // a space the wrap dropped
			pos.i++
			o, ok = next()
		}
		if !ok || o != r {
			// A break the wrap inserted.
			flush()
			out.WriteRune(r)
			continue
		}
		if r == '\n' {
			flush()
			out.WriteRune(r)
		} else {
			if pos.run != cur {
				flush()
				cur = pos.run
			}
			piece.WriteRune(r)
		}
		pos.i += utf8.RuneLen(r)
	}
	flush()
	return out.String()
}

func (l Label) Size() (Size, Size) {
	w, h := Text{NodeBase: l.NodeBase, String: l.styled(l.plain())}.Size()
	if l.Wrap && l.W.Kind == SizeAuto {
		w = Grow(1)
	}
	return w, h
}

func (l Label) Render(r Rect) string {
	text := l.plain()
	if l.Wrap {
		text = wrapCells(text, r.insetBy(l.insetSides()).W)
	}
	out := l.styled(text)
	if w := r.insetBy(l.insetSides()).W; l.Truncate && !l.Wrap && w > 0 {
		lines := strings.Split(out, "\n")
		for i, ln := range lines {
			if ansi.StringWidth(ln) <= w {
				continue
			}
			if tail := TruncatedTail(l.plain(), l.KeepEnd); tail != "" && len(lines) == 1 && ansi.StringWidth(tail)+2 <= w {
				n := ansi.StringWidth(ln)
				lines[i] = ansi.Truncate(ln, w-ansi.StringWidth(tail), "…") + ansi.Cut(ln, n-ansi.StringWidth(tail), n)
				continue
			}
			lines[i] = ansi.Truncate(ln, w, "…")
		}
		out = strings.Join(lines, "\n")
	}
	return fillBox(out, r, l.padSides(), l.box())
}

// TruncatedTail is the end of text a cut-off line keeps: its last keep
// bytes, "" when they are no part of it (or all).
func TruncatedTail(text string, keep int) string {
	if keep <= 0 || keep >= len(text) || !utf8.RuneStart(text[len(text)-keep]) || strings.Contains(text, "\n") {
		return ""
	}
	return text[len(text)-keep:]
}

// wrapHeight is a wrapping Label's Auto height in cells at a rect width
// (its unwrapped height when the width is unknown, width < 0).
func (l Label) wrapHeight(width int) Size {
	_, h := l.Size()
	if h.Kind != SizeContent || width < 0 {
		return h
	}
	inset := l.insetSides()
	lines := strings.Count(wrapCells(l.plain(), satSub(width, inset.horiz())), "\n") + 1
	return Size{Kind: SizeContent, Value: satAdd(lines, inset.vert())}
}

// wrapCells wraps text by words at width cells, breaking words wider than
// the width; width <= 0 leaves it unwrapped. "\n" still breaks lines.
func wrapCells(text string, width int) string {
	if width <= 0 {
		return text
	}
	return ansi.Wrap(text, width, "")
}

// ansiRole is a Color role in an unthemed terminal: the terminal's own
// palette (its ANSI colors and greys), so status and accent text keep their
// meaning without a theme; nil for the default and primary text.
func ansiRole(c TextColor) color.Color {
	switch c {
	case ColorSecondary:
		return lipgloss.Color("244")
	case ColorMuted:
		return lipgloss.Color("242")
	case ColorDisabled:
		return lipgloss.Color("239")
	case ColorAccent, ColorInfo:
		return ansi.BasicColor(4) // blue
	case ColorSuccess:
		return ansi.BasicColor(2)
	case ColorWarning:
		return ansi.BasicColor(3)
	case ColorDanger:
		return ansi.BasicColor(1)
	}
	return nil
}
