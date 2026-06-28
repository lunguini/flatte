package layout

// Element is anything that can participate in the layout tree. The engine
// calls Layout to obtain a subtree expressing the element's structure and
// sizing at the given dimensions.
//
// During the measure pass, Layout is called with generous dimensions to
// discover the element's intrinsic size (the returned subtree is measured).
// During the render pass, Layout is called with the solved dimensions, and
// the returned subtree is rendered within the element's rect.
//
// An element may return different subtrees at different sizes — truncating
// text, hiding chrome, or reorganizing layout as space contracts.
type Element interface {
	Layout(w, h int) Node
}

// noConstraint is passed to Layout during the measure pass to signal
// "no size limit" — the element should return its preferred subtree.
const noConstraint = 1 << 20
