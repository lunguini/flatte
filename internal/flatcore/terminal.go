package flatcore

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/term"
)

const fallbackRenderWidth = 72

type RenderContext struct {
	Width int
}

type fdWriter interface {
	Fd() uintptr
}

type Renderer interface {
	Draw(io.Writer, string, RenderContext)
	Reset()
}

type FullRenderer struct{}

func (FullRenderer) Draw(out io.Writer, frame string, ctx RenderContext) {
	Draw(out, frame)
}

func (FullRenderer) Reset() {}

type DiffRenderer struct {
	previous []string
	width    int
	drew     bool
}

func NewDiffRenderer() *DiffRenderer {
	return &DiffRenderer{}
}

func (r *DiffRenderer) Draw(out io.Writer, frame string, ctx RenderContext) {
	rows := frameRows(frame)
	if !r.drew || r.width != ctx.Width {
		Draw(out, frame)
		r.previous = rows
		r.width = ctx.Width
		r.drew = true
		return
	}

	sharedRows := len(rows)
	if len(r.previous) < sharedRows {
		sharedRows = len(r.previous)
	}
	for index := 0; index < sharedRows; index++ {
		if rows[index] == r.previous[index] {
			continue
		}
		writeRow(out, index+1, rows[index])
	}
	for index := sharedRows; index < len(rows); index++ {
		writeRow(out, index+1, rows[index])
	}
	for index := len(rows); index < len(r.previous); index++ {
		clearRow(out, index+1)
	}

	r.previous = rows
}

func (r *DiffRenderer) Reset() {
	r.previous = nil
	r.width = 0
	r.drew = false
}

func RenderContextFor(out io.Writer) RenderContext {
	width := fallbackRenderWidth
	if file, ok := out.(fdWriter); ok && term.IsTerminal(int(file.Fd())) {
		// term.GetSize returns (width, height, err) — width first.
		if terminalWidth, _, err := term.GetSize(int(file.Fd())); err == nil && terminalWidth > 0 {
			width = terminalWidth
		}
	}
	return RenderContext{Width: width}
}

func Draw(out io.Writer, frame string) {
	fmt.Fprintf(out, "\x1b[H\x1b[2J%s", TerminalFrame(frame))
}

func TerminalFrame(frame string) string {
	frame = strings.ReplaceAll(frame, "\r\n", "\n")
	return strings.ReplaceAll(frame, "\n", "\r\n")
}

func frameRows(frame string) []string {
	frame = strings.ReplaceAll(frame, "\r\n", "\n")
	return strings.Split(frame, "\n")
}

func writeRow(out io.Writer, row int, content string) {
	fmt.Fprintf(out, "\x1b[%d;1H\x1b[2K%s", row, content)
}

func clearRow(out io.Writer, row int) {
	fmt.Fprintf(out, "\x1b[%d;1H\x1b[2K", row)
}
