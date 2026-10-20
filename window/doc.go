// Package window runs a Flatte app in a native desktop window instead of a
// terminal. The app is unchanged — the same State, Handle, and View — and the
// host is chosen at the call site:
//
//	flatte.Run(ctx, app)                   // terminal
//	window.Run(ctx, app, window.Options{}) // native window
//
// The window draws the app's Frame.Root as real components: Labels in the
// system font by text role, Boxes as rounded strokes (an overlay's box as a
// raised surface), Inputs as outlined text fields with input-method support.
// Layout runs in points (see layout.LayoutScene): sizes are columns and
// lines of the UI font, padding is spacing steps, and a border takes a band
// with its stroke down the middle, as a terminal border takes a cell — so a
// layout written for the terminal keeps its proportions without terminal
// chunkiness. Nodes the window does not know — a Text of ANSI output, a
// container's Chrome, a frame with no Root — show as a monospace cell grid
// (line-drawing, block, and Powerline characters drawn as shapes that fill
// their cells), so every existing app runs.
//
// Geometry reaches the app in cells, as in a terminal: ResizeEvent and
// LayoutEvent sizes are whole columns and lines, and MouseEvent coordinates
// are cells whose offset from TargetRect is exact inside the target.
//
// macOS only for now, with no cgo: the system frameworks are bound at run
// time through purego. On other platforms Run returns ErrUnsupported.
//
// Differences from the terminal host: Ctrl-C is delivered to Handle (quit
// with ⌘Q or by closing the window); Effects.Print, Suspend, Exec, and the
// clipboard methods are no-ops; Frame.Cursor shows only for frames without
// a Root (a Root frame's cursor is the focused Input's caret).
package window
