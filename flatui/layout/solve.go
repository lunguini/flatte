package layout

import "math"

// The solver is generic over its length unit so the terminal (int cells) and
// a GUI (float64 points) share one implementation of the distribution,
// clipping, and hit-testing rules and cannot drift. The cell instantiation is
// what Compose and Solve run; it is pinned to the historical behavior by the
// solver corpus test.

// unit is a solver length: terminal cells or GUI points.
type unit interface{ ~int | ~float64 }

// metric turns the layout vocabulary into lengths (design units: sizes are
// columns and lines, Pad and Gap are spacing steps) and supplies the
// arithmetic, which saturates for cells and is exact for points.
type metric[T unit] interface {
	cols(n int) T  // a column count (widths)
	lines(n int) T // a line count (heights)
	steps(n int) T // spacing steps (Pad, Gap)
	// points is a Points size: exact in points, rounded up to cells.
	points(n int, horizontal bool) T
	border() T // one border edge

	add(a, b T) T
	sub(a, b T) T
	times(a T, k int) T
	share(total T, ratio float64) T
	half(a T) T
	// spread splits a distribution remainder over n grow children: each
	// gets per, and the first extra get one more unit.
	spread(rem T, n int) (per T, extra int)
	one() T

	// leaf reports a leaf's own size claims (not a known container's) when
	// laid out width wide; width < 0 means the width is not known yet. Only
	// a wrapping Label's height depends on it.
	leaf(n Node, width T) (w, h sized[T])
}

// noWidth is the width passed when measuring before a width is known.
func noWidth[T unit]() T { return T(-1) }

// sized is a Size in solver units.
type sized[T unit] struct {
	Kind   SizeKind
	Value  T
	Weight float64
}

func toSized[T unit](m metric[T], s Size, horizontal bool) sized[T] {
	if s.Kind == SizePoints {
		return sized[T]{Kind: SizeFixed, Value: m.points(max(s.Value, 0), horizontal)}
	}
	v := m.lines(s.Value)
	if horizontal {
		v = m.cols(s.Value)
	}
	return sized[T]{Kind: s.Kind, Value: v, Weight: s.Weight}
}

func fromSized(s sized[int]) Size { return Size{Kind: s.Kind, Value: s.Value, Weight: s.Weight} }

// normalized gives malformed sizes the same deterministic fallbacks as
// normalizeSize (see there).
func (s sized[T]) normalized() sized[T] {
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
		return sized[T]{Kind: SizeAuto}
	}
}

func growSized[T unit]() sized[T] { return sized[T]{Kind: SizeGrow, Weight: 1} }

// --- cell metric ---

// cells is the terminal metric: every unit is one cell, and arithmetic
// saturates so malformed caller geometry stays representable.
type cells struct{}

func (cells) cols(n int) int  { return n }
func (cells) lines(n int) int { return n }
func (cells) steps(n int) int { return n }
func (cells) points(n int, horizontal bool) int {
	per := PointsPerLine
	if horizontal {
		per = PointsPerColumn
	}
	return (n + per - 1) / per
}
func (cells) border() int { return 1 }
func (cells) add(a, b int) int {
	return satAdd(a, b)
}
func (cells) sub(a, b int) int { return satSub(a, b) }
func (cells) times(a, k int) int {
	if k <= 0 || a <= 0 {
		return 0
	}
	if a > maxInt/k {
		return maxInt
	}
	return a * k
}
func (cells) share(total int, ratio float64) int { return fractionOf(total, ratio) }
func (cells) half(a int) int                     { return a / 2 }
func (cells) spread(rem, n int) (int, int)       { return rem / n, rem % n }
func (cells) one() int                           { return 1 }
func (cells) leaf(n Node, width int) (w, h sized[int]) {
	w, h = sizedOf(cells{}, n)
	if l, ok := wrapping(n); ok {
		h = toSized[int](cells{}, l.wrapHeight(width), false)
	}
	return w, h
}

// wraps reports whether n's height can depend on its width: it is, or holds
// in its layout, a wrapping Label. Only such subtrees are measured again
// once a width is known, so layouts without wrapping cost what they did.
func wraps(n Node) bool {
	if _, ok := wrapping(n); ok {
		return true
	}
	if sc, ok := n.(scroller); ok {
		child, _ := sc.scroll()
		return child != nil && wraps(child)
	}
	if c, ok := n.(childContainer); ok {
		return anyWraps(c.GetChildren())
	}
	return false
}

// anyWraps reports whether a non-overlay child wraps (see wraps).
func anyWraps(children []Node) bool {
	for _, c := range children {
		if !isOverlay(c) && wraps(c) {
			return true
		}
	}
	return false
}

// wrapping reports whether n is a Label that wraps.
func wrapping(n Node) (Label, bool) {
	switch l := n.(type) {
	case Label:
		return l, l.Wrap
	case *Label:
		if l != nil {
			return *l, l.Wrap
		}
	}
	return Label{}, false
}
func sizedOf[T unit](m metric[T], n Node) (w, h sized[T]) {
	sw, sh := n.Size()
	return toSized(m, sw, true), toSized(m, sh, false)
}

// --- geometry in solver units ---

type rect[T unit] struct{ X, Y, W, H T }

type sidesOf[T unit] struct{ top, right, bottom, left T }

func (s sidesOf[T]) any() bool { return s.top > 0 || s.right > 0 || s.bottom > 0 || s.left > 0 }

func horizOf[T unit](m metric[T], s sidesOf[T]) T {
	return m.add(max(s.left, 0), max(s.right, 0))
}

func vertOf[T unit](m metric[T], s sidesOf[T]) T {
	return m.add(max(s.top, 0), max(s.bottom, 0))
}

func endOf[T unit](m metric[T], origin, size T) T { return m.add(origin, max(size, 0)) }

func insetOf[T unit](m metric[T], r rect[T], s sidesOf[T]) rect[T] {
	w, h := max(r.W, 0), max(r.H, 0)
	left := min(max(s.left, 0), w)
	right := min(max(s.right, 0), w-left)
	top := min(max(s.top, 0), h)
	bottom := min(max(s.bottom, 0), h-top)
	return rect[T]{m.add(r.X, left), m.add(r.Y, top), w - left - right, h - top - bottom}
}

func containsOf[T unit](m metric[T], r rect[T], x, y T) bool {
	return x >= r.X && x < endOf(m, r.X, r.W) && y >= r.Y && y < endOf(m, r.Y, r.H)
}

// intersectOf is Rect.Intersect in solver units (see there for the
// no-overlap convention).
func intersectOf[T unit](m metric[T], r, s rect[T]) rect[T] {
	x0 := max(r.X, s.X)
	y0 := max(r.Y, s.Y)
	x1 := min(endOf(m, r.X, r.W), endOf(m, s.X, s.W))
	y1 := min(endOf(m, r.Y, r.H), endOf(m, s.Y, s.H))
	if x0 > x1 {
		x0 = min(max(x1, s.X), endOf(m, s.X, s.W))
		x1 = x0
	}
	if y0 > y1 {
		y0 = min(max(y1, s.Y), endOf(m, s.Y, s.H))
		y1 = y0
	}
	return rect[T]{x0, y0, max(m.sub(x1, x0), 0), max(m.sub(y1, y0), 0)}
}

// --- node introspection in solver units ---

// insetsOf is a node's full per-side inset: padding steps plus the border.
func insetsOf[T unit](m metric[T], n Node) sidesOf[T] {
	return insetsFrom(m, getPadSides(n), getBox(n))
}

func insetsBase[T unit](m metric[T], nb NodeBase) sidesOf[T] {
	return insetsFrom(m, nb.padSides(), nb.box())
}

func insetsFrom[T unit](m metric[T], p sides, box Box) sidesOf[T] {
	s := sidesOf[T]{m.steps(p.top), m.steps(p.right), m.steps(p.bottom), m.steps(p.left)}
	if box.hasBorder() {
		b := m.border()
		s = sidesOf[T]{m.add(s.top, b), m.add(s.right, b), m.add(s.bottom, b), m.add(s.left, b)}
	}
	return s
}

// intrinsic is a node's size claims in solver units before its width is
// known. Known containers are measured here, in the metric's units; every
// other node reports through the metric's leaf rule.
func intrinsic[T unit](m metric[T], n Node) (w, h sized[T]) { return measure(m, n, noWidth[T]()) }

// measure is intrinsic for a node laid out width wide (width < 0: not
// known): heights that depend on width — wrapped text — are measured at it.
func measure[T unit](m metric[T], n Node, width T) (w, h sized[T]) {
	switch c := n.(type) {
	case Row:
		return measureContainer(m, c.NodeBase, c.Children, true, width)
	case *Row:
		if c != nil {
			return measureContainer(m, c.NodeBase, c.Children, true, width)
		}
	case Col:
		return measureContainer(m, c.NodeBase, c.Children, false, width)
	case *Col:
		if c != nil {
			return measureContainer(m, c.NodeBase, c.Children, false, width)
		}
	case Scroll:
		return measureScroll(m, c)
	case *Scroll:
		if c != nil {
			return measureScroll(m, *c)
		}
	}
	return m.leaf(n, width)
}

// measureContainer is Row/Col sizing: on an Auto axis, prefer measured
// content; if there is none but a child grows on that axis, grow.
func measureContainer[T unit](m metric[T], nb NodeBase, children []Node, horizontal bool, width T) (w, h sized[T]) {
	w, h = toSized(m, nb.W, true), toSized(m, nb.H, false)
	if w.Kind != SizeAuto && h.Kind != SizeAuto {
		return w, h
	}
	inset := insetsBase(m, nb)
	if w.Kind == SizeFixed {
		width = w.Value
	}
	if width >= 0 {
		width = max(m.sub(width, horizOf(m, inset)), 0)
	}
	mainSize, crossSize, mainGrow, crossGrow := measureChildrenOf(m, children, horizontal, m.steps(max(nb.Gap, 0)), width)
	mainInset, crossInset := horizOf(m, inset), vertOf(m, inset)
	mainAxis, crossAxis := &w, &h
	if !horizontal {
		mainInset, crossInset = crossInset, mainInset
		mainAxis, crossAxis = &h, &w
	}
	if mainAxis.Kind == SizeAuto {
		if mainSize > 0 {
			*mainAxis = sized[T]{Kind: SizeContent, Value: m.add(mainSize, mainInset)}
		} else if mainGrow {
			*mainAxis = growSized[T]()
		}
	}
	if crossAxis.Kind == SizeAuto {
		if crossSize > 0 {
			*crossAxis = sized[T]{Kind: SizeContent, Value: m.add(crossSize, crossInset)}
		} else if crossGrow {
			*crossAxis = growSized[T]()
		}
	}
	return w, h
}

// measureChildrenOf sums main-axis claims (with gaps) and takes the largest
// cross-axis claim, reporting whether any child grows on either axis. With
// a known inner width, heights are measured at the width each child gets:
// a Col's children get its width, a Row's the widths it distributes.
func measureChildrenOf[T unit](m metric[T], children []Node, horizontal bool, gap, width T) (main, cross T, mainGrow, crossGrow bool) {
	// Overlays are laid out on their own layer: they do not size their
	// parent (as distribution leaves them out too).
	children = nonOverlayChildren(children)
	gap = max(gap, 0)
	var rowWidths []T
	if horizontal && width >= 0 && anyWraps(children) {
		rowWidths = distribute(m, children, true, gap, width, noWidth[T]())
	}
	for i, c := range children {
		cw, ch := intrinsic(m, c)
		switch {
		case rowWidths != nil:
			_, ch = measure(m, c, rowWidths[i])
		case !horizontal && width >= 0 && wraps(c):
			_, ch = measure(m, c, crossOf(cw, width))
		}
		mn, cr := cw.normalized(), ch.normalized()
		if !horizontal {
			mn, cr = cr, mn
		}
		if mn.Kind == SizeFixed || mn.Kind == SizeContent {
			main = m.add(main, mn.Value)
		}
		if mn.Kind == SizeGrow {
			mainGrow = true
		}
		if i > 0 {
			main = m.add(main, gap)
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

// measureScroll: an Auto width measures the child's; an Auto height grows
// (a Scroll exists to be smaller than its content).
func measureScroll[T unit](m metric[T], s Scroll) (w, h sized[T]) {
	w, h = toSized(m, s.W, true), toSized(m, s.H, false)
	if w.Kind == SizeAuto && s.Child != nil && !isOverlay(s.Child) {
		cw, _ := intrinsic(m, s.Child)
		switch cw = cw.normalized(); cw.Kind {
		case SizeFixed, SizeContent:
			w = sized[T]{Kind: SizeContent, Value: m.add(cw.Value, horizOf(m, insetsBase(m, s.NodeBase)))}
		case SizeGrow:
			w = growSized[T]()
		}
	}
	if h.Kind == SizeAuto {
		h = growSized[T]()
	}
	return w, h
}

// --- distribution ---

// distribute computes main-axis sizes for children in avail, with gap
// between them: fixed and content claims first (in order, until avail runs
// out), then grow children share the remainder by weight.
func distribute[T unit](m metric[T], children []Node, horizontal bool, gap, avail, cross T) []T {
	n := len(children)
	if n == 0 {
		return nil
	}
	gap, avail = max(gap, 0), max(avail, 0)
	if n > 1 {
		if total := m.times(gap, n-1); total > avail {
			avail = 0
		} else {
			avail -= total
		}
	}

	sizes := make([]sized[T], n)
	var fixedSum T
	growSum, maxGrow := 0.0, 0.0
	growCount := 0
	for i, c := range children {
		cw, ch := intrinsic(m, c)
		if horizontal {
			sizes[i] = cw.normalized()
		} else {
			if cross >= 0 && wraps(c) {
				_, ch = measure(m, c, crossOf(cw, cross))
			}
			sizes[i] = ch.normalized()
		}
		switch sizes[i].Kind {
		case SizeFixed, SizeContent:
			fixedSum = m.add(fixedSum, sizes[i].Value)
		case SizeGrow:
			growSum += sizes[i].Weight
			maxGrow = max(maxGrow, sizes[i].Weight)
			growCount++
		}
	}

	// Grow children whose weights sum to zero share equally.
	equalGrow := growCount > 0 && growSum == 0
	if equalGrow {
		growSum = float64(growCount)
	}
	fixedBudget := min(fixedSum, avail)
	remaining := avail - fixedBudget
	// Finite weights can still overflow their sum; normalize by the largest.
	useScaledGrow := math.IsInf(growSum, 0)
	if useScaledGrow {
		growSum = 0
		for _, s := range sizes {
			if s.Kind == SizeGrow && maxGrow > 0 {
				growSum += s.Weight / maxGrow
			}
		}
	}

	result := make([]T, n)
	var growUsed, fixedUsed T
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
				result[i] = m.share(remaining, w/growSum)
			}
			growUsed = min(remaining, m.add(growUsed, result[i]))
		}
	}

	// Hand out what rounding left over in one pass (cells) — never one unit
	// at a time, which with a huge avail would effectively never finish.
	if rem := remaining - growUsed; rem > 0 && growCount > 0 {
		per, extra := m.spread(rem, growCount)
		for i, s := range sizes {
			if s.Kind != SizeGrow {
				continue
			}
			result[i] = m.add(result[i], per)
			if extra > 0 {
				result[i] = m.add(result[i], m.one())
				extra--
			}
		}
	}
	return result
}

// crossOf is a child's cross-axis length: Fixed pins, everything else
// stretches to fill.
func crossOf[T unit](s sized[T], avail T) T {
	s = s.normalized()
	avail = max(avail, 0)
	if s.Kind == SizeFixed && s.Value < avail {
		return s.Value
	}
	return avail
}

// layerLength maps an overlay's one-axis size into the viewport: Fixed and
// Content are exact (clamped), Auto fills, Grow is a fraction of the
// viewport (Weight >= 1 fills).
func layerLength[T unit](m metric[T], s sized[T], viewport T) T {
	s = s.normalized()
	viewport = max(viewport, 0)
	switch s.Kind {
	case SizeFixed, SizeContent:
		return min(s.Value, viewport)
	case SizeGrow:
		if s.Weight >= 1 {
			return viewport
		}
		return m.share(viewport, s.Weight)
	default:
		return viewport
	}
}

func centeredOf[T unit](m metric[T], n Node, vpW, vpH T) rect[T] {
	w, _ := intrinsic(m, n)
	lw := layerLength(m, w, vpW)
	_, h := measure(m, n, lw)
	lh := layerLength(m, h, vpH)
	return rect[T]{m.half(vpW - lw), m.half(vpH - lh), lw, lh}
}

// anchoredOf places a layer of r's size just below anchor, left-aligned to
// it — above it when there is no room below — and inside the frame.
func anchoredOf[T unit](r, anchor rect[T], vpW, vpH T) rect[T] {
	r.X = min(max(anchor.X, 0), max(vpW-r.W, 0))
	r.Y = anchor.Y + anchor.H
	if r.Y+r.H > vpH && anchor.Y-r.H >= 0 {
		r.Y = anchor.Y - r.H
	}
	r.Y = min(max(r.Y, 0), max(vpH-r.H, 0))
	return r
}

// clampScrollOf is ClampScroll in solver units.
func clampScrollOf[T unit](m metric[T], offset, viewport, content T) T {
	return min(max(offset, 0), max(m.sub(max(content, 0), max(viewport, 0)), 0))
}

// scrollContentOf is the height a Scroll's child is laid out at for a
// window viewport tall: Fixed exactly, Content at least the window,
// Grow/Auto the window.
func scrollContentOf[T unit](h sized[T], viewport T) T {
	switch h = h.normalized(); h.Kind {
	case SizeFixed:
		return h.Value
	case SizeContent:
		return max(h.Value, viewport)
	default:
		return viewport
	}
}

// --- the walk ---

// painter receives what the walk decides, in paint order. The cell painter
// composes a terminal frame; geometry-only solves pass none.
type painter[T unit] interface {
	leaf(n Node, r, clip rect[T])
	chrome(n Node, r, clip rect[T])
	layer(r rect[T])
}

type hitOf[T unit] struct {
	id         string
	rect, area rect[T]
}

type dupOf[T unit] struct {
	id          string
	first, dupe rect[T]
}

type solver[T unit] struct {
	m     metric[T]
	paint painter[T] // nil: geometry only
	rects map[string]rect[T]
	first map[string]rect[T]
	dups  []dupOf[T]
	hits  []hitOf[T]
	// clip is the visible region: the frame, narrowed by each enclosing
	// Scroll's window. Painting and hit areas are confined to it.
	clip rect[T]
}

// solveTree runs the single solve(+paint) walk over root in a frame w×h:
// the base pass, then each overlay layer centered on the frame.
func solveTree[T unit](m metric[T], root Node, w, h T, p painter[T]) *solver[T] {
	frame := rect[T]{0, 0, w, h}
	s := &solver[T]{m: m, paint: p, rects: map[string]rect[T]{}, first: map[string]rect[T]{}, clip: frame}
	// An overlay root is resolved only at its centered layer rect, never once
	// as a frame-sized base node.
	if !isOverlay(root) {
		s.walk(root, frame)
	}
	for _, o := range findOverlays(root) {
		r := centeredOf(m, o, w, h)
		if a, ok := s.rects[getAnchor(o)]; ok {
			r = anchoredOf(r, a, w, h)
		}
		if p != nil {
			p.layer(r)
		}
		// The layer is a hit barrier: above everything painted before it,
		// below its own nodes.
		s.hits = append(s.hits, hitOf[T]{rect: r, area: intersectOf(m, r, s.clip)})
		s.walk(o, r)
	}
	return s
}

// walk distributes rects top-down, records every ID'd node's rect and hit
// area, and hands leaves and container chrome to the painter. Containers are
// never Render'd here, so Row/Col may delegate their Render back to Compose.
func (s *solver[T]) walk(n Node, r rect[T]) {
	m := s.m
	if id := getID(n); id != "" {
		if first, ok := s.first[id]; ok {
			s.dups = append(s.dups, dupOf[T]{id, first, r})
		} else {
			s.first[id] = r
		}
		s.rects[id] = r
		s.hits = append(s.hits, hitOf[T]{id: id, rect: r, area: intersectOf(m, r, s.clip)})
	}

	if sc, ok := n.(scroller); ok {
		s.walkScroll(n, sc, r)
		return
	}

	container, isContainer := n.(childContainer)
	if !isContainer {
		if s.paint != nil {
			s.paint.leaf(n, r, s.clip)
		}
		return
	}
	if s.paint != nil {
		s.paint.chrome(n, r, s.clip)
	}

	children := nonOverlayChildren(container.GetChildren())
	if len(children) == 0 {
		return
	}
	horizontal := container.IsHorizontal()
	inner := insetOf(m, r, insetsOf[T](m, n))
	avail := inner.H
	pos := inner.Y
	if horizontal {
		avail, pos = inner.W, inner.X
	}
	gap := m.steps(getGap(n))
	cross := inner.W
	if horizontal {
		cross = noWidth[T]()
	}
	mainSizes := distribute(m, children, horizontal, gap, avail, cross)
	innerEnd := endOf(m, pos, avail)
	for i, c := range children {
		cw, ch := intrinsic(m, c)
		var cr rect[T]
		if horizontal {
			cr = rect[T]{pos, inner.Y, mainSizes[i], crossOf(ch, inner.H)}
		} else {
			cr = rect[T]{inner.X, pos, crossOf(cw, inner.W), mainSizes[i]}
		}
		// Fixed/Content children are not shrunk by distribution; clamp so an
		// overflowing row/col cannot place a child past its parent.
		s.walk(c, intersectOf(m, cr, inner))
		pos = min(innerEnd, m.add(pos, m.add(mainSizes[i], gap)))
	}
}

// walkScroll lays a Scroll's child out at its natural height behind the
// Scroll's inner box and walks it with the clip narrowed to that box.
func (s *solver[T]) walkScroll(n Node, sc scroller, r rect[T]) {
	m := s.m
	if s.paint != nil {
		s.paint.chrome(n, r, s.clip)
	}
	child, offset := sc.scroll()
	if child == nil || isOverlay(child) {
		return
	}
	inner := insetOf(m, r, insetsOf[T](m, n))
	cw, _ := intrinsic(m, child)
	_, ch := measure(m, child, crossOf(cw, inner.W))
	contentH := scrollContentOf(ch, inner.H)
	off := clampScrollOf(m, m.lines(offset), inner.H, contentH)
	cr := rect[T]{inner.X, m.sub(inner.Y, off), crossOf(cw, inner.W), contentH}

	saved := s.clip
	s.clip = intersectOf(m, saved, inner)
	s.walk(child, cr)
	s.clip = saved
}
