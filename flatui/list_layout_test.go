package flatui

import (
	"strconv"
	"strings"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestListLayoutUsesRenderRow(t *testing.T) {
	var l List
	l.SetCount(3)
	l.SetHeight(3)
	l.RenderRow = func(i int, sel bool) string {
		if sel {
			return "> row" + strconv.Itoa(i)
		}
		return "  row" + strconv.Itoa(i)
	}
	out, _ := layout.SolveAndCompose(l.Layout(), 20, 3)
	if !strings.Contains(out, "> row0") || !strings.Contains(out, "  row1") {
		t.Fatalf("list layout wrong:\n%s", out)
	}
}
