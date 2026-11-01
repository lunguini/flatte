package layout

import (
	"image/color"
	"reflect"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

// --- TextStyle / Label ---

func TestTextStyleRolesMatchTerminalLook(t *testing.T) {
	// The terminal look of each role is pinned to the styles samples used
	// before roles existed (flatui.Title / flatui.Subtle), byte for byte.
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("235")).Padding(0, 1)
	subtle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	for _, tc := range []struct {
		style TextStyle
		want  string
	}{
		{TextStyle{}, "hello"},
		{TextStyle{Role: TextBody}, "hello"},
		{TextStyle{Role: TextTitle}, title.Render("hello")},
		{TextStyle{Role: TextSubtle}, subtle.Render("hello")},
		{TextStyle{Role: TextRole(99)}, "hello"}, // unknown roles read as body
	} {
		if got := tc.style.Render("hello"); got != tc.want {
			t.Errorf("%+v.Render = %q, want %q", tc.style, got, tc.want)
		}
	}
}

func TestTextStyleOverrides(t *testing.T) {
	red := lipgloss.Color("1")
	for _, tc := range []struct {
		style TextStyle
		want  string
	}{
		{TextStyle{Bold: true}, lipgloss.NewStyle().Bold(true).Render("x")},
		{TextStyle{Italic: true, Underline: true, Strikethrough: true}, lipgloss.NewStyle().Italic(true).Underline(true).Strikethrough(true).Render("x")},
		{TextStyle{Fg: red}, lipgloss.NewStyle().Foreground(red).Render("x")},
		{TextStyle{Bg: red}, lipgloss.NewStyle().Background(red).Render("x")},
		// Overrides apply on top of the role.
		{TextStyle{Role: TextSubtle, Fg: red}, lipgloss.NewStyle().Foreground(red).Render("x")},
		{TextStyle{Role: TextSubtle, Bold: true}, lipgloss.NewStyle().Foreground(lipgloss.Color("244")).Bold(true).Render("x")},
	} {
		if got := tc.style.Render("x"); got != tc.want {
			t.Errorf("%+v.Render = %q, want %q", tc.style, got, tc.want)
		}
	}
}

func TestLabelSizeAndRender(t *testing.T) {
	l := Label{Text: "hi", Style: TextStyle{Role: TextTitle}}
	w, h := l.Size()
	if w != (Size{Kind: SizeContent, Value: 4}) || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Fatalf("title label size = %+v %+v, want content 4x1 (role padding counts)", w, h)
	}
	if got, want := l.Render(Rect{W: 4, H: 1}), (TextStyle{Role: TextTitle}).Render("hi"); ansi.Strip(got) != ansi.Strip(want) {
		t.Fatalf("Render = %q, want %q", got, want)
	}
	multi := Label{NodeBase: NodeBase{Pad: 1}, Text: "a\nbcd"}
	if w, h := multi.Size(); w.Value != 5 || h.Value != 4 {
		t.Fatalf("multi-line padded label = %+v %+v, want 5x4", w, h)
	}
	fixed := Label{NodeBase: NodeBase{W: Fixed(9), H: Grow(1)}, Text: "a"}
	if w, h := fixed.Size(); w != Fixed(9) || h != Grow(1) {
		t.Fatalf("explicit sizes not kept: %+v %+v", w, h)
	}
}

func TestLabelMatchesStyledText(t *testing.T) {
	// A Label paints exactly what a Text holding the role-rendered string
	// paints, so migrating a Text to a Label changes no cells.
	style := TextStyle{Role: TextSubtle}
	label := Compose(Col{Children: []Node{Label{Text: "quiet", Style: style}}}, 12, 2)
	text := Compose(Col{Children: []Node{Text{String: style.Render("quiet")}}}, 12, 2)
	if label.Frame != text.Frame {
		t.Fatalf("label frame %q != text frame %q", label.Frame, text.Frame)
	}
}

// --- Box ---

func TestBoxBorderKindsAndInset(t *testing.T) {
	for _, tc := range []struct {
		name   string
		base   NodeBase
		corner string
		inset  int
	}{
		{"none", NodeBase{}, "", 0},
		{"bordered shorthand", NodeBase{Bordered: true}, "╭", 1},
		{"rounded", NodeBase{Box: Box{Border: BorderRounded}}, "╭", 1},
		{"normal", NodeBase{Box: Box{Border: BorderNormal}}, "┌", 1},
		{"box wins over shorthand", NodeBase{Bordered: true, Box: Box{Border: BorderNormal}}, "┌", 1},
		{"unknown kind draws rounded", NodeBase{Box: Box{Border: BorderKind(42)}}, "╭", 1},
	} {
		root := Col{NodeBase: tc.base, Children: []Node{Text{NodeBase: NodeBase{ID: "in"}, String: "x"}}}
		c := Compose(root, 6, 3)
		if got := c.Rects["in"]; got.X != tc.inset || got.Y != tc.inset {
			t.Errorf("%s: child at %+v, want inset %d", tc.name, got, tc.inset)
		}
		first := plainLines(c.Frame)[0]
		if tc.corner == "" {
			if first != "x" {
				t.Errorf("%s: first line = %q", tc.name, first)
			}
		} else if !strings.HasPrefix(first, tc.corner) {
			t.Errorf("%s: first line = %q, want corner %q", tc.name, first, tc.corner)
		}
	}
}

func TestBoxBorderedShorthandIsByteIdentical(t *testing.T) {
	a := Compose(Col{NodeBase: NodeBase{Bordered: true, Pad: 1}, Children: []Node{Text{String: "x"}}}, 8, 5).Frame
	b := Compose(Col{NodeBase: NodeBase{Box: Box{Border: BorderRounded}, Pad: 1}, Children: []Node{Text{String: "x"}}}, 8, 5).Frame
	if a != b {
		t.Fatalf("Bordered and Box{BorderRounded} differ:\n%q\n%q", a, b)
	}
}

func TestBoxColors(t *testing.T) {
	grey, blue := lipgloss.Color("240"), lipgloss.Color("4")
	r := Rect{W: 4, H: 3}
	got := Text{NodeBase: NodeBase{Box: Box{Border: BorderNormal, BorderColor: grey}}, String: "x"}.Render(r)
	want := lipgloss.NewStyle().Width(4).Height(3).MaxWidth(4).MaxHeight(3).Border(lipgloss.NormalBorder()).BorderForeground(grey).Render("x")
	if got != want {
		t.Fatalf("border color:\n%q\nwant\n%q", got, want)
	}
	got = Text{NodeBase: NodeBase{Box: Box{Background: blue}}, String: "x"}.Render(r)
	want = lipgloss.NewStyle().Width(4).Height(3).MaxWidth(4).MaxHeight(3).Background(blue).Render("x")
	if got != want {
		t.Fatalf("background:\n%q\nwant\n%q", got, want)
	}
}

func TestBoxBackgroundPaintsContainerWithoutInset(t *testing.T) {
	blue := lipgloss.Color("4")
	c := Compose(Col{NodeBase: NodeBase{Box: Box{Background: blue}}, Children: []Node{Text{NodeBase: NodeBase{H: Fixed(1), W: Fixed(1)}, String: "x"}}}, 3, 2)
	// Row 2 holds no child; the container's background must still show.
	if !strings.Contains(strings.Split(c.Frame, "\n")[1], "\x1b[") {
		t.Fatalf("container background not painted under empty rows: %q", c.Frame)
	}
}

// TestBoxBackgroundShowsBehindLeaves: a Box's background is behind its
// content, so a leaf cell without a background of its own keeps the box's
// (a window draws text over the fill; the terminal must match). A leaf's own
// background still wins.
func TestBoxBackgroundShowsBehindLeaves(t *testing.T) {
	bg, own := lipgloss.Color("57"), lipgloss.Color("196")
	root := Col{NodeBase: NodeBase{PadLeft: 1, Box: Box{Background: bg}}, Children: []Node{
		Label{Text: "plain", Style: TextStyle{Fg: lipgloss.Color("252")}},
		Text{String: "ansi " + lipgloss.NewStyle().Bold(true).Render("bold")},
		Label{Text: "own", Style: TextStyle{Bg: own}},
	}}
	buf := uv.NewScreenBuffer(12, 4)
	uv.NewStyledString(Compose(root, 12, 4).Frame).Draw(buf, buf.Bounds())
	want := func(x, y int, c color.Color) {
		t.Helper()
		cell := buf.CellAt(x, y)
		if cell == nil || !reflect.DeepEqual(cell.Style.Bg, c) {
			var got color.Color
			if cell != nil {
				got = cell.Style.Bg
			}
			t.Errorf("cell %d,%d (%q) background %v, want %v", x, y, cell.Content, got, c)
		}
	}
	for y := range 2 {
		for x := range 12 {
			want(x, y, bg) // text and its padding both keep the box's fill
		}
	}
	want(1, 2, own)
	want(11, 3, bg) // below the leaves: the fill itself
}

func TestSpacerBackgroundSources(t *testing.T) {
	blue := lipgloss.Color("4")
	viaMethod := NewSpacer().WithBackground(blue).Render(Rect{W: 3, H: 1})
	viaBox := (&Spacer{NodeBase: NodeBase{Box: Box{Background: blue}}}).Render(Rect{W: 3, H: 1})
	if viaMethod != viaBox || !strings.Contains(viaBox, "\x1b[") {
		t.Fatalf("spacer backgrounds differ: %q vs %q", viaMethod, viaBox)
	}
	var none color.Color
	if plain := (&Spacer{bg: none}).Render(Rect{W: 3, H: 1}); plain != "   " {
		t.Fatalf("plain spacer = %q", plain)
	}
}

// --- Input ---

func TestInputSize(t *testing.T) {
	w, h := Input{}.Size()
	if w != Grow(1) || h != (Size{Kind: SizeContent, Value: 1}) {
		t.Fatalf("Input size = %+v %+v, want Grow(1) x content 1", w, h)
	}
	w, h = Input{NodeBase: NodeBase{W: Fixed(5), Bordered: true}}.Size()
	if w != Fixed(5) || h.Value != 3 {
		t.Fatalf("bordered input size = %+v %+v", w, h)
	}
}

func TestInputRendersValueOrPlaceholder(t *testing.T) {
	if got := ansi.Strip(Input{Value: "Ada"}.Render(Rect{W: 6, H: 1})); got != "Ada   " {
		t.Errorf("value render = %q", got)
	}
	ph := Input{Placeholder: "name"}.Render(Rect{W: 6, H: 1})
	if ansi.Strip(ph) != "name  " || !strings.Contains(ph, "\x1b[") {
		t.Errorf("placeholder render = %q (want subtle-styled)", ph)
	}
	styled := Input{Value: "x", Style: TextStyle{Bold: true}}.Render(Rect{W: 1, H: 1})
	if ansi.Strip(styled) != "x" || !strings.Contains(styled, "\x1b[1m") {
		t.Errorf("styled value = %q", styled)
	}
}

func TestInputScrollsToKeepCaretVisible(t *testing.T) {
	in := Input{Value: "abcdefghij", Caret: 10, Focused: true}
	if got := ansi.Strip(in.Render(Rect{W: 4, H: 1})); got != "hij " {
		t.Errorf("caret at end: %q, want \"hij \" (room for the caret)", got)
	}
	if x, _, ok := Compose(in, 4, 1).Cursor(); !ok || x != 3 {
		t.Errorf("scrolled caret x = %d (%v), want 3 (last cell)", x, ok)
	}
	in.Caret = 2
	if got := ansi.Strip(in.Render(Rect{W: 4, H: 1})); got != "abcd" {
		t.Errorf("caret near start: %q", got)
	}
	// Unfocused fields show the start: no caret to keep visible.
	in.Caret, in.Focused = 10, false
	if got := ansi.Strip(in.Render(Rect{W: 4, H: 1})); got != "abcd" {
		t.Errorf("unfocused: %q", got)
	}
}

func TestInputCaretBecomesFrameCursor(t *testing.T) {
	root := Col{Children: []Node{
		Text{NodeBase: NodeBase{H: Fixed(1)}, String: "header"},
		Row{NodeBase: NodeBase{Gap: 1}, Children: []Node{
			Label{Text: "name:"},
			Input{NodeBase: NodeBase{ID: "name"}, Value: "Ada", Caret: 1, Focused: true},
		}},
	}}
	c := Compose(root, 20, 3)
	x, y, ok := c.Cursor()
	if !ok || x != 7 || y != 1 {
		t.Fatalf("Cursor = %d,%d,%v, want 7,1 (after \"A\")", x, y, ok)
	}
	// Read the painted cell under the cursor: the caret sits before "d".
	if line := []rune(plainLines(c.Frame)[y]); string(line[x]) != "d" {
		t.Fatalf("cell under cursor = %q in %q", string(line[x]), string(line))
	}
	if _, _, ok := Compose(Input{Value: "x"}, 5, 1).Cursor(); ok {
		t.Error("unfocused input produced a cursor")
	}
	if _, _, ok := (Composition{}).Cursor(); ok {
		t.Error("empty composition produced a cursor")
	}
}

func TestInputCaretWideRunesAndClamping(t *testing.T) {
	for _, tc := range []struct {
		value string
		caret int
		wantX int
	}{
		{"日本", len("日"), 2},     // wide rune: two cells
		{"日本", 1, 0},            // mid-rune byte offset: the caret sits before the rune
		{"日本", len("日") + 1, 2}, // mid-rune in the second rune
		{"aé", 2, 1},            // mid-rune after an ASCII byte
		{"ab", 99, 2},           // past the end clamps to the end
		{"ab", -3, 0},           // negative clamps to the start
		{"", 0, 0},
	} {
		in := Input{Value: tc.value, Caret: tc.caret, Focused: true}
		x, _, ok := Compose(in, 10, 1).Cursor()
		if !ok || x != tc.wantX {
			t.Errorf("value %q caret %d: cursor x = %d (%v), want %d", tc.value, tc.caret, x, ok, tc.wantX)
		}
	}
}

func TestInputCaretTopmostFocusedWins(t *testing.T) {
	base := Input{NodeBase: NodeBase{ID: "base"}, Value: "base", Caret: 0, Focused: true}
	modal := Input{NodeBase: NodeBase{ID: "modal", Overlay: true, W: Fixed(6), H: Fixed(1)}, Value: "m", Caret: 1, Focused: true}
	c := Compose(Col{Children: []Node{base, modal}}, 10, 3)
	r := c.Rects["modal"]
	if x, y, ok := c.Cursor(); !ok || x != r.X+1 || y != r.Y {
		t.Fatalf("Cursor = %d,%d,%v, want the overlay input's caret at %d,%d", x, y, ok, r.X+1, r.Y)
	}
}

func TestInputCaretHiddenWhenScrolledOut(t *testing.T) {
	child := Col{Children: []Node{
		Input{NodeBase: NodeBase{H: Fixed(1)}, Value: "a", Focused: true},
		Text{NodeBase: NodeBase{H: Fixed(5)}, String: "x"},
	}}
	if _, _, ok := Compose(Scroll{OffsetY: 2, Child: child}, 5, 2).Cursor(); ok {
		t.Fatal("caret of a scrolled-out input became the cursor")
	}
	if x, y, ok := Compose(Scroll{OffsetY: 0, Child: child}, 5, 2).Cursor(); !ok || x != 0 || y != 0 {
		t.Fatalf("visible input cursor = %d,%d,%v", x, y, ok)
	}
}

func TestInputCaretInsideInset(t *testing.T) {
	in := Input{NodeBase: NodeBase{Bordered: true, PadLeft: 1}, Value: "ab", Caret: 2, Focused: true}
	x, y, ok := Compose(in, 10, 3).Cursor()
	if !ok || x != 4 || y != 1 {
		t.Fatalf("cursor = %d,%d,%v, want 4,1 (border + pad + 2 cells)", x, y, ok)
	}
	if _, _, ok := Compose(Input{NodeBase: NodeBase{Bordered: true}, Focused: true}, 2, 2).Cursor(); ok {
		t.Fatal("input with no inner box produced a cursor")
	}
}

func TestPrimitivesSolveMatchesCompose(t *testing.T) {
	root := Col{NodeBase: NodeBase{Box: Box{Border: BorderNormal}}, Children: []Node{
		Label{NodeBase: NodeBase{ID: "l"}, Text: "t", Style: TextStyle{Role: TextTitle}},
		Input{NodeBase: NodeBase{ID: "i"}, Value: "v", Focused: true},
	}}
	if !reflect.DeepEqual(Compose(root, 10, 5).Rects, Solve(root, 10, 5)) {
		t.Fatal("Solve diverges from Compose")
	}
}
