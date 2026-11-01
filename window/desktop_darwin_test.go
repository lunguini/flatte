package window

import (
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"
	"github.com/lunguini/flatte"
)

func TestDesktopNativeServicesProviderABI(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	if err := loadDesktopNative(); err != nil {
		t.Fatal(err)
	}
	var events []flatte.Event
	withPool(func() {
		target := objc.ID(desktopNative.target).Send(sel("new"))
		defer target.Send(sel("release"))
		d := &nativeDesktop{target: target, send: func(ev flatte.Event) { events = append(events, ev) }}
		desktopCurrent = d
		defer func() { desktopCurrent = nil }()
		if !objc.Send[bool](target, sel("respondsToSelector:"), sel("receiveText:userData:error:")) {
			t.Fatal("provider selector absent")
		}
		// A named private pasteboard never changes the user's clipboard.
		board := pasteboard{name: "flatte-test-desktop-service"}
		defer board.board().Send(sel("releaseGlobally"))
		text := "harmless exact selection\n日本語 <literal>"
		board.set(text)
		var nativeError objc.ID
		target.Send(sel("receiveText:userData:error:"), board.board(), nsString("capture"), &nativeError)
		if nativeError != 0 {
			t.Fatalf("service error %s", plainString(nativeError))
		}
		if len(events) != 0 {
			t.Fatal("service mutated app inside native callback")
		}
		runPosted()
		if len(events) != 1 || events[0] != (flatte.DesktopEvent{ID: "capture", Source: "service", Text: text}) {
			t.Fatalf("events %#v", events)
		}
		events = nil
		board.set("")
		target.Send(sel("receiveText:userData:error:"), board.board(), nsString("capture"), &nativeError)
		runPosted()
		if nativeError == 0 || len(events) != 1 || events[0].(flatte.DesktopEvent).Error == "" || events[0].(flatte.DesktopEvent).Text != "" {
			t.Fatalf("empty selection error missing: %#v", events)
		}
		events = nil
		board.set(strings.Repeat("x", maxDesktopSelection+1))
		target.Send(sel("receiveText:userData:error:"), board.board(), nsString("capture"), &nativeError)
		runPosted()
		if len(events) != 1 || events[0].(flatte.DesktopEvent).Error == "" || events[0].(flatte.DesktopEvent).Text != "" {
			t.Fatalf("large selection accepted")
		}
	})
}
