package window

import (
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/lunguini/flatte/flatui/layout"
)

var mediaIcons = []layout.IconName{
	layout.IconPlay, layout.IconPause, layout.IconPrevious, layout.IconNext, layout.IconShuffle,
	layout.IconRepeat, layout.IconSpeaker, layout.IconHome, layout.IconHeart, layout.IconHeartOutline,
	layout.IconClock, layout.IconEqualizer, layout.IconLibrary,
}

func TestMediaIconsStayInTheirBox(t *testing.T) {
	const cx, cy, s = 50.0, 40.0, 20.0
	in := func(p point) bool {
		return p.X >= cx-s/2-0.01 && p.X <= cx+s/2+0.01 && p.Y >= cy-s/2-0.01 && p.Y <= cy+s/2+0.01
	}
	for _, name := range mediaIcons {
		g, ok := symbol(name, cx, cy, s)
		if !ok || len(g.polys)+len(g.circles) == 0 {
			t.Fatalf("icon %d has no shape", name)
		}
		for _, p := range g.polys {
			for _, pt := range p.pts {
				if !in(pt) {
					t.Errorf("icon %d point %+v outside its %vpt box", name, pt, s)
				}
			}
		}
	}
	// Previous is next mirrored about the center.
	next, _ := symbol(layout.IconNext, cx, cy, s)
	prev, _ := symbol(layout.IconPrevious, cx, cy, s)
	if a, b := next.polys[0].pts[1], prev.polys[0].pts[1]; a.X+b.X != 2*cx || a.Y != b.Y {
		t.Errorf("previous %+v is not next %+v mirrored", b, a)
	}
}

func TestSnapshotMediaButtons(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	sq := layout.NodeBase{W: layout.Points(32), H: layout.Points(32)}
	root := func() layout.Node {
		icons := layout.Row{NodeBase: layout.NodeBase{Gap: 1}}
		for _, n := range mediaIcons {
			icons.Children = append(icons.Children, layout.Icon{Name: n, Text: "? "})
		}
		buttons := layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
			layout.Button{NodeBase: sq, Icon: layout.IconShuffle, Label: "⇄", Plain: true},
			layout.Button{NodeBase: sq, Icon: layout.IconPrevious, Label: "⏮", Plain: true},
			layout.Button{NodeBase: sq, Icon: layout.IconPlay, Label: "▶", Default: true},
			layout.Button{NodeBase: sq, Icon: layout.IconNext, Label: "⏭", Plain: true},
			layout.Button{NodeBase: sq, Icon: layout.IconRepeat, Label: "↻", Plain: true},
			layout.Button{Label: "Text"},
		}}
		return layout.Col{NodeBase: layout.NodeBase{Pad: 1, Gap: 1}, Children: []layout.Node{icons, buttons}}
	}
	img, err := Snapshot(snapApp(root), Options{Width: 40, Height: 6}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("FLAT_SNAPSHOT_MEDIA"); path != "" {
		f, _ := os.Create(path)
		defer f.Close()
		png.Encode(f, img)
	}
	if img.Bounds().Dx() == 0 {
		t.Fatal("empty snapshot")
	}
}

func TestDefaultButtonTakesItsStyleColors(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	red := color.NRGBA{0xe0, 0x30, 0x30, 0xff}
	root := func() layout.Node {
		return layout.Col{Children: []layout.Node{layout.Row{Children: []layout.Node{
			layout.Button{NodeBase: layout.NodeBase{W: layout.Points(40), H: layout.Points(40)}, Icon: layout.IconPlay, Label: "▶", Default: true, Style: layout.TextStyle{Bg: red, Fg: color.Black}},
		}}}}
	}
	img, err := Snapshot(snapApp(root), Options{Width: 10, Height: 4}, 1)
	if err != nil {
		t.Fatal(err)
	}
	// Inside the circle, left of the icon: the style's fill, not the accent.
	if got := img.At(6, 20); !near(got, red, 0x18) {
		t.Fatalf("fill at (6,20) = %v, want %v", got, red)
	}
	// The icon, at the circle's center, in the style's Fg.
	if got := img.At(20, 20); !near(got, color.Black, 0x18) {
		t.Fatalf("icon at the center = %v, want black", got)
	}
}

func TestChevronButtonsDrawTheirChevron(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []layout.IconName{layout.IconChevronLeft, layout.IconChevronRight} {
		root := func() layout.Node {
			return layout.Col{Children: []layout.Node{layout.Row{Children: []layout.Node{
				layout.Button{NodeBase: layout.NodeBase{W: layout.Points(32), H: layout.Points(32)}, Icon: name, Label: "‹", Plain: true},
			}}}}
		}
		img, err := Snapshot(snapApp(root), Options{Width: 8, Height: 3}, 1)
		if err != nil {
			t.Fatal(err)
		}
		bg := img.At(1, 1)
		lit := 0
		for y := 8; y < 24; y++ {
			for x := 8; x < 24; x++ {
				if !near(img.At(x, y), bg, 0x20) {
					lit++
				}
			}
		}
		if lit == 0 {
			t.Errorf("icon %d: the button's middle is blank", name)
		}
	}
}
