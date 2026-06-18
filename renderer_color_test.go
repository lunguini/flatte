package flat

import (
	"bytes"
	"io"
	"testing"

	"github.com/charmbracelet/colorprofile"
)

func TestNewTerminalRendererDetectsColorFromOutputWriter(t *testing.T) {
	renderBuffer := &bytes.Buffer{}
	output := &bytes.Buffer{}

	oldDetect := detectRendererColorProfile
	t.Cleanup(func() { detectRendererColorProfile = oldDetect })

	var gotWriter io.Writer
	detectRendererColorProfile = func(w io.Writer, env []string) colorprofile.Profile {
		gotWriter = w
		return colorprofile.TrueColor
	}

	_ = newTerminalRenderer(renderBuffer, output, nil, false)

	if gotWriter != output {
		t.Fatalf("color profile detected from %T, want output writer %T", gotWriter, output)
	}
}
