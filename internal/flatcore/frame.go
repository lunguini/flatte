package flatcore

// Frame is what View returns: rendered content plus terminal metadata.
// The zero value is a blank frame with no cursor and no title.
type Frame struct {
	// Content is the styled frame text.
	Content string
	// Cursor places the hardware cursor, in frame cell coordinates
	// ((0,0) is the frame's top-left). nil hides the cursor.
	Cursor *Cursor
	// Title sets the terminal window title when non-empty. It is emitted
	// only when it changes, and reset on exit if it was ever set.
	Title string
}

// Cursor is a hardware-cursor position in frame cell coordinates.
type Cursor struct {
	X, Y int
}

// framesEqual reports whether two frames would render identically,
// comparing the cursor by value rather than by pointer.
func framesEqual(a, b Frame) bool {
	if a.Content != b.Content || a.Title != b.Title {
		return false
	}
	if (a.Cursor == nil) != (b.Cursor == nil) {
		return false
	}
	return a.Cursor == nil || *a.Cursor == *b.Cursor
}
