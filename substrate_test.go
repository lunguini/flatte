package flat

import (
	"strings"
	"testing"

	uv "github.com/charmbracelet/ultraviolet"
)

// These tests pin the ultraviolet behaviors Phase 3 relies on. If a future
// re-pin of uv breaks one, the migration cost surfaces here first.

func TestStyledStringRoundTripsThroughCells(t *testing.T) {
	frame := "\x1b[1mtitle\x1b[m\nplain line"

	styled := uv.NewStyledString(frame)
	buf := uv.NewScreenBuffer(20, 2)
	styled.Draw(buf, buf.Bounds())

	if cell := buf.CellAt(0, 0); cell == nil || cell.Content != "t" {
		t.Fatalf("CellAt(0,0) = %+v, want 't'", cell)
	}
	if got := strings.TrimRight(strings.Split(buf.String(), "\n")[1], " "); got != "plain line" {
		t.Fatalf("plain row = %q, want %q", got, "plain line")
	}
}

func TestBufferRenderPreservesContent(t *testing.T) {
	styled := uv.NewStyledString("\x1b[44mhello\x1b[m world")
	buf := uv.NewScreenBuffer(12, 1)
	styled.Draw(buf, buf.Bounds())

	rendered := buf.Render()
	if !strings.Contains(rendered, "hello") || !strings.Contains(rendered, "world") {
		t.Fatalf("Render() lost content: %q", rendered)
	}
}
