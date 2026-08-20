package flatui

// maxWidgetDimension bounds caller-provided cell counts before they reach
// allocation and repetition primitives. It is deliberately far above a real
// terminal dimension while keeping malformed values deterministic and finite.
const maxWidgetDimension = 1 << 16

func clampWidgetDimension(n int) int {
	return min(max(n, 0), maxWidgetDimension)
}
