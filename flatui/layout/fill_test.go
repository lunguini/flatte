package layout

import (
	"strings"
	"testing"
)

// A Fill-only box reaches a GUI as a BoxOp; the terminal draws nothing for
// it (its own background shows), so the composed frame is blank.
func TestFillBoxEmitsSceneOpAndTerminalIgnoresIt(t *testing.T) {
	root := Col{NodeBase: NodeBase{ID: "pane", Box: Box{Fill: FillField}}}
	scene := LayoutScene(root, 100, 40, Metrics{Column: 10, Line: 20, Border: 10})
	found := false
	for _, op := range scene.Ops {
		if b, ok := op.(BoxOp); ok && b.ID == "pane" && b.Box.Fill == FillField {
			found = true
		}
	}
	if !found {
		t.Fatal("Fill-only box produced no BoxOp")
	}
	content, _ := SolveAndCompose(root, 6, 2)
	if strings.TrimSpace(content) != "" || strings.Contains(content, "\x1b[48") {
		t.Fatalf("terminal painted a Fill box: %q", content)
	}
}
