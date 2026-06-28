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
	"strings"

	"charm.land/lipgloss/v2"
)

// --- Geometry types ---

type Rect struct{ X, Y, W, H int }

func (r Rect) Inset(n int) Rect {
	return Rect{r.X + n, r.Y + n, r.W - 2*n, r.H - 2*n}
}

func (r Rect) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}

type SizeKind int

const (
	SizeAuto    SizeKind = iota // stretch on cross-axis, zero on main
	SizeFixed                   // exact cell count, pinned on both axes
	SizeGrow                    // proportional fill of remaining space
	SizeContent                 // measured — Fixed on main axis, stretches on cross
)

type Size struct {
	Kind   SizeKind
	Value  int
	Weight float64
}

func Fixed(n int) Size    { return Size{Kind: SizeFixed, Value: n} }
func Grow(w float64) Size { return Size{Kind: SizeGrow, Weight: w} }
func Auto() Size          { return Size{Kind: SizeAuto} }

// --- Node interface ---

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
		}
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
			if growSum > 0 {
				result[i] = int(float64(remaining) * s.Weight / growSum)
			}
			growUsed += result[i]
		}
	}

	// Distribute remainder left-to-right to Grow children.
	rem := remaining - growUsed
	for i, s := range sizes {
		if rem <= 0 {
			break
		}
		if s.Kind == SizeGrow {
			result[i]++
			rem--
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

// measureChildren computes the intrinsic main-axis and cross-axis for
// a container with Auto sizing.
func measureChildren(children []Node, horizontal bool, gap int) (main, cross int) {
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
		if i > 0 {
			main += gap
		}
		if (cr.Kind == SizeFixed || cr.Kind == SizeContent) && cr.Value > cross {
			cross = cr.Value
		}
	}
	return
}

// --- Rendering helpers ---

func fillRect(content string, r Rect, pad int, bordered bool) string {
	if r.W <= 0 || r.H <= 0 {
		return ""
	}
	if content == "" {
		lines := make([]string, r.H)
		for i := range lines {
			lines[i] = strings.Repeat(" ", r.W)
		}
		return strings.Join(lines, "\n")
	}
	style := lipgloss.NewStyle().Width(r.W).Height(r.H).MaxWidth(r.W).MaxHeight(r.H)
	if pad > 0 {
		style = style.Padding(0, pad, pad, pad)
	}
	if bordered {
		style = style.Border(lipgloss.RoundedBorder())
	}
	return style.Render(content)
}

// Render is the entry point: tree in, composed string out.
func Render(n Node, width, height int) string {
	base := n.Render(Rect{0, 0, width, height})

	// Second pass: overlay nodes composited on top.
	for _, o := range findOverlays(n) {
		mr := centerRect(o, width, height)
		base = compositeAt(base, o.Render(mr), mr.X, mr.Y)
	}
	return base
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

func Solve(root Node, width, height int) map[string]Rect {
	rects := make(map[string]Rect)
	solveNode(root, Rect{0, 0, width, height}, rects)
	// Overlay pass
	for _, o := range findOverlays(root) {
		mr := centerRect(o, width, height)
		if id := getID(o); id != "" {
			rects[id] = mr
		}
	}
	return rects
}

func solveNode(n Node, r Rect, out map[string]Rect) {
	if id := getID(n); id != "" {
		out[id] = r
	}
	children := nonOverlayChildren(getChildren(n))
	if len(children) == 0 {
		return
	}
	container := n.(childContainer)
	gap := getGap(n)
	horizontal := container.IsHorizontal()
	inset := getInset(n)
	inner := r.Inset(inset)
	mainSizes := distributeMain(children, horizontal, gap, mainAvail(inner, horizontal))

	pos := inner.X
	if !horizontal {
		pos = inner.Y
	}
	for i, c := range children {
		cw, ch := c.Size()
		var cr Rect
		if horizontal {
			cross := crossAxis(ch, inner.H)
			cr = Rect{pos, inner.Y, mainSizes[i], cross}
		} else {
			cross := crossAxis(cw, inner.W)
			cr = Rect{inner.X, pos, cross, mainSizes[i]}
		}
		solveNode(c, cr, out)
		pos += mainSizes[i] + gap
	}
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

func resolveLayerSize(s Size, viewport int) int {
	switch s.Kind {
	case SizeFixed:
		if s.Value > viewport {
			return viewport
		}
		return s.Value
	case SizeGrow:
		if s.Weight >= 1 {
			return viewport
		}
		return int(float64(viewport) * s.Weight)
	default:
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
