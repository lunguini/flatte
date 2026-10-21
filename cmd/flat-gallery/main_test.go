package main

import (
	"os"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui/layout"
)

func send(s *State, evs ...flatte.Event) {
	for _, ev := range evs {
		Handle(s, ev, flatte.Effects[State]{})
	}
}

func TestKeysChooseAndFit(t *testing.T) {
	s := NewState()
	send(s, flatte.KeyEvent{Key: flatte.KeyLeft})
	if s.selected != 2 {
		t.Fatalf("left from the first: %d", s.selected)
	}
	send(s, flatte.KeyEvent{Key: flatte.KeyRight}, flatte.KeyEvent{Key: flatte.KeyTab})
	if s.selected != 1 {
		t.Fatalf("right, tab: %d", s.selected)
	}
	f := flatte.KeyEvent{Key: flatte.KeyCharacter, Rune: 'f'}
	send(s, f)
	if s.fit != layout.ImageCover {
		t.Fatalf("fit %v", s.fit)
	}
	send(s, f, f)
	if s.fit != layout.ImageContain {
		t.Fatalf("fit wraps to %v", s.fit)
	}
}

func TestClickChoosesAPicture(t *testing.T) {
	d := flatest.Start(flatte.App[State]{State: NewState(), Handle: Handle, View: View}, 80)
	d.Send(flatte.ResizeEvent{Width: 80, Height: 34})
	_, comp := flatte.ComposeFrame(d.Frame(), 80, 34)
	r, ok := comp.Rects["waves"]
	if !ok {
		t.Fatal("no rect for waves")
	}
	d.Send(flatte.MouseEvent{X: r.X + 2, Y: r.Y + 1, Button: flatte.MouseLeft, Action: flatte.MousePress})
	if d.State().selected != 2 {
		t.Fatalf("selected %d", d.State().selected)
	}
	// Pictures are Points sizes: 160×96pt is 20 columns by 6 lines.
	if r.W != 20 || r.H != 6 {
		t.Errorf("picture rect %+v", r)
	}
}

// The pictures are made once: View hands the window the same images every
// frame, so it converts each once.
func TestViewKeepsItsPictures(t *testing.T) {
	s := NewState()
	images := func() []any {
		var out []any
		var walk func(n layout.Node)
		walk = func(n layout.Node) {
			switch n := n.(type) {
			case layout.Image:
				out = append(out, n.Source)
			case layout.Col:
				for _, c := range n.Children {
					walk(c)
				}
			case layout.Row:
				for _, c := range n.Children {
					walk(c)
				}
			}
		}
		walk(View(s, flatte.RenderContext{}).Root)
		return out
	}
	a, b := images(), images()
	if len(a) != 3 {
		t.Fatalf("%d images", len(a))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("image %d differs between frames", i)
		}
	}
}

func TestGallerySnapshot(t *testing.T) {
	s := NewState()
	f, _ := flatte.ComposeFrame(View(s, flatte.RenderContext{Width: 80}), 80, 34)
	if os.Getenv("FLAT_PRINT") != "" {
		t.Log("\n" + flatest.RenderFrame(f))
	}
	flatest.AssertGoldenFrame(t, "testdata/gallery.golden", f)
	if !strings.Contains(flatest.CleanFrame(f.Content), "bands of waves") {
		t.Error("the terminal does not describe the pictures")
	}
}
