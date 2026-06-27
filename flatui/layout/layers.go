package layout

// solveOverlayPass walks the tree looking for Layer-flagged nodes. Each layer
// is solved against the full viewport rect (not its flex parent), so it can
// position itself independently — this is how modals and command palettes
// work without ad-hoc coordinate math.
//
// Layer nodes were excluded from normal flex layout in solveNode, so the base
// layout already fits as if the layer weren't there. This pass overlays them.
func solveOverlayPass(n Node, vp Rect, out map[string]Rect) {
	for _, c := range n.Children {
		if c.Overlay {
			solveLayer(c, vp, out)
		}
		solveOverlayPass(c, vp, out)
	}
}

// solveLayer positions a layer node against the viewport. The layer's W and H
// constraints resolve against the viewport dimensions: Fixed uses the literal
// value, Grow takes a fraction of the viewport, Auto fills entirely. The layer
// is centered within the viewport.
func solveLayer(n Node, vp Rect, out map[string]Rect) {
	lw := resolveLayerSize(n.W, vp.W)
	lh := resolveLayerSize(n.H, vp.H)

	// Center within viewport.
	x := vp.X + (vp.W-lw)/2
	y := vp.Y + (vp.H-lh)/2

	solveNode(n, Rect{X: x, Y: y, W: lw, H: lh}, out)
}

// resolveLayerSize resolves a Size constraint against a viewport dimension.
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
	default: // SizeAuto — fill the viewport.
		return viewport
	}
}
