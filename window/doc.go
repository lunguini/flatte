// Package window runs a Flatte app in a native desktop window instead of a
// terminal. The app is unchanged — the same State, Handle, and View — and the
// host is chosen at the call site:
//
//	flatte.Run(ctx, app)                   // terminal
//	window.Run(ctx, app, window.Options{}) // native window
//
// The window draws the app's Frame.Root as real components: Labels in the
// theme's font by text role, Boxes as rounded strokes (an overlay's box as a
// raised surface, a titled border's title in its stroke), Inputs as outlined
// text fields (multi-line ones as text areas) with input-method support,
// and scroll bars as thin thumbs.
// Layout runs in points (see layout.LayoutScene): sizes are columns and
// lines of the UI font, padding is spacing steps, and a border takes a band
// with its stroke down the middle, as a terminal border takes a cell — so a
// layout written for the terminal keeps its proportions without terminal
// chunkiness. Nodes the window does not know — a Text of ANSI output, a
// container's Chrome, a frame with no Root — show as a monospace cell grid
// (line-drawing, block, and Powerline characters drawn as shapes that fill
// their cells), so every existing app runs.
//
// The look is a Theme: colors for what the app does not color itself, the
// UI and fixed-pitch font families and sizes, line spacing, and corner
// radius. With no Options.Theme the window follows the system's light or
// dark appearance as it changes; set one (Dark or Light with fields
// changed) to fix the look:
//
//	th := window.Dark()
//	th.Accent, th.Radius = color.RGBA{0xff, 0x80, 0x00, 0xff}, -1 // square
//	window.Run(ctx, app, window.Options{Theme: &th})
//
// Geometry reaches the app in cells, as in a terminal: ResizeEvent and
// LayoutEvent sizes are whole columns and lines, and MouseEvent coordinates
// are cells whose offset from TargetRect is exact inside the target.
//
// macOS only for now, with no cgo: the system frameworks are bound at run
// time through purego. On other platforms Run returns ErrUnsupported.
//
// Menus: App.Menus is the native menu bar, between the application menu
// (Quit, ⌘Q) and the Window menu (Close, ⌘W); an item's first ⌘ shortcut is
// its native key equivalent, and its other shortcuts are routed by the
// host. App.ContextMenu shows as a native popup on a right-click or
// Control-click. Choices arrive as MenuEvents. An item with the ⌘C shortcut
// copies selected Label text instead, when there is some.
//
// Editing: the clipboard effects use the system pasteboard (ReadClipboard
// answers with a ClipboardEvent), ⌘V delivers a PasteEvent unless a menu
// item claims it, and ⌘ chords no menu item claims arrive as keys with ModSuper —
// except ⌘← and ⌘→, which arrive as Home and End (with Shift when held).
// Pointer events over an Input carry the caret under the pointer, and ↑/↓
// in a focused multi-line Input carry the caret a visual row away, both in
// the window's own text layout (see MouseEvent.Caret, KeyEvent.Caret). The
// caret blinks.
//
// Differences from the terminal host: Ctrl-C is delivered to Handle (quit
// with ⌘Q or by closing the window); Effects.Print, Suspend, and Exec are
// no-ops; Frame.Cursor shows only for frames without a Root (a Root frame's
// cursor is the focused Input's caret).
package window
