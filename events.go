package flatte

import "github.com/lunguini/flatte/flatui/layout"

// Event is the closed set of host inputs the loop delivers to Handle:
// terminal input, resizes, window appearance, and the solved layout of a
// Root frame. It is sealed: the framework defines every implementation, apps
// only consume them with a type switch. This is not TEA — events are host
// inputs, never app-defined messages; async results remain StateUpdates.
type Event interface{ isEvent() }

type Key int

const (
	KeyUnknown Key = iota
	KeyUp
	KeyDown
	KeyEnter
	KeyCtrlC
	KeyBackspace
	KeyCharacter
	KeyTab
	KeyEscape
	KeyLeft
	KeyRight
	KeyDelete
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
)

// Mod is a bitmask of key modifiers.
type Mod int

const (
	ModShift Mod = 1 << iota
	ModAlt
	ModCtrl
	// ModSuper is the Command key on macOS (the Windows or Super key
	// elsewhere): reported by a window, and by terminals that report it —
	// as Super, or as the xterm encoding's Meta bit (CSI 1;9D for ⌘←).
	ModSuper
)

func (m Mod) Contains(mods Mod) bool { return m&mods == mods }

// KeyEvent is a key press. Rune is set when Key is KeyCharacter.
//
// HasCaret reports that the host placed Caret: for KeyUp and KeyDown while
// a multi-line layout.Input has focus in a host that lays its text out
// differently from the app's widget (a window wraps by words in its font),
// Caret is the byte offset into that Input's Value one visual row up or
// down, at the column the caret is moving along. Pass it to the widget
// (flatui.Textarea.MoveTo) instead of its own row move. Terminal hosts
// leave it unset: the widget's rows are the terminal's.
type KeyEvent struct {
	Key  Key
	Rune rune
	Mod  Mod

	Caret    int
	HasCaret bool
}

// ResizeEvent reports the terminal size in cells. The loop delivers one at
// startup and, on Unix, one per SIGWINCH. On Windows, ultraviolet can deliver
// console window-size events when connected to a real console. Sizes fall back
// to 72×24 when the output is not a terminal.
type ResizeEvent struct {
	Width  int
	Height int
}

// AppearanceEvent reports the active light or dark appearance of a window
// host. It arrives at startup and when the host's appearance changes.
type AppearanceEvent struct{ Appearance Appearance }

// PasteEvent is a bracketed paste (paste mode is on by default; see
// WithoutBracketedPaste).
type PasteEvent struct{ Text string }

// ClipboardEvent delivers the terminal's answer to fx.ReadClipboard
// (OSC52, system selection). Unsupported terminals never answer — treat
// the event as optional and do not wait for it.
type ClipboardEvent struct{ Text string }

// FocusEvent reports terminal focus changes (see WithReportFocus).
type FocusEvent struct{ Focused bool }

// ModifiersEvent reports a change to the held modifier keys in a window.
// Terminal hosts report modifiers on MouseEvent instead.
type ModifiersEvent struct {
	Mod    Mod
	Window string
}

type MouseButton int

const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
	// MouseWheelLeft and MouseWheelRight are horizontal scrolling, as a
	// trackpad's sideways swipe or a tilted wheel reports it.
	MouseWheelLeft
	MouseWheelRight
	// MouseBack and MouseForward are a mouse's side buttons.
	MouseBack
	MouseForward
)

type MouseAction int

const (
	MousePress MouseAction = iota
	MouseRelease
	MouseMotion
)

// LayoutEvent reports the solved rect of every ID'd node in a Root frame.
// The host delivers it before painting whenever those rects differ from the
// ones it last delivered, so state derived from geometry — a list's visible
// rows, a pane's width — is adopted before the frame is shown. Handle it like
// ResizeEvent: copy what you need into state, and the next View sees it.
// Rects is the handler's own copy.
type LayoutEvent struct {
	Rects map[string]layout.Rect
	// PointRects preserves native point geometry before conversion to cells.
	// Window hosts populate it; terminal hosts leave it nil. Like Rects,
	// this is the handler's own copy. Prefer it for native measurements.
	PointRects map[string]layout.PointRect
	// Line is a window's line height in points: what a Scroll's OffsetY and
	// a wheel's Scroll count, so an app can scroll by points
	// (Scroll.OffsetPoints). Terminal hosts leave it 0.
	Line float64
	// Window is the window the rects are in ("" the main window).
	Window string
	// Faces is a window's cell of every grid face (layout.Face), in
	// points, so an app can size a cell grid set in a face from its node's
	// PointRect (CellSize.Cells). Terminal hosts leave it nil (a cell is a
	// cell). Like Rects, this is the handler's own copy.
	Faces map[layout.Face]layout.CellSize
}

// MouseEvent is a mouse press/release/motion/wheel; X/Y are zero-based
// cell coordinates from the top-left of the frame (see WithMouse).
type MouseEvent struct {
	X, Y   int
	Button MouseButton
	Action MouseAction
	Mod    Mod
	// Target is the ID of the topmost ID'd layout node under the pointer in
	// the last drawn frame, when that frame set Frame.Root. It is "" for
	// string frames, over an overlay area with no ID'd node, or where no
	// ID'd node is painted.
	Target string
	// TargetRect is the solved rect of Target (zero when Target is ""), so a
	// click can be made node-local with X - TargetRect.X, Y - TargetRect.Y.
	TargetRect layout.Rect
	// HasCaret reports that the pointer is over a layout.Input with an ID
	// (Target) and Caret is the byte offset into its Value nearest the
	// pointer, as the host laid the text out — so a click or drag can place
	// the caret exactly whatever the font. A window keeps reporting carets
	// for the Input a drag began on, clamped to it.
	Caret    int
	HasCaret bool
	// Clicks is the native consecutive click count (2: word, 3: all).
	// Zero means a normal single click, as in terminal events.
	Clicks int
	// Clear is a press on a clearable input's clear affordance. Target
	// remains the input ID; the app applies the edit on its loop.
	Clear bool
	// Window is the window the pointer is in ("" the main window); X, Y,
	// Target, and TargetRect are in its frame.
	Window string
	// Scroll is how many lines a wheel event scrolls, toward the end
	// (down or right) when positive: ±1 per notch, or a fraction of a line
	// from a trackpad in an app run WithPreciseScrolling. Read it with
	// ScrollLines, which also covers events that leave it zero.
	Scroll float64
	// HasPrecise reports that PreciseX, PreciseY is the pointer in
	// fractional cells (a window's points over the cell size), in the same
	// frame as X, Y — which are those rounded down. A terminal has whole
	// cells only; read the pointer with Point to cover both.
	PreciseX, PreciseY float64
	HasPrecise         bool
}

// ScrollLines is how many lines a wheel event scrolls, toward the end
// when positive: Scroll when set, else one notch for a wheel button, else
// zero.
func (m MouseEvent) ScrollLines() float64 {
	switch {
	case m.Scroll != 0:
		return m.Scroll
	case m.Button == MouseWheelDown || m.Button == MouseWheelRight:
		return 1
	case m.Button == MouseWheelUp || m.Button == MouseWheelLeft:
		return -1
	}
	return 0
}

// Point is the pointer in fractional cells: PreciseX, PreciseY when the
// host had them, else X, Y.
func (m MouseEvent) Point() (x, y float64) {
	if m.HasPrecise {
		return m.PreciseX, m.PreciseY
	}
	return float64(m.X), float64(m.Y)
}

// DropEvent reports files or text dropped on a window (a terminal turns a
// dropped file into a PasteEvent of its path instead). X, Y, Target, and
// TargetRect locate the drop as a MouseEvent's do; Paths are the dropped
// files, and Text is dropped text when there are none.
type DropEvent struct {
	X, Y       int
	Target     string
	TargetRect layout.Rect
	Paths      []string
	Text       string
	// Window is the window it was dropped on ("" the main window).
	Window string
}

// LeaveEvent reports that the pointer left the node ID, whose Box asked for
// it (layout.Box.ReportLeave), in a window: what an app holds still while
// the pointer is there can move again. Terminals never send it; an app gets
// the pointer's moves there with MouseModeAllMotion.
type LeaveEvent struct {
	ID string
	// Window is the node's window ("" the main window).
	Window string
}

func (LeaveEvent) isEvent()      {}
func (KeyEvent) isEvent()        {}
func (ResizeEvent) isEvent()     {}
func (PasteEvent) isEvent()      {}
func (ClipboardEvent) isEvent()  {}
func (FocusEvent) isEvent()      {}
func (ModifiersEvent) isEvent()  {}
func (MouseEvent) isEvent()      {}
func (LayoutEvent) isEvent()     {}
func (AppearanceEvent) isEvent() {}
func (DropEvent) isEvent()       {}
