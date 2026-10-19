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

func rectEnd(origin, size int) int {
	return satAdd(origin, nonNegative(size))
}

func (r Rect) Inset(n int) Rect {
	return r.insetBy(sides{top: n, right: n, bottom: n, left: n})
}

// insetBy shrinks r by a per-side inset.
func (r Rect) insetBy(s sides) Rect {
	w, h := max(r.W, 0), max(r.H, 0)
	left := min(nonNegative(s.left), w)
	right := min(nonNegative(s.right), w-left)
	top := min(nonNegative(s.top), h)
	bottom := min(nonNegative(s.bottom), h-top)
	return Rect{satAdd(r.X, left), satAdd(r.Y, top), w - left - right, h - top - bottom}
}

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < rectEnd(r.X, r.W) && y >= r.Y && y < rectEnd(r.Y, r.H)
}

// Intersect returns the overlapping region of r and s. If they do not overlap,
// it returns a zero-area rect anchored at the clamped corner. It is used to
// clamp a child's solved rect to its parent's inner box, so distribution
// overflow (e.g. two Fixed(8) children in a 10-wide row) can never yield a rect
// that escapes the parent or the frame — hit-testing trusts these rects.
func (r Rect) Intersect(s Rect) Rect {
	x0 := max(r.X, s.X)
	y0 := max(r.Y, s.Y)
	x1 := min(rectEnd(r.X, r.W), rectEnd(s.X, s.W))
	y1 := min(rectEnd(r.Y, r.H), rectEnd(s.Y, s.H))
	// On no overlap, collapse the origin onto the clamped far edge so the empty
	// rect still sits inside s rather than reporting an out-of-bounds corner.
	if x0 > x1 {
		x0 = min(max(x1, s.X), rectEnd(s.X, s.W))
		x1 = x0
	}
	if y0 > y1 {
		y0 = min(max(y1, s.Y), rectEnd(s.Y, s.H))
		y1 = y0
	}
	return Rect{x0, y0, nonNegative(satSub(x1, x0)), nonNegative(satSub(y1, y0))}
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

func Fixed(n int) Size    { return Size{Kind: SizeFixed, Value: max(n, 0)} }
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
	case SizeFixed, SizeContent:
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
	if n.Bordered {
		s.top = satAdd(s.top, 1)
		s.right = satAdd(s.right, 1)
		s.bottom = satAdd(s.bottom, 1)
		s.left = satAdd(s.left, 1)
	}
	return s
}

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
	gap = max(gap, 0)
	avail = nonNegative(avail)
	if n > 1 {
		gaps := n - 1
		if gap > avail/gaps {
			avail = 0
		} else {
			avail -= gap * gaps
		}
	}

	// Collect sizes, sum fixed, sum grow weights.
	sizes := make([]Size, n)
	fixedSum := 0
	growSum := 0.0
	maxGrow := 0.0
	growCount := 0
	for i, c := range children {
		cw, ch := c.Size()
		if horizontal {
			sizes[i] = normalizeSize(cw)
		} else {
			sizes[i] = normalizeSize(ch)
		}
		switch sizes[i].Kind {
		case SizeFixed, SizeContent:
			fixedSum = satAdd(fixedSum, sizes[i].Value)
		case SizeGrow:
			growSum += sizes[i].Weight
			if sizes[i].Weight > maxGrow {
				maxGrow = sizes[i].Weight
			}
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

	fixedBudget := min(fixedSum, avail)
	remaining := avail - fixedBudget
	// A finite but huge collection of finite weights can overflow growSum.
	// Normalize by the largest weight in that case; the resulting denominator
	// is bounded by the number of children and preserves proportions.
	useScaledGrow := math.IsInf(growSum, 0)
	if useScaledGrow {
		growSum = 0
		for _, s := range sizes {
			if s.Kind == SizeGrow && maxGrow > 0 {
				growSum += s.Weight / maxGrow
			}
		}
	}

	result := make([]int, n)
	growUsed := 0
	fixedUsed := 0
	for i, s := range sizes {
		switch s.Kind {
		case SizeFixed, SizeContent:
			claim := s.Value
			if fixedUsed >= fixedBudget {
				claim = 0
			} else {
				claim = min(claim, fixedBudget-fixedUsed)
			}
			result[i] = claim
			fixedUsed += claim
		case SizeGrow:
			w := s.Weight
			if equalGrow {
				w = 1
			}
			if useScaledGrow && maxGrow > 0 {
				w /= maxGrow
			}
			if growSum > 0 && remaining > 0 {
				result[i] = fractionOf(remaining, w/growSum)
			}
			growUsed = min(remaining, satAdd(growUsed, result[i]))
		}
	}

	// Distribute the rounding remainder to Grow children in O(n), rather than
	// decrementing one cell at a time. The latter can take effectively forever
	// when a tiny relative weight rounds every initial share to zero while the
	// available width is MaxInt.
	rem := remaining - growUsed
	if rem > 0 && growCount > 0 {
		per, extra := rem/growCount, rem%growCount
		for i, s := range sizes {
			if s.Kind != SizeGrow {
				continue
			}
			result[i] = satAdd(result[i], per)
			if extra > 0 {
				result[i] = satAdd(result[i], 1)
				extra--
			}
		}
	}
	return result
}

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

// crossAxis returns the cross-axis dimension for a child.
// Fixed pins; everything else stretches to fill.
func crossAxis(s Size, avail int) int {
	s = normalizeSize(s)
	avail = max(avail, 0)
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
	gap = max(gap, 0)
	for i, c := range children {
		cw, ch := c.Size()
		cw, ch = normalizeSize(cw), normalizeSize(ch)
		var m, cr Size
		if horizontal {
			m, cr = cw, ch
		} else {
			m, cr = ch, cw
		}
		if m.Kind == SizeFixed || m.Kind == SizeContent {
			main = satAdd(main, m.Value)
		}
		if m.Kind == SizeGrow {
			mainGrow = true
		}
		if i > 0 {
			main = satAdd(main, gap)
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

func fillRect(content string, r Rect, pad sides, bordered bool) string {
	return fillRectWithBg(content, r, pad, color.Transparent, bordered)
}

func fillRectWithBg(content string, r Rect, pad sides, c color.Color, bordered bool) string {
	if r.W <= 0 || r.H <= 0 {
		return ""
	}

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
	if c != color.Transparent {
		style = style.Background(c)
	}
	if pad.any() {
		style = style.Padding(pad.top, pad.right, pad.bottom, pad.left)
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
	rects := make(map[string]Rect)
	state := walkState{rects: rects, first: make(map[string]Rect)}
	// Match SolveAndCompose: an overlay root is resolved only at its centered
	// layer rect, never once as a viewport-sized base node.
	if !isOverlay(root) {
		walk(root, Rect{0, 0, width, height}, nil, &state)
	}
	// Overlay pass: recurse so overlay descendants get rects too, matching
	// SolveAndCompose's overlay walk for hit-test parity.
	for _, o := range findOverlays(root) {
		mr := centerRect(o, width, height)
		walkOverlay(o, mr, nil, &state)
	}
	return rects, state.diagnostics
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

func isOverlay(n Node) bool {
	o, ok := n.(interface{ IsOverlay() bool })
	return ok && o.IsOverlay()
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
	s = normalizeSize(s)
	viewport = max(viewport, 0)
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
