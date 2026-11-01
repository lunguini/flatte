package flatui

import (
	"github.com/lunguini/flatte"
	"testing"
)

func TestTextFieldMouseSelectionAndClear(t *testing.T) {
	f := TextField{Value: "hello café world"}
	click := func(at, count int) {
		f.HandleMouse("q", flatte.MouseEvent{Target: "q", Button: flatte.MouseLeft, Action: flatte.MousePress, HasCaret: true, Caret: at, Clicks: count})
	}
	click(8, 2)
	if f.SelectedText() != "café" {
		t.Fatalf("word %q", f.SelectedText())
	}
	click(3, 3)
	if f.SelectedText() != f.Value {
		t.Fatal("triple click did not select all")
	}
	f.HandleMouse("q", flatte.MouseEvent{Target: "q", Button: flatte.MouseLeft, Action: flatte.MouseRelease, HasCaret: true, Caret: 3, Clicks: 3})
	if f.SelectedText() != f.Value {
		t.Fatal("release cleared multi-click selection")
	}
	click(1, 1)
	f.HandleMouse("q", flatte.MouseEvent{Target: "q", Button: flatte.MouseLeft, Action: flatte.MouseMotion, HasCaret: true, Caret: 5})
	if f.SelectedText() != "ello" {
		t.Fatalf("drag %q", f.SelectedText())
	}
	if !f.HandleMouse("q", flatte.MouseEvent{Target: "q", Button: flatte.MouseLeft, Action: flatte.MousePress, Clear: true}) || f.Value != "" || f.Cursor != 0 {
		t.Fatal("clear did not reset field")
	}
	if f.HandleMouse("q", flatte.MouseEvent{Target: "other", Button: flatte.MouseLeft, Action: flatte.MousePress, Clear: true}) {
		t.Fatal("foreign input claimed")
	}
}
func TestTextFieldDoubleClickKeepsGraphemeClusters(t *testing.T) {
	for _, tc := range []struct {
		text string
		at   int
		want string
	}{
		{"x e\u0301lan y", 4, "e\u0301lan"},
		{"x 👩‍👩‍👧‍👦 y", 4, "👩‍👩‍👧‍👦"},
		{"alpha   beta", 6, "   "},
		{"a,b", 1, ","},
		{"hello", 5, "hello"},
	} {
		f := TextField{Value: tc.text}
		f.HandleMouse("q", flatte.MouseEvent{Target: "q", Button: flatte.MouseLeft, Action: flatte.MousePress, HasCaret: true, Caret: tc.at, Clicks: 2})
		if got := f.SelectedText(); got != tc.want {
			t.Errorf("%q at%d selected %q want%q", tc.text, tc.at, got, tc.want)
		}
	}
}
