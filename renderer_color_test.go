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

func TestRenderContextForCarriesOutputColorProfile(t *testing.T) {
	oldDetect := detectRendererColorProfile
	t.Cleanup(func() { detectRendererColorProfile = oldDetect })

	var detected io.Writer
	detectRendererColorProfile = func(w io.Writer, env []string) colorprofile.Profile {
		detected = w
		return colorprofile.TrueColor
	}

	var out bytes.Buffer
	ctx := RenderContextFor(&out)
	if detected != &out {
		t.Fatalf("RenderContextFor detected profile from %T, want output writer", detected)
	}
	if ctx.ColorProfile != colorprofile.TrueColor {
		t.Fatalf("ColorProfile = %v, want %v", ctx.ColorProfile, colorprofile.TrueColor)
	}
}
