package window

import (
	"os"
	"runtime"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte/flatui/layout"
)

// mainThread runs a function on the process's main thread, where AppKit
// makes windows.
var mainThread = make(chan func())

func init() { runtime.LockOSThread() }

func TestMain(m *testing.M) {
	code := make(chan int, 1)
	go func() { code <- m.Run() }()
	for {
		select {
		case f := <-mainThread:
			f()
		case c := <-code:
			os.Exit(c)
		}
	}
}

func onMain(f func()) {
	done := make(chan struct{})
	mainThread <- func() { defer close(done); f() }
	<-done
}

func TestTitleBarPoints(t *testing.T) {
	for _, c := range []struct {
		h    int
		want float64
	}{{0, TitleBarHeight}, {20, TitleBarHeight}, {TitleBarHeight, TitleBarHeight}, {38, 38}, {52, 52}} {
		if got := titleBarPoints(Options{HideTitleBar: true, TitleBarHeight: c.h}); got != c.want {
			t.Errorf("TitleBarHeight %d: strip %v, want %v", c.h, got, c.want)
		}
	}
}

// The window buttons are where AppKit itself lays them out: in the middle of
// a tall strip from the start, and after every layout of the title bar, with
// no draw in between to put them back. The strip stays the app's to click.
func TestWindowButtonsStayPut(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	for _, h := range []float64{40, 52} {
		onMain(func() {
			class("NSApplication").Send(sel("sharedApplication"))
			cls, err := registerViewClass()
			if err != nil {
				t.Error(err)
				return
			}
			win, view := makeWindow(cls, layout.Metrics{Column: 8, Line: 20}, "one", 100, 30, 20, 4, "", chrome{hideTitle: true, titleH: h})
			defer win.Send(sel("release"))
			defer view.Send(sel("release"))
			place := func() (x, middle float64) {
				win.Send(sel("layoutIfNeeded"))
				b := win.Send(sel("standardWindowButton:"), 0)
				wf := objc.Send[CGRect](win, sel("frame"))
				f := objc.Send[CGRect](b, sel("frame"))
				f.Origin = objc.Send[CGPoint](b.Send(sel("superview")), sel("convertPoint:toView:"), f.Origin, objc.ID(0))
				return f.Origin.X, wf.Size.Height - f.Origin.Y - f.Size.Height/2
			}
			x, mid := place()
			if mid < h/2-1.5 || mid > h/2+1.5 {
				t.Errorf("strip %v: buttons' middle at %v", h, mid)
			}
			for name, change := range map[string]func(){
				"resize": func() { win.Send(sel("setFrame:display:"), cgRect(100, 100, 700, 500), false) },
				"title":  func() { win.Send(sel("setTitle:"), nsString("another")) },
				"edited": func() { win.Send(sel("setDocumentEdited:"), true) },
			} {
				change()
				if gx, gm := place(); gx != x || gm != mid {
					t.Errorf("strip %v: after %s at %v, %v; was %v, %v", h, name, gx, gm, x, mid)
				}
			}
			wf := objc.Send[CGRect](win, sel("frame"))
			if hit := view.Send(sel("superview")).Send(sel("hitTest:"), CGPoint{X: wf.Size.Width / 2, Y: wf.Size.Height - h/2}); hit != view {
				t.Errorf("strip %v: the toolbar takes the strip's clicks", h)
			}
		})
	}
}

func TestFullScreenPresentation(t *testing.T) {
	full := uint(nsPresentationFullScreen | nsPresentationAutoHideMenuBar)
	for proposed, want := range map[uint]uint{
		full:                     full | nsPresentationAutoHideToolbar,
		nsPresentationFullScreen: nsPresentationFullScreen,
		0:                        0,
	} {
		if got := fullScreenPresentation(proposed); got != want {
			t.Errorf("%#x: %#x, want %#x", proposed, got, want)
		}
	}
}
