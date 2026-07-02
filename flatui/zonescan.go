package flatui

// ZoneScanner maps named rectangles to hit regions. Rects are fed from solved
// layout geometry via Set — the blessed render path composites through a cell
// buffer, which drops any inline string markers, so zones are always derived
// from coordinates rather than scanned out of rendered output.
type ZoneScanner struct {
	order []string
	rects map[string]Rect
}

func NewZoneScanner() *ZoneScanner {
	return &ZoneScanner{rects: make(map[string]Rect)}
}

// Reset clears all zones. Use before repopulating from geometry each frame.
func (z *ZoneScanner) Reset() {
	z.order = z.order[:0]
	clear(z.rects)
}

// Set registers id at rect r, replacing any existing rect for id. A re-Set does
// not change hit priority; the original insertion order is preserved so At stays
// stable across frames.
func (z *ZoneScanner) Set(id string, r Rect) {
	if _, exists := z.rects[id]; !exists {
		z.order = append(z.order, id)
	}
	z.rects[id] = r
}

// At returns the last-inserted zone containing x,y. Later Set calls for new ids
// take priority when rectangles overlap.
func (z ZoneScanner) At(x, y int) (string, bool) {
	for i := len(z.order) - 1; i >= 0; i-- {
		id := z.order[i]
		if z.rects[id].Contains(x, y) {
			return id, true
		}
	}
	return "", false
}

func (z ZoneScanner) Rect(id string) (Rect, bool) {
	r, ok := z.rects[id]
	return r, ok
}

func (z ZoneScanner) In(id string, x, y int) bool {
	r, ok := z.rects[id]
	return ok && r.Contains(x, y)
}
