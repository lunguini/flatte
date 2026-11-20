package window

import (
	"context"
	"image/png"
	"os"
	"testing"
	"time"
)

// The tooltip draws as a raised bubble with readable text in a real
// AppKit render. FLAT_TIP_PNG saves the picture.
func TestTooltipDrawsInAppKit(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	m := metrics()
	b := &fakeBackend{}
	h := newHost(context.Background(), testApp(controlsState()), m, b.post, func() {}, func() {})
	t.Cleanup(func() { h.cancel(); h.session.Cancel() })
	now := time.Unix(1000, 0)
	h.clock = func() time.Time { return now }
	h.start(300, 160)
	h.render()
	r := h.scene.Rects["ok"]
	h.hover(r.X+4, r.Y+4)
	now = now.Add(tipDelay)
	p := h.presentation()
	n := len(h.scene.Ops)
	drawn := h.withTip(h.scene, p)
	op, ok := tipText(drawn, n)
	if !ok {
		t.Fatal("no tooltip")
	}
	img := renderImage(drawn, darkTheme, m, 300, 160, 2, nil, p)
	if path := os.Getenv("FLAT_TIP_PNG"); path != "" {
		f, _ := os.Create(path)
		png.Encode(f, img)
		f.Close()
	}
	bg := toColor(darkTheme.bg)
	// Just right of the text, inside the bubble: the raised surface.
	surface := img.At(int((op.Rect.X+op.Rect.W+2)*2), int((op.Rect.Y+op.Rect.H/2)*2))
	if near(surface, bg, 3) {
		t.Errorf("bubble %v reads as the window background %v", surface, bg)
	}
}
