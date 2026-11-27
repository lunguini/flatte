package layout

// Fit uses Preferred when its measured width fits the space it gets, and
// Fallback otherwise. A points backend measures Preferred in its active font;
// a cell backend measures it in cells. Put IDs on the nodes in each branch.
type Fit struct {
	// Preferred is used when its measured width fits.
	Preferred Node
	// Fallback is used when Preferred is too wide. A nil fallback keeps Preferred.
	Fallback Node
}

func (f Fit) Size() (Size, Size) {
	n := f.Preferred
	if n == nil {
		n = f.Fallback
	}
	if n == nil {
		return Size{}, Size{}
	}
	return n.Size()
}

func (f Fit) Render(r Rect) string {
	n := fittedChild(cells{}, f, r.W)
	if n == nil {
		return ""
	}
	return n.Render(r)
}

func fittedChild[T unit](m metric[T], f Fit, width T) Node {
	if f.Preferred == nil {
		return f.Fallback
	}
	if f.Fallback == nil || width < 0 {
		return f.Preferred
	}
	w, _ := measure(m, f.Preferred, noWidth[T]())
	w = w.normalized()
	if (w.Kind == SizeFixed || w.Kind == SizeContent) && w.Value <= width {
		return f.Preferred
	}
	return f.Fallback
}
