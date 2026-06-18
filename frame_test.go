package flat

import "testing"

func TestFramesEqual(t *testing.T) {
	cursor := func(x, y int) *Cursor { return &Cursor{X: x, Y: y} }
	cases := []struct {
		name string
		a, b Frame
		want bool
	}{
		{"both zero", Frame{}, Frame{}, true},
		{"same content", Frame{Content: "x"}, Frame{Content: "x"}, true},
		{"different content", Frame{Content: "x"}, Frame{Content: "y"}, false},
		{"different title", Frame{Title: "a"}, Frame{Title: "b"}, false},
		{"nil vs set cursor", Frame{}, Frame{Cursor: cursor(0, 0)}, false},
		{"equal cursors, distinct pointers", Frame{Cursor: cursor(1, 2)}, Frame{Cursor: cursor(1, 2)}, true},
		{"different cursor position", Frame{Cursor: cursor(1, 2)}, Frame{Cursor: cursor(1, 3)}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := framesEqual(tc.a, tc.b); got != tc.want {
				t.Fatalf("framesEqual = %v, want %v", got, tc.want)
			}
		})
	}
}
