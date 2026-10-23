// Package layout is a flexbox-style layout and composition engine for Flatte.
//
// Every visual thing is a Node — an interface with two methods:
//
//	type Node interface {
//	    Size() (w, h Size)      // constraints for the parent's distribution
//	    Render(r Rect) string   // output at the solved rect
//	}
//
// Concrete types embed NodeBase for shared geometry (W, H, Pad, Gap, Bordered)
// and add their own fields. Row distributes horizontally, Col vertically.
// Text carries a string. Widgets may expose Layout nodes; the primitive nodes
// implement both methods.
//
// Solve returns geometry without painting. SolveAndCompose performs one shared
// top-down walk that distributes rectangles, records IDs, and paints leaves
// into a clipped cell buffer. Row and Col use that same compositor when they
// are rendered as nested nodes, so layout geometry and pixels cannot drift.
package layout

import (
	"image/color"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// --- Geometry types ---

type Rect struct{ X, Y, W, H int }

const maxInt = int(^uint(0) >> 1)

// saturating arithmetic keeps malformed, caller-authored geometry inside the
// representable int domain. Layout dimensions are counts, so negative widths
// and heights are treated as zero at the boundary.
func satAdd(a, b int) int {
	if b > 0 && a > maxInt-b {
		return maxInt
	}
	if b < 0 && a < -maxInt-1-b {
		return -maxInt - 1
	}
	return a + b
}

func satSub(a, b int) int {
	if b > 0 && a < -maxInt-1+b {
		return -maxInt - 1
	}
	if b < 0 && a > maxInt+b {
		return maxInt
	}
	return a - b
}

func nonNegative(n int) int {
	if n < 0 {
		return 0
	}
	return n
}

func (r Rect) Inset(n int) Rect {
	return r.insetBy(sides{top: n, right: n, bottom: n, left: n})
}

// insetBy shrinks r by a per-side inset.
func (r Rect) insetBy(s sides) Rect {
	return Rect(insetOf[int](cells{}, rect[int](r), sidesOf[int]{s.top, s.right, s.bottom, s.left}))
}

func (r Rect) Contains(x, y int) bool { return containsOf[int](cells{}, rect[int](r), x, y) }

// Intersect returns the overlapping region of r and s. If they do not overlap,
// it returns a zero-area rect anchored at the clamped corner. It is used to
// clamp a child's solved rect to its parent's inner box, so distribution
// overflow (e.g. two Fixed(8) children in a 10-wide row) can never yield a rect
// that escapes the parent or the frame — hit-testing trusts these rects.
func (r Rect) Intersect(s Rect) Rect {
	return Rect(intersectOf[int](cells{}, rect[int](r), rect[int](s)))
}

type SizeKind int

const (
	// SizeAuto is the default "no explicit claim" input. If a node can measure
	// natural content on an auto axis, its Size method should return
	// SizeContent. If it cannot, auto contributes zero on the parent's main
	// axis and stretches on the cross axis.
	SizeAuto SizeKind = iota

	// SizeFixed is an exact caller-specified cell count.
	SizeFixed

	// SizeGrow takes a weighted share of the parent's remaining main-axis
	// space after fixed/content children are allocated.
	SizeGrow

	// SizeContent is a measured natural size, not usually a caller-authored
	// mode. Text and containers return it when an auto axis can be measured
	// from content or children. It behaves like fixed on the parent's main
	// axis, while still stretching on the cross axis.
	SizeContent

	// SizePoints is an exact length in points, for layouts designed for a
	// window. A GUI lays it out exactly; a terminal reads it at a nominal
	// cell of PointsPerColumn × PointsPerLine, rounding up. Otherwise it
	// behaves like SizeFixed.
	SizePoints
)

// PointsPerColumn and PointsPerLine are the nominal terminal cell a
// terminal reads Points sizes at: the 13pt macOS monospace cell, rounded.
const (
	PointsPerColumn = 8
	PointsPerLine   = 16
)

// Size is a one-axis layout constraint. Callers normally set Auto, Fixed, or
// Grow through the constructors below. SizeContent is produced by Size methods
// after measuring natural content, preserving the distinction between "caller
// forced this size" (Fixed) and "this is the content's natural size" (Content).
//
// Weight is normally the proportional share a Grow child takes of a container's
// leftover main-axis space. For overlays it has a second, intentional meaning
// (see resolveLayerSize): a Grow overlay with Weight>=1 fills the whole
// viewport, while Weight<1 sizes to that fraction of the viewport. This dual
// use lets a caller express both "modal that is 60% of the screen"
// (Grow(0.6)) and "full-screen layer" (Grow(1)) without a separate field.
type Size struct {
	Kind   SizeKind
	Value  int
	Weight float64
}

func Fixed(n int) Size    { return Size{Kind: SizeFixed, Value: max(n, 0)} }
func Points(n int) Size   { return Size{Kind: SizePoints, Value: max(n, 0)} }
func Grow(w float64) Size { return normalizeSize(Size{Kind: SizeGrow, Weight: w}) }
func Auto() Size          { return Size{Kind: SizeAuto} }

// normalizeSize gives malformed public Size values a deterministic production
// fallback. Fixed/Content counts cannot be negative. Grow weights must be
// finite and non-negative; invalid weights become zero and participate in the
// existing equal-share fallback when every grow weight is zero. Unknown kinds
// behave as Auto. Consumers normalize as well as constructors because callers
// can author Size values directly.
func normalizeSize(s Size) Size {
	switch s.Kind {
	case SizeAuto:
		return s
	case SizeFixed, SizeContent, SizePoints:
		s.Value = max(s.Value, 0)
		return s
	case SizeGrow:
		if s.Weight < 0 || math.IsNaN(s.Weight) || math.IsInf(s.Weight, 0) {
			s.Weight = 0
		}
		return s
	default:
		return Size{Kind: SizeAuto}
	}
}

// Node is the primitive layout contract. Implementations should keep the two
// methods separate:
//
//   - Size declares layout intent for each axis. It may return caller-authored
//     constraints from NodeBase (Auto, Fixed, Grow), or measured natural
//     content as SizeContent. It must not depend on a final Rect, because the
//     parent calls Size before it knows the child's allocation.
//
//   - Render draws the node inside the already-solved Rect. It should fill,
//     clip, wrap, border, or join content to fit that rect, but it should not
//     choose its own position or size in the parent.
//
// In other words: Size answers "what space do I need or want?", while Render
// answers "given this exact space, what string do I produce?".
//
// Note on cross-axis geometry: a child whose cross-axis Size is not Fixed
// stretches to fill the parent's cross axis (SizeContent included), so the
// rect recorded for a one-line ID'd Text in a Row spans the row's full
// height. That is correct for painting and coarse hit-testing; pin the
// cross axis with Fixed when a hit target must be exactly content-sized.
type Node interface {
	Size() (w, h Size)
	Render(r Rect) string
}

// NodeBase provides shared geometry for all concrete node types.
// Embed it and override Size/Render as needed.
type NodeBase struct {
	W, H     Size
	Pad      int
	Gap      int
	Bordered bool
	Overlay  bool
	ID       string

	// PadTop/PadRight/PadBottom/PadLeft add per-side padding on top of the
	// symmetric Pad (and inside the border, when Bordered). Extracted after
	// two panes needed asymmetric insets: horizontal-only padding and
	// "no top pad, one-row bottom pad" cannot be said with Pad alone.
	PadTop, PadRight, PadBottom, PadLeft int

	// Chrome, when set on a container, paints the node's own decoration
	// (styled border, background, title) instead of the default Pad/Box
	// fill. It receives the node's full solved rect and is painted before
	// (under) the children. It does not change the layout inset: Pad and
	// the border still declare the space the chrome occupies, so children
	// are placed inside that inset regardless of what Chrome draws. Leaves
	// ignore it — a leaf's Render owns all of its pixels already. Chrome is
	// terminal-only; prefer Box where it can say what is needed.
	Chrome func(r Rect) string

	// Box is the node's semantic decoration: border kind and color, and
	// background. A terminal draws it with box-drawing cells, a GUI as a
	// stroked and filled rect. A border takes one cell of inset per side,
	// exactly like Bordered (which is shorthand for Box{Border:
	// BorderRounded}; Box.Border wins when both are set).
	Box Box

	// Tooltip is help text a GUI shows while the pointer rests on the node
	// (the innermost node with one); the terminal ignores it.
	Tooltip string

	// Anchor places an Overlay next to the node with this ID instead of
	// centered on the frame: just below it and left-aligned to it, above
	// it when there is no room below, kept inside the frame — for a
	// dropdown or a context menu. An anchor that is not laid out (or an
	// empty one) centers the overlay as usual.
	Anchor string
}

func (n NodeBase) tooltip() string { return n.Tooltip }

type tooltipNode interface{ tooltip() string }

// BorderKind is the line style of a Box border.
type BorderKind int

const (
	BorderNone BorderKind = iota
	BorderRounded
	BorderNormal
)

// Box is a node's decoration. Nil colors mean the backend's default (no
// background; the terminal's default border color). Unknown border kinds
// draw as BorderRounded. Background is behind the node's content: leaf cells
// without a background of their own show it. Selected marks the node as the
// current selection: a GUI highlights it, while the terminal adds nothing
// (the app's markers and styles show selection there). Round asks a GUI to
// round a background's corners (a pill when the box is one line tall); the
// terminal cannot, and draws it square. Title labels a border: the terminal
// writes " Title " in TitleStyle into the top rule after its first rule
// glyph (cut to fit), and a GUI sets it in the top stroke, which breaks
// around it.
type Box struct {
	Border      BorderKind
	BorderColor color.Color
	Background  color.Color
	Selected    bool
	Round       bool
	Title       string
	TitleStyle  TextStyle
}

// box resolves the Bordered shorthand into the effective Box.
func (n NodeBase) box() Box {
	b := n.Box
	if b.Border == BorderNone && n.Bordered {
		b.Border = BorderRounded
	}
	return b
}

func (b Box) hasBorder() bool { return b.Border != BorderNone }

func (b Box) hasBackground() bool { return b.Background != nil && b.Background != color.Transparent }

func (b Box) lipglossBorder() lipgloss.Border {
	if b.Border == BorderNormal {
		return lipgloss.NormalBorder()
	}
	return lipgloss.RoundedBorder()
}

func (n NodeBase) Size() (Size, Size) { return n.W, n.H }
func (n NodeBase) GetID() string      { return n.ID }
func (n NodeBase) IsOverlay() bool    { return n.Overlay }

// sides is a per-side cell count (padding or full inset), clockwise from top.
type sides struct{ top, right, bottom, left int }

func (s sides) any() bool  { return s.top > 0 || s.right > 0 || s.bottom > 0 || s.left > 0 }
func (s sides) horiz() int { return satAdd(nonNegative(s.left), nonNegative(s.right)) }
func (s sides) vert() int  { return satAdd(nonNegative(s.top), nonNegative(s.bottom)) }

// clampTo constrains per-side claims to a concrete box without allowing the
// second side to push the first outside it. The order matches Rect.insetBy:
// left/top win when malformed callers claim more than the entire box.
func (s sides) clampTo(width, height int) sides {
	width, height = nonNegative(width), nonNegative(height)
	left := min(nonNegative(s.left), width)
	right := min(nonNegative(s.right), width-left)
	top := min(nonNegative(s.top), height)
	bottom := min(nonNegative(s.bottom), height-top)
	return sides{top: top, right: right, bottom: bottom, left: left}
}

// padSides is the node's padding per side: symmetric Pad plus the per-side
// extras (border excluded).
func (n NodeBase) padSides() sides {
	pad := max(n.Pad, 0)
	return sides{
		top:    satAdd(pad, nonNegative(n.PadTop)),
		right:  satAdd(pad, nonNegative(n.PadRight)),
		bottom: satAdd(pad, nonNegative(n.PadBottom)),
		left:   satAdd(pad, nonNegative(n.PadLeft)),
	}
}

// insetSides is the full per-side layout inset: padding plus the border cell.
func (n NodeBase) insetSides() sides {
	s := n.padSides()
	if n.box().hasBorder() {
		s.top = satAdd(s.top, 1)
		s.right = satAdd(s.right, 1)
		s.bottom = satAdd(s.bottom, 1)
		s.left = satAdd(s.left, 1)
	}
	return s
}

func (n NodeBase) chrome() func(Rect) string { return n.Chrome }

// fractionOf returns floor(total*ratio), clamped to total. Keeping the
// conversion behind an explicit cap avoids converting 2^wordsize to a
// negative int when float64 rounds MaxInt up to the next power of two.
func fractionOf(total int, ratio float64) int {
	if total <= 0 || ratio <= 0 || math.IsNaN(ratio) {
		return 0
	}
	if ratio >= 1 || math.IsInf(ratio, 1) {
		return total
	}
	v := float64(total) * ratio
	if v >= float64(total) {
		return total
	}
	return int(v)
}

// --- Rendering helpers ---

// fillBox renders content into r inside pad and box: the terminal painting
// of a node's Box, padding, and content.
func fillBox(content string, r Rect, pad sides, box Box) string {
	if r.W <= 0 || r.H <= 0 {
		return ""
	}
	bordered := box.hasBorder()

	// Lip Gloss Width/Height are *minimum* total-frame sizes; when
	// content+padding+border exceed them, MaxWidth/MaxHeight hard-truncate —
	// eating the bottom/right chrome. Clipping the content to the inner box
	// first means the frame can never overflow, so borders and padding stay
	// intact no matter what the caller hands in.
	borderWidth, borderHeight := 0, 0
	if bordered {
		borderWidth = min(2, r.W)
		borderHeight = min(2, r.H)
	}
	pad = pad.clampTo(r.W-borderWidth, r.H-borderHeight)
	inset := pad
	if bordered {
		inset.top = satAdd(inset.top, 1)
		inset.right = satAdd(inset.right, 1)
		inset.bottom = satAdd(inset.bottom, 1)
		inset.left = satAdd(inset.left, 1)
	}
	content = clipToBox(content, r.W-inset.horiz(), r.H-inset.vert())

	style := lipgloss.NewStyle().Width(r.W).Height(r.H).MaxWidth(r.W).MaxHeight(r.H)
	if box.hasBackground() {
		style = style.Background(box.Background)
	}
	if pad.any() {
		style = style.Padding(pad.top, pad.right, pad.bottom, pad.left)
	}
	if bordered {
		style = style.Border(box.lipglossBorder())
		if box.BorderColor != nil {
			style = style.BorderForeground(box.BorderColor)
		}
	}

	out := style.Render(content)
	if bordered && box.Title != "" {
		out = titled(out, r.W, box)
	}
	return out
}

// titled writes box's title into the top rule of a bordered block w cells
// wide, after the corner and one rule glyph; a block too narrow for a
// title character keeps its rule.
func titled(block string, w int, box Box) string {
	avail := w - 3 // corner, rule glyph, ..., corner
	if avail < 1 {
		return block
	}
	title := box.TitleStyle.Render(" " + box.Title + " ")
	if lipgloss.Width(title) > avail {
		title = ansi.Truncate(title, avail, "")
	}
	b := box.lipglossBorder()
	rule := lipgloss.NewStyle()
	if box.BorderColor != nil {
		rule = rule.Foreground(box.BorderColor)
	}
	top := rule.Render(b.TopLeft+b.Top) + title + rule.Render(strings.Repeat(b.Top, avail-lipgloss.Width(title))+b.TopRight)
	if i := strings.IndexByte(block, '\n'); i >= 0 {
		return top + block[i:]
	}
	return top
}

// clipToBox truncates content to at most h lines of w visible columns.
func clipToBox(s string, w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > h {
		lines = lines[:h]
	}
	for i, line := range lines {
		if lipgloss.Width(line) > w {
			lines[i] = ansi.Truncate(line, w, "")
		}
	}
	return strings.Join(lines, "\n")
}

// --- Container interfaces for introspection (Solve, overlay pass) ---

type childContainer interface {
	GetChildren() []Node
	IsHorizontal() bool
}

func getChildren(n Node) []Node {
	if c, ok := n.(childContainer); ok {
		return c.GetChildren()
	}
	return nil
}

// --- Solve (for hit-testing) ---

// Solve returns the id→rect geometry for a tree without painting. It is the
// geometry-only mode of the shared walk (buf nil), so its rects are identical
// to the ones SolveAndCompose records while composing.
func Solve(root Node, width, height int) map[string]Rect {
	rects, _ := SolveWithDiagnostics(root, width, height)
	return rects
}

// SolveWithDiagnostics returns geometry and reports repeated non-empty IDs in
// deterministic traversal order. Duplicate IDs retain the existing last-node-
// wins map behavior; empty IDs are intentionally ignored.
func SolveWithDiagnostics(root Node, width, height int) (map[string]Rect, []DuplicateIDDiagnostic) {
	if width <= 0 || height <= 0 {
		return map[string]Rect{}, nil
	}
	s := solveTree[int](cells{}, root, width, height, nil)
	return cellRects(s), cellDiagnostics(s)
}

// --- Overlay helpers ---

func findOverlays(n Node) []Node {
	var result []Node
	if o, ok := n.(interface{ IsOverlay() bool }); ok && o.IsOverlay() {
		result = append(result, n)
	}
	for _, c := range getChildren(n) {
		result = append(result, findOverlays(c)...)
	}
	return result
}

func isOverlay(n Node) bool {
	o, ok := n.(interface{ IsOverlay() bool })
	return ok && o.IsOverlay()
}

// --- Small helpers ---

func getAnchor(n Node) string {
	if a, ok := n.(interface{ anchor() string }); ok {
		return a.anchor()
	}
	return ""
}

func (n NodeBase) anchor() string { return n.Anchor }

func getID(n Node) string {
	if ider, ok := n.(interface{ GetID() string }); ok {
		return ider.GetID()
	}
	return ""
}

func getAlign(n Node) Align {
	type aligner interface{ GetAlign() Align }
	if a, ok := n.(aligner); ok {
		return a.GetAlign()
	}
	return AlignStretch
}

func getGap(n Node) int {
	type gapper interface{ GetGap() int }
	if g, ok := n.(gapper); ok {
		return max(g.GetGap(), 0)
	}
	return 0
}

func getInsetSides(n Node) sides {
	type inser interface{ insetSides() sides }
	if ins, ok := n.(inser); ok {
		return ins.insetSides()
	}
	return sides{}
}
