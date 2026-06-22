package flatte

// Event is the closed set of terminal inputs the loop delivers to Handle.
// It is sealed: the framework defines every implementation, apps only
// consume them with a type switch. This is not TEA — events are terminal
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
// startup and one per SIGWINCH; sizes fall back to 72×24 when the output is
// not a terminal.
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

// MouseEvent is a mouse press/release/motion/wheel; X/Y are zero-based
// cell coordinates from the top-left of the frame (see WithMouse).
type MouseEvent struct {
	X, Y   int
	Button MouseButton
	Action MouseAction
	Mod    Mod
}

func (KeyEvent) isEvent()       {}
func (ResizeEvent) isEvent()    {}
func (PasteEvent) isEvent()     {}
func (ClipboardEvent) isEvent() {}
func (FocusEvent) isEvent()     {}
func (MouseEvent) isEvent()     {}
