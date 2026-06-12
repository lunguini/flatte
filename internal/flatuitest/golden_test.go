package flatuitest

import (
	"testing"

	"github.com/lunguini/flat/internal/flatcore"
)

func TestRenderFrameWithoutMetadataEqualsCleanFrame(t *testing.T) {
	frame := flatcore.Frame{Content: "\x1b[1mplain\x1b[0m\n"}
	if got, want := RenderFrame(frame), CleanFrame(frame.Content); got != want {
		t.Fatalf("RenderFrame = %q, want %q", got, want)
	}
}

func TestRenderFrameAppendsMetadataFooters(t *testing.T) {
	frame := flatcore.Frame{
		Content: "body",
		Cursor:  &flatcore.Cursor{X: 12, Y: 4},
		Title:   "demo",
	}
	want := "body\n[cursor 12,4]\n[title demo]"
	if got := RenderFrame(frame); got != want {
		t.Fatalf("RenderFrame = %q, want %q", got, want)
	}
}
