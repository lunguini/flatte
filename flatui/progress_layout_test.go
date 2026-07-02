package flatui

import (
	"strings"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestProgressLayoutMatchesView(t *testing.T) {
	p := NewProgress(10)
	p.SetPercent(50)
	out, _ := layout.SolveAndCompose(p.Layout(), 40, 1)
	if !strings.Contains(out, "50%") {
		t.Fatalf("progress layout missing label:\n%s", out)
	}
}
