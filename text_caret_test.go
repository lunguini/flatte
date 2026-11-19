package flatte

import (
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestComposeFramePlacesTextCaret(t *testing.T) {
	f, _ := ComposeFrame(Frame{Root: layout.Text{String: "ab\ncd", Caret: &layout.Caret{X: 1, Y: 1}}}, 10, 3)
	if f.Cursor == nil || f.Cursor.X != 1 || f.Cursor.Y != 1 {
		t.Fatalf("cursor = %+v, want 1,1", f.Cursor)
	}
}
