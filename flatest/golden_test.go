package flatest

import (
	"testing"

	"github.com/lunguini/flat"
)

func TestRenderFrameWithoutMetadataEqualsCleanFrame(t *testing.T) {
	frame := flat.Frame{Content: "\x1b[1mplain\x1b[0m\n"}
	if got, want := RenderFrame(frame), CleanFrame(frame.Content); got != want {
		t.Fatalf("RenderFrame = %q, want %q", got, want)
	}
}

func TestRenderFrameAppendsMetadataFooters(t *testing.T) {
	frame := flat.Frame{
		Content: "body",
		Cursor:  &flat.Cursor{X: 12, Y: 4},
		Title:   "demo",
	}
	want := "body\n[cursor 12,4]\n[title demo]"
	if got := RenderFrame(frame); got != want {
		t.Fatalf("RenderFrame = %q, want %q", got, want)
	}
}

func TestAssertFramesMatchesSequenceGolden(t *testing.T) {
	d := Start(counterApp(), 40)
	frames := Frames(d,
		func(d *Driver[counter]) {}, // initial frame
		func(d *Driver[counter]) { d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: '+'}) },
		func(d *Driver[counter]) { d.Send(flat.KeyEvent{Key: flat.KeyCharacter, Rune: '+'}) },
	)
	AssertFrames(t, "testdata/counter-sequence.golden", frames)
}
