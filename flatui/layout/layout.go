// Package layout is a flexbox-style layout engine for Flatte.
//
// Every visual thing is a Node — an interface with two methods:
//
//	type Node interface {
//	    Size() (w, h Size)      // constraints for the parent's distribution
//	    Render(r Rect) string   // output at the solved rect
//	}
//
// Concrete types embed NodeBase for shared geometry (W, H, Pad, Gap, Border)
// and add their own fields. Row distributes horizontally, Col vertically.
// Text carries a string. Widgets embed NodeBase and implement both methods.
//
// There is no separate solver pass — containers distribute in their own
// Render via a shared distributeMain helper. Measurement happens naturally
// through Size() calls (each type computes its intrinsic size, querying
// children as needed).
package layout

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// --- Geometry types ---

type Rect struct{ X, Y, W, H int }

func (r Rect) Inset(n int) Rect {
	return Rect{r.X + n, r.Y + n, r.W - 2*n, r.H - 2*n}
}

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

// Intersect returns the overlapping region of r and s. If they do not overlap,
// it returns a zero-area rect anchored at the clamped corner. It is used to
// clamp a child's solved rect to its parent's inner box, so distribution
// overflow (e.g. two Fixed(8) children in a 10-wide row) can never yield a rect
// that escapes the parent or the frame — hit-testing trusts these rects.
func (r Rect) Intersect(s Rect) Rect {
	x0 := max(r.X, s.X)
	y0 := max(r.Y, s.Y)
	x1 := min(r.X+r.W, s.X+s.W)
	y1 := min(r.Y+r.H, s.Y+s.H)
	// On no overlap, collapse the origin onto the clamped far edge so the empty
	// rect still sits inside s rather than reporting an out-of-bounds corner.
	if x0 > x1 {
		x0 = x1
	}
	if y0 > y1 {
		y0 = y1
	}
	return Rect{x0, y0, x1 - x0, y1 - y0}
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

func Fixed(n int) Size    { return Size{Kind: SizeFixed, Value: n} }
func Grow(w float64) Size { return Size{Kind: SizeGrow, Weight: w} }
func Auto() Size          { return Size{Kind: SizeAuto} }

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

	// Chrome, when set on a container, paints the node's own decoration
	// (styled border, background, title) instead of the default Pad/Bordered
	// fill. It receives the node's full solved rect and is painted before
	// (under) the children. It does not change the layout inset: Pad and
	// Bordered still declare the space the chrome occupies, so children are
	// placed inside that inset regardless of what Chrome draws. Leaves ignore
	// it — a leaf's Render owns all of its pixels already.
	Chrome func(r Rect) string
}

func (n NodeBase) Size() (Size, Size) { return n.W, n.H }
func (n NodeBase) GetID() string      { return n.ID }
func (n NodeBase) IsOverlay() bool    { return n.Overlay }

func (n NodeBase) innerInset() int {
	s := n.Pad
	if n.Bordered {
		s++
	}
	return s
}

func (n NodeBase) pad() int                  { return n.Pad }
func (n NodeBase) bordered() bool            { return n.Bordered }
func (n NodeBase) chrome() func(Rect) string { return n.Chrome }

// --- Distribution helpers (shared by Row and Col) ---

// distributeMain computes main-axis sizes for children.
// horizontal=true for Row, false for Col.
func distributeMain(children []Node, horizontal bool, gap, avail int) []int {
	n := len(children)
	if n == 0 {
		return nil
	}
	avail -= gap * (n - 1)
	if avail < 0 {
		avail = 0
	}

	// Collect sizes, sum fixed, sum grow weights.
	sizes := make([]Size, n)
	fixedSum := 0
	growSum := 0.0
	growCount := 0
	for i, c := range children {
		cw, ch := c.Size()
		if horizontal {
			sizes[i] = cw
		} else {
			sizes[i] = ch
		}
		switch sizes[i].Kind {
		case SizeFixed, SizeContent:
			fixedSum += sizes[i].Value
		case SizeGrow:
			growSum += sizes[i].Weight
			growCount++
		}
	}

	// Degenerate input: grow children exist but their weights sum to zero
	// (Grow(0) or a zero-value Size{Kind: SizeGrow}). Treat them as equal
	// weight so the space is shared rather than dropped.
	equalGrow := growCount > 0 && growSum == 0
	if equalGrow {
		growSum = float64(growCount)
	}

	remaining := avail - fixedSum
	if remaining < 0 {
		remaining = 0
	}

	result := make([]int, n)
	growUsed := 0
	for i, s := range sizes {
		switch s.Kind {
		case SizeFixed, SizeContent:
			result[i] = s.Value
		case SizeGrow:
			w := s.Weight
			if equalGrow {
				w = 1
			}
			if growSum > 0 {
				result[i] = int(float64(remaining) * w / growSum)
			}
			growUsed += result[i]
		}
	}

	// Distribute the rounding remainder to Grow children, round-robin
	// left-to-right until it is exhausted (deterministic: earlier children win
	// the extra cell).
	rem := remaining - growUsed
	for rem > 0 {
		progressed := false
		for i, s := range sizes {
			if rem <= 0 {
				break
			}
			if s.Kind == SizeGrow {
				result[i]++
				rem--
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}
	return result
}

// crossAxis returns the cross-axis dimension for a child.
// Fixed pins; everything else stretches to fill.
func crossAxis(s Size, avail int) int {
	if s.Kind == SizeFixed && s.Value < avail {
		return s.Value
	}
	return avail
}

// measureChildren computes the intrinsic main-axis and cross-axis for a
// container with Auto sizing. It also reports whether any child is Grow on the
// main or cross axis, so a container whose only claim on an axis is "grow" can
// itself report Grow(1) instead of collapsing to zero.
func measureChildren(children []Node, horizontal bool, gap int) (main, cross int, mainGrow, crossGrow bool) {
	for i, c := range children {
		cw, ch := c.Size()
		var m, cr Size
		if horizontal {
			m, cr = cw, ch
		} else {
			m, cr = ch, cw
		}
		if m.Kind == SizeFixed || m.Kind == SizeContent {
			main += m.Value
		}
		if m.Kind == SizeGrow {
			mainGrow = true
		}
		if i > 0 {
			main += gap
		}
		if cr.Kind == SizeGrow {
			crossGrow = true
		}
		if (cr.Kind == SizeFixed || cr.Kind == SizeContent) && cr.Value > cross {
			cross = cr.Value
		}
	}
	return
}

// --- Rendering helpers ---

func fillRect(content string, r Rect, pad int, bordered bool) string {
	return fillRectWithBg(content, r, pad, color.Transparent, bordered)
}

func fillRectWithBg(content string, r Rect, pad int, c color.Color, bordered bool) string {
	if r.W <= 0 || r.H <= 0 {
		return ""
	}

	// Lip Gloss Width/Height are *minimum* total-frame sizes; when
	// content+padding+border exceed them, MaxWidth/MaxHeight hard-truncate —
	// eating the bottom/right chrome. Clipping the content to the inner box
	// first means the frame can never overflow, so borders and padding stay
	// intact no matter what the caller hands in.
	inset := pad
	if bordered {
		inset++
	}
	content = clipToBox(content, r.W-2*inset, r.H-2*inset)

	style := lipgloss.NewStyle().Width(r.W).Height(r.H).MaxWidth(r.W).MaxHeight(r.H)
	if c != color.Transparent {
		style = style.Background(c)
	}
	if pad > 0 {
		style = style.Padding(pad)
	}
	if bordered {
		style = style.Border(lipgloss.RoundedBorder())
	}

	return style.Render(content)
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
	rects := make(map[string]Rect)
	walk(root, Rect{0, 0, width, height}, nil, rects)
	// Overlay pass: recurse so overlay descendants get rects too, matching
	// SolveAndCompose's overlay walk for hit-test parity.
	for _, o := range findOverlays(root) {
		mr := centerRect(o, width, height)
		walk(o, mr, nil, rects)
	}
	return rects
}

func mainAvail(inner Rect, horizontal bool) int {
	if horizontal {
		return inner.W
	}
	return inner.H
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

func centerRect(n Node, vpW, vpH int) Rect {
	w, h := n.Size()
	lw := resolveLayerSize(w, vpW)
	lh := resolveLayerSize(h, vpH)
	return Rect{(vpW - lw) / 2, (vpH - lh) / 2, lw, lh}
}

// resolveLayerSize maps an overlay layer's one-axis Size to a concrete cell
// count within the viewport. Fixed and Content are exact sizes clamped to the
// viewport — Content is a measured natural size, so an overlay whose Size()
// reports Content (e.g. a Col of measurable children) is content-sized, not
// stretched. Auto means "no measurable content on this axis" and fills the
// whole viewport. For Grow, Weight carries a meaning specific to overlays:
// Weight>=1 means the full viewport and Weight<1 means that fraction of the
// viewport (see the Size doc for why this dual meaning is intentional).
func resolveLayerSize(s Size, viewport int) int {
	switch s.Kind {
	case SizeFixed, SizeContent:
		if s.Value > viewport {
			return viewport
		}
		return s.Value
	case SizeGrow:
		if s.Weight >= 1 {
			return viewport
		}
		return int(float64(viewport) * s.Weight)
	default: // SizeAuto
		return viewport
	}
}

// --- Small helpers ---

func getID(n Node) string {
	if ider, ok := n.(interface{ GetID() string }); ok {
		return ider.GetID()
	}
	return ""
}

func getGap(n Node) int {
	type gapper interface{ GetGap() int }
	if g, ok := n.(gapper); ok {
		return g.GetGap()
	}
	return 0
}

func getInset(n Node) int {
	type inser interface{ innerInset() int }
	if ins, ok := n.(inser); ok {
		return ins.innerInset()
	}
	return 0
}
