package snakeapp

import (
	"os"
	"testing"

	"github.com/lunguini/flatte/flatest"
)

// TestGenerateGoldens rewrites the golden files from the same frames the
// assertions use. There is no auto-update flag in the harness, so this is the
// deliberate, opt-in path: FLAT_GEN_GOLDENS=1 go test -run TestGenerateGoldens.
func TestGenerateGoldens(t *testing.T) {
	if os.Getenv("FLAT_GEN_GOLDENS") == "" {
		t.Skip("set FLAT_GEN_GOLDENS=1 to regenerate")
	}
	for _, c := range goldenFrames() {
		body := flatest.RenderFrame(c.frame) + "\n"
		if err := os.WriteFile("testdata/"+c.name+".golden", []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
