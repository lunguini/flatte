package window

import (
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/lunguini/flatte/flatui/layout"
)

// A window draws in sRGB whatever screen it is on, so its colors neither
// differ between screens nor change as it moves: opaque, translucent, or
// switched between the two.
func TestWindowsDrawInSRGB(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	for _, translucent := range []bool{false, true} {
		onMain(func() {
			class("NSApplication").Send(sel("sharedApplication"))
			cls, err := registerViewClass()
			if err != nil {
				t.Error(err)
				return
			}
			win, view := makeWindow(cls, layout.Metrics{Column: 8, Line: 20}, "one", 100, 30, 20, 4, "", chrome{translucent: translucent})
			defer win.Send(sel("release"))
			defer view.Send(sel("release"))
			srgb := class("NSColorSpace").Send(sel("sRGBColorSpace"))
			check := func(when string) {
				if cs := win.Send(sel("colorSpace")); !objc.Send[bool](cs, sel("isEqual:"), srgb) {
					t.Errorf("translucent %v, %s: %s", translucent, when, plainString(cs.Send(sel("localizedName"))))
				}
				rep := view.Send(sel("bitmapImageRepForCachingDisplayInRect:"), cgRect(0, 0, 10, 10))
				if cs := rep.Send(sel("colorSpace")); !objc.Send[bool](cs, sel("isEqual:"), srgb) {
					t.Errorf("translucent %v, %s: the view draws in %s", translucent, when, plainString(cs.Send(sel("localizedName"))))
				}
			}
			check("new")
			screens := class("NSScreen").Send(sel("screens"))
			for i := range objc.Send[int](screens, sel("count")) {
				f := objc.Send[CGRect](screens.Send(sel("objectAtIndex:"), i), sel("frame"))
				win.Send(sel("setFrameOrigin:"), CGPoint{X: f.Origin.X + 50, Y: f.Origin.Y + 50})
				check("on another screen")
			}
			if translucent {
				takeOut(win, view)
			} else {
				effect := newEffect(100, 100)
				putBehind(win, view, effect, true)
				effect.Send(sel("release"))
			}
			check("switched")
		})
	}
}
