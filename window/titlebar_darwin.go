package window

import (
	"math"

	"github.com/ebitengine/purego/objc"
)

// nsWindowStyleMaskFullScreen is NSWindowStyleMaskFullScreen.
const nsWindowStyleMaskFullScreen = 1 << 14

// titleBarPoints is the height of a HideTitleBar window's strip: the
// app's, or the system's when the app names none.
func titleBarPoints(o Options) float64 {
	if o.TitleBarHeight > TitleBarHeight {
		return float64(o.TitleBarHeight)
	}
	return TitleBarHeight
}

// placeTitleButtons centers the window buttons vertically in a strip h
// points tall (taller than the system's title bar), as a browser's are in
// its tab strip: the buttons' container grows to h and each button sits in
// its middle. AppKit lays the title bar out again on resize and on leaving
// full screen, so this runs before every draw and moves only what moved;
// in full screen the system's own title bar is left alone.
func placeTitleButtons(win objc.ID, h float64) {
	if h <= TitleBarHeight || objc.Send[uint](win, sel("styleMask"))&nsWindowStyleMaskFullScreen != 0 {
		return
	}
	closeButton := win.Send(sel("standardWindowButton:"), 0)
	if closeButton == 0 {
		return
	}
	bar := closeButton.Send(sel("superview"))
	container := bar.Send(sel("superview"))
	if bar == 0 || container == 0 {
		return
	}
	wf := objc.Send[CGRect](win, sel("frame"))
	cf := objc.Send[CGRect](container, sel("frame"))
	if want := cgRect(cf.Origin.X, wf.Size.Height-h, cf.Size.Width, h); !sameRect(cf, want) {
		container.Send(sel("setFrame:"), want)
	}
	if bf := objc.Send[CGRect](bar, sel("frame")); !sameRect(bf, cgRect(0, 0, cf.Size.Width, h)) {
		bar.Send(sel("setFrame:"), cgRect(0, 0, cf.Size.Width, h))
	}
	for i := range 3 { // close, minimize, zoom
		b := win.Send(sel("standardWindowButton:"), i)
		if b == 0 {
			continue
		}
		f := objc.Send[CGRect](b, sel("frame"))
		y := math.Round((h - f.Size.Height) / 2)
		if f.Origin.Y != y {
			b.Send(sel("setFrameOrigin:"), CGPoint{X: f.Origin.X, Y: y})
		}
	}
}

func sameRect(a, b CGRect) bool {
	return a.Origin == b.Origin && a.Size == b.Size
}
