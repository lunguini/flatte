package flatte

import "github.com/lunguini/flatte/flatui/layout"

// Event is the closed set of host inputs the loop delivers to Handle:
// terminal input, resizes, and the solved layout of a Root frame. It is
// sealed: the framework defines every implementation, apps only consume them
// with a type switch. This is not TEA — events are host inputs, never
// app-defined messages; async results remain StateUpdates.
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
)

func (m Mod) Contains(mods Mod) bool { return m&mods == mods }

// KeyEvent is a key press. Rune is set when Key is KeyCharacter.
type KeyEvent struct {
	Key  Key
	Rune rune
	Mod  Mod
}

// ResizeEvent reports the terminal size in cells. The loop delivers one at
// startup and, on Unix, one per SIGWINCH. On Windows, ultraviolet can deliver
// console window-size events when connected to a real console. Sizes fall back
// to 72×24 when the output is not a terminal.
type ResizeEvent struct {
	Width  int
	Height int
}

// PasteEvent is a bracketed paste (paste mode is on by default; see
// WithoutBracketedPaste).
type PasteEvent struct{ Text string }

// ClipboardEvent delivers the terminal's answer to fx.ReadClipboard
// (OSC52, system selection). Unsupported terminals never answer — treat
// the event as optional and do not wait for it.
type ClipboardEvent struct{ Text string }

// FocusEvent reports terminal focus changes (see WithReportFocus).
type FocusEvent struct{ Focused bool }

type MouseButton int

const (
	MouseNone MouseButton = iota
	MouseLeft
	MouseMiddle
	MouseRight
	MouseWheelUp
	MouseWheelDown
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
}

func (KeyEvent) isEvent()       {}
func (ResizeEvent) isEvent()    {}
func (PasteEvent) isEvent()     {}
func (ClipboardEvent) isEvent() {}
func (FocusEvent) isEvent()     {}
func (MouseEvent) isEvent()     {}
func (LayoutEvent) isEvent()    {}
