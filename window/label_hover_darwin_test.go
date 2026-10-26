package window

import (
	"image/color"
	"image/png"
	"os"
	"testing"
	"time"

	"github.com/lunguini/flatte/flatui/layout"
)

func TestLabelHoverForegroundFadesWithoutBoxFill(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(100, 0)
	m := metrics()
	accent := color.NRGBA{R: 255, G: 48, B: 164, A: 255}
	s := &testState{root: func(*testState) layout.Node {
		return layout.Label{
			NodeBase: layout.NodeBase{
				ID: "artist", W: layout.Points(70), H: layout.Points(26),
				Box: layout.Box{Interactive: true, Hover: color.Transparent},
			},
			Text: "Artist", Style: layout.TextStyle{Fg: color.White, Size: 20}, HoverFg: accent,
		}
	}}
	h, _ := newTestHost(t, s)
	h.surface.clock = func() time.Time { return now }
	width, height := 80*m.Column, 4*m.Line
	h.start(width, height)
	h.render()
	r := h.scene.Rects["artist"]
	base := renderImage(h.scene, darkTheme, m, width, height, 1, nil, presentation{})
	shape, _ := h.hover(r.X+r.W/2, r.Y+r.H/2)
	if shape != pointerHand || !h.over["artist"] {
		t.Fatalf("transparent interactive label lost hover routing: cursor=%v over=%v", shape, h.over)
	}

	now = now.Add(hoverFade / 2)
	halfPres := h.presentation()
	if level := halfPres.hovers["artist"]; level < 0.49 || level > 0.51 {
		t.Fatalf("hover did not use its fade: %v", halfPres.hovers)
	}
	half := renderImage(h.scene, darkTheme, m, width, height, 1, nil, halfPres)
	now = now.Add(hoverFade / 2)
	fullPres := h.presentation()
	full := renderImage(h.scene, darkTheme, m, width, height, 1, nil, fullPres)

	boxX, boxY := int(r.X+r.W-2), int(r.Y+r.H-2)
	bg := base.At(boxX, boxY)
	if !near(half.At(boxX, boxY), bg, 1) || !near(full.At(boxX, boxY), bg, 1) {
		t.Fatal("transparent interactive hover painted a box fill")
	}
	br, bgc, bb, _ := base.At(4, 13).RGBA()
	_, hg, _, _ := half.At(4, 13).RGBA()
	fr, fgc, fb, _ := full.At(4, 13).RGBA()
	if !(bgc > hg && hg > fgc && bb > fb && fr+1000 >= br) {
		t.Fatalf("label foreground did not fade toward accent: base=%v half=%v full=%v", base.At(4, 13), half.At(4, 13), full.At(4, 13))
	}
	if path := os.Getenv("FLAT_HOVER_PREVIEW"); path != "" {
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, full); err != nil {
			f.Close()
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
