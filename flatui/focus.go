package flatui

// FocusRing is app-owned focus index state for cycling among focusable regions.
// It owns no key policy; apps decide whether Tab, Shift-Tab, mouse, or another
// command calls Next, Prev, or Select.
type FocusRing struct {
	count int
	index int
}

// SetCount sets the number of focusable regions, clamping the focused index.
func (f *FocusRing) SetCount(n int) {
	f.count = max(n, 0)
	f.clamp()
}

// Count is the number of focusable regions.
func (f FocusRing) Count() int { return f.count }

// Index is the focused index, or 0 when the ring is empty.
func (f FocusRing) Index() int { return f.index }

// Select focuses index i, clamped to the available region count.
func (f *FocusRing) Select(i int) {
	f.index = i
	f.clamp()
}

// Next moves focus forward, wrapping at the end.
func (f *FocusRing) Next() {
	if f.count == 0 {
		f.index = 0
		return
	}
	f.index = (f.index + 1) % f.count
}

// Prev moves focus backward, wrapping at the beginning.
func (f *FocusRing) Prev() {
	if f.count == 0 {
		f.index = 0
		return
	}
	f.index = (f.index - 1 + f.count) % f.count
}

// Focused reports whether i is the focused index.
func (f FocusRing) Focused(i int) bool {
	return f.count > 0 && f.index == i
}

func (f *FocusRing) clamp() {
	if f.count == 0 {
		f.index = 0
		return
	}
	f.index = min(max(f.index, 0), f.count-1)
}
