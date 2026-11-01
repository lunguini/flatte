// Package window runs a Flatte app in a GUI surface instead of a terminal.
// The app is unchanged — the same State, Handle, and View — and the host is
// selected with a flatte.Run option:
//
//	flatte.Run(ctx, app)                                    // terminal
//	flatte.Run(ctx, app, window.Options{})                  // native window
//	flatte.Run(ctx, app, window.Options{}, flatte.WithMouse(flatte.MouseModeCellMotion))
//	flatte.Run(ctx, app, media.Controls{})                  // macOS terminal media controls
//
// On macOS, Options selects an AppKit window. On js/wasm, the same option
// selects a browser canvas; without it, Run keeps the cell-grid page.
//
// Run passes the window driver a *flatte.Session. Native callbacks send events
// and frame requests through its non-blocking Step method; the driver keeps
// point layout and AppKit painting on its main-thread surfaces. Custom drivers
// implement flatte.Host in the root package; see the quick reference's
// "Writing a host" section.
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
//	flatte.Run(ctx, app, window.Options{Theme: &th})
//
// Geometry reaches the app in cells, as in a terminal: ResizeEvent and
// LayoutEvent sizes are whole columns and lines, and MouseEvent coordinates
// are cells whose offset from TargetRect is exact inside the target.
//
// The native backend is macOS-only and uses no cgo: system frameworks are
// bound at run time through purego. The js/wasm backend paints LayoutScene
// operations into a 2D canvas, measures text with Canvas, loads Options.Fonts
// through FontFace before the first frame, and scales the backing store for
// device pixel ratio. It uses a hidden textarea for keyboard, IME, paste, and
// caret input. Images are encoded from the app's image.Image source into a
// Blob and decoded asynchronously with createImageBitmap; failed decodes show
// a subtle music-note placeholder. Source fetching still follows the browser's
// CORS rules.
//
// In a page, Options.Theme and Fonts apply. Title is the fallback document
// title. Desktop, Width, Height, MinWidth, MinHeight, Autosave, HideTitleBar,
// and Translucent are ignored. Browser Alerts use alert/confirm; file panels,
// desktop services, menu presentation, and secondary windows are unavailable.
// On other native platforms Options installs no host and flatte.Run falls back
// to the terminal. Supported is true on macOS and js/wasm.
// The separate github.com/lunguini/flatte/window/media package provides
// media.Controls for terminal runs. It is a fallback host, so it can be
// passed with Options; the window host already connects media when
// App.NowPlaying is set. Other genuinely conflicting hosts return
// flatte.ErrMultipleHosts.
//
// Menus: App.Menus is the native menu bar, between the application menu
// (Quit, ⌘Q) and the Window menu (Close, ⌘W); an item's first ⌘ shortcut is
// its native key equivalent, and its other shortcuts are routed by the
// host. App.ContextMenu shows as a native popup on a right-click or
// Control-click. Choices arrive as MenuEvents. An item with the ⌘C shortcut
// copies selected Label text instead, when there is some.
// The browser canvas routes App.Menus shortcuts as MenuEvents, but does not
// draw the menu bar or context popups.
//
// Windows: App.Windows declares secondary windows, each opened as its own
// native window drawn by View (RenderContext.Window); the close button sends
// a WindowCloseRequest, and key changes a WindowFocused. Options.MinWidth,
// MinHeight, and Autosave set the main window's minimum size and saved
// frame; the Window menu minimizes, zooms, and enters full screen.
// The browser canvas does not open secondary windows.
//
// Dialogs: OpenFile, SaveFile, and Alert show the system panels and alerts,
// run after the handler that asked returns. Files or text dropped on the
// window arrive as a DropEvent.
// In a browser, Alert uses alert/confirm, file panels report ErrNoDialogs,
// and drops include file names and plain text.
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
