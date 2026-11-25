package window

import "github.com/ebitengine/purego/objc"

const (
	nsWindowToolbarStyleUnified        = 3 // 52 points
	nsWindowToolbarStyleUnifiedCompact = 4 // 40 points, 38 before macOS 26
	nsTitlebarSeparatorStyleNone       = 1

	// unifiedStrip is the strip height from which the unified toolbar's
	// bar is the nearer one.
	unifiedStrip = 46

	nsPresentationAutoHideMenuBar = 1 << 2
	nsPresentationHideMenuBar     = 1 << 3
	nsPresentationFullScreen      = 1 << 10
	nsPresentationAutoHideToolbar = 1 << 11
)

// titleBarPoints is the height of a HideTitleBar window's strip: the
// app's, or the system's when the app names none.
func titleBarPoints(o Options) float64 {
	if o.TitleBarHeight > TitleBarHeight {
		return float64(o.TitleBarHeight)
	}
	return TitleBarHeight
}

// tallTitleBar gives a HideTitleBar window whose strip is h points tall the
// system's title bar nearest that height, by an empty toolbar: AppKit then
// lays the window buttons out in the middle of it itself, always. (Moving
// the buttons instead would have them jump: AppKit puts them back on every
// layout of the title bar, until the next move.)
func tallTitleBar(win objc.ID, h float64) {
	if h <= TitleBarHeight {
		return
	}
	bar := alloc("NSToolbar").Send(sel("initWithIdentifier:"), nsString("flatte.titlebar"))
	style := nsWindowToolbarStyleUnifiedCompact
	if h >= unifiedStrip {
		style = nsWindowToolbarStyleUnified
	}
	win.Send(sel("setToolbarStyle:"), style)
	win.Send(sel("setToolbar:"), bar)
	win.Send(sel("setTitlebarSeparatorStyle:"), nsTitlebarSeparatorStyleNone)
	bar.Send(sel("release"))
}

// fullScreenPresentation is the full-screen presentation for what AppKit
// proposes: a toolbar (tallTitleBar's, empty) shows with the menu bar only,
// not over the app's own strip.
func fullScreenPresentation(proposed uint) uint {
	if proposed&nsPresentationFullScreen == 0 || proposed&(nsPresentationAutoHideMenuBar|nsPresentationHideMenuBar) == 0 {
		return proposed // AppKit refuses a hiding toolbar without these
	}
	return proposed | nsPresentationAutoHideToolbar
}
