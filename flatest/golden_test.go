package flatest

import (
	"testing"

	"github.com/lunguini/flatte"
)

func TestRenderFrameWithoutMetadataEqualsCleanFrame(t *testing.T) {
	frame := flatte.Frame{Content: "\x1b[1mplain\x1b[0m\n"}
	if got, want := RenderFrame(frame), CleanFrame(frame.Content); got != want {
		t.Fatalf("RenderFrame = %q, want %q", got, want)
	}
}

func TestRenderFrameAppendsMetadataFooters(t *testing.T) {
	frame := flatte.Frame{
		Content: "body",
		Cursor:  &flatte.Cursor{X: 12, Y: 4},
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
		func(d *Driver[counter]) { d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '+'}) },
		func(d *Driver[counter]) { d.Send(flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: '+'}) },
	)
	AssertFrames(t, "testdata/counter-sequence.golden", frames)
}
