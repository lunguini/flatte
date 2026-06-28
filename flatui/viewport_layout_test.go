package flatui

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestViewportLayoutRendersVisibleSlice(t *testing.T) {
	var v Viewport
	v.SetSize(20, 2)
	v.SetContent("a\nb\nc")
	out := layout.Render(v.Layout(), 20, 2)
	if !strings.Contains(out, "a") || strings.Contains(out, "c") {
		t.Fatalf("viewport layout window wrong:\n%s", out)
	}
}
