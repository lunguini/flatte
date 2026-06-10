package flatcore

import (
	"io"

	"golang.org/x/term"
)

const (
	fallbackRenderWidth  = 72
	fallbackRenderHeight = 24
)

type RenderContext struct {
	Width int
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

func RenderContextFor(out io.Writer) RenderContext {
	width, _ := terminalSize(out)
	return RenderContext{Width: width}
}
