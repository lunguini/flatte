package flatte

import (
	"io"
	"os"

	"github.com/charmbracelet/colorprofile"
	"golang.org/x/term"
)

const (
	fallbackRenderWidth  = 72
	fallbackRenderHeight = 24
)

type RenderContext struct {
	Width        int
	Height       int
	ColorProfile colorprofile.Profile
	// Window is the window being drawn: "" for the main window, or a
	// secondary window's ID (App.Windows).
	Window string
	// Graphical reports a host that draws in points — a native window:
	// Points sizes, images, and icon shapes show as themselves. False in a
	// terminal, where they round up to whole cells and show their text, so
	// a view can size generously for one and compactly for the other.
	Graphical    bool
	Capabilities Capabilities
}

// Capabilities reports effect and input support provided by the current host.
type Capabilities struct {
	Exec, Suspend, Print                  bool
	Alert, OpenFile, SaveFile             bool
	Notifications, Clipboard, Drop, Mouse bool
	Graphical, Quit                       bool
}

type fdWriter interface {
	Fd() uintptr
}

// terminalSize returns the output terminal's size in cells, falling back to
// 72×24 when the output is not a terminal (pipes in tests).
func terminalSize(out io.Writer) (width, height int) {
	width, height = fallbackRenderWidth, fallbackRenderHeight
	if file, ok := out.(fdWriter); ok && term.IsTerminal(int(file.Fd())) {
		// term.GetSize returns (width, height, err) — width first.
		if w, h, err := term.GetSize(int(file.Fd())); err == nil {
			if w > 0 {
				width = w
			}
			if h > 0 {
				height = h
			}
		}
	}
	return width, height
}

// RenderContextFor reports the terminal facts available to View. It mirrors
// renderer color-profile detection so apps can pick theme styles without
// owning terminal probing.
func RenderContextFor(out io.Writer) RenderContext {
	width, height := terminalSize(out)
	return RenderContext{
		Width:        width,
		Height:       height,
		ColorProfile: detectRendererColorProfile(out, os.Environ()),
		Capabilities: Capabilities{Exec: true, Suspend: true, Print: true, Alert: true, OpenFile: true, SaveFile: true, Clipboard: true, Mouse: true, Quit: true},
	}
}
