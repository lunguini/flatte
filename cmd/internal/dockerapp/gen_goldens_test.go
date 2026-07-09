package dockerapp

import (
	"os"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
)

func TestGenerateGoldens(t *testing.T) {
	if os.Getenv("FLAT_GEN_GOLDENS") == "" {
		t.Skip("set FLAT_GEN_GOLDENS=1 to regenerate")
	}
	cases := []struct {
		path   string
		screen screen
	}{
		{"testdata/containers.golden", screenContainers},
		{"testdata/images.golden", screenImages},
		{"testdata/volumes.golden", screenVolumes},
	}
	for _, c := range cases {
		s := NewState()
		Handle(s, flatte.ResizeEvent{Width: 80, Height: 24}, flatte.Effects[State]{})
		s.screen = c.screen
		frame := View(s, flatte.RenderContext{Width: 80})
		body := flatest.RenderFrame(frame) + "\n"
		if err := os.WriteFile(c.path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
