package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/ebitengine/purego/objc"
)

// probeView is the Go side of the single FlatProbeView instance. AppKit
// calls back with self; the probe has exactly one view, so a package-level
// value is enough.
type probeView struct {
	buf     editBuffer
	verbose bool
}

var theView = &probeView{}

func (v *probeView) logf(format string, args ...any) {
	if v.verbose {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func (v *probeView) changed(self objc.ID) {
	v.logf("  text=%q marked=%+v selected=%+v", v.buf.Text(), v.buf.MarkedRange(), v.buf.SelectedRange())
	self.Send(sel("setNeedsDisplay:"), true)
}

var (
	viewClassOnce sync.Once
	viewClass     objc.Class
	viewClassErr  error
)

// registerViewClass defines FlatProbeView, an NSView subclass that draws
// with CoreText and implements NSTextInputClient. Several of its methods
// take or return NSRange/NSRect by value, which is what the probe checks.
func registerViewClass() (objc.Class, error) {
	viewClassOnce.Do(func() {
		viewClass, viewClassErr = objc.RegisterClass(
			"FlatProbeView",
			objc.GetClass("NSView"),
			[]*objc.Protocol{objc.GetProtocol("NSTextInputClient")},
			nil,
			viewMethods(),
		)
	})
	return viewClass, viewClassErr
}

func viewMethods() []objc.MethodDef {
	v := theView
	return []objc.MethodDef{
		{Cmd: sel("isFlipped"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("acceptsFirstResponder"), Fn: func(objc.ID, objc.SEL) bool { return true }},
		{Cmd: sel("drawRect:"), Fn: func(self objc.ID, _ objc.SEL, _ CGRect) {
			ctx := class("NSGraphicsContext").Send(sel("currentContext")).Send(sel("CGContext"))
			drawFrame(uintptr(ctx), objc.Send[CGRect](self, sel("bounds")), &v.buf)
		}},
		{Cmd: sel("keyDown:"), Fn: func(self objc.ID, _ objc.SEL, event objc.ID) {
			// Route through the input context so input methods see the key.
			self.Send(sel("interpretKeyEvents:"), class("NSArray").Send(sel("arrayWithObject:"), event))
		}},

		// NSTextInputClient.
		{Cmd: sel("hasMarkedText"), Fn: func(objc.ID, objc.SEL) bool { return v.buf.HasMarked() }},
		{Cmd: sel("markedRange"), Fn: func(objc.ID, objc.SEL) NSRange { return fromSpan(v.buf.MarkedRange()) }},
		{Cmd: sel("selectedRange"), Fn: func(objc.ID, objc.SEL) NSRange { return fromSpan(v.buf.SelectedRange()) }},
		{Cmd: sel("setMarkedText:selectedRange:replacementRange:"), Fn: func(self objc.ID, _ objc.SEL, text objc.ID, selected, replace NSRange) {
			s := plainString(text)
			v.logf("setMarkedText %q selected=%+v replace=%+v", s, toSpan(selected), toSpan(replace))
			v.buf.SetMarked(s, toSpan(selected), toSpan(replace))
			v.changed(self)
		}},
		{Cmd: sel("unmarkText"), Fn: func(self objc.ID, _ objc.SEL) {
			v.logf("unmarkText")
			v.buf.Unmark()
			v.changed(self)
		}},
		{Cmd: sel("validAttributesForMarkedText"), Fn: func(objc.ID, objc.SEL) objc.ID {
			return class("NSArray").Send(sel("array"))
		}},
		{Cmd: sel("attributedSubstringForProposedRange:actualRange:"), Fn: func(objc.ID, objc.SEL, NSRange, *NSRange) objc.ID {
			return 0 // reconversion is out of scope
		}},
		{Cmd: sel("insertText:replacementRange:"), Fn: func(self objc.ID, _ objc.SEL, text objc.ID, replace NSRange) {
			s := plainString(text)
			v.logf("insertText %q replace=%+v", s, toSpan(replace))
			v.buf.Insert(s, toSpan(replace))
			v.changed(self)
		}},
		{Cmd: sel("characterIndexForPoint:"), Fn: func(objc.ID, objc.SEL, CGPoint) uint { return nsNotFound }},
		{Cmd: sel("firstRectForCharacterRange:actualRange:"), Fn: func(self objc.ID, _ objc.SEL, r NSRange, actual *NSRange) CGRect {
			if actual != nil {
				*actual = r
			}
			rect := objc.Send[CGRect](self, sel("convertRect:toView:"), caretRect(&v.buf), objc.ID(0))
			if win := self.Send(sel("window")); win != 0 {
				rect = objc.Send[CGRect](win, sel("convertRectToScreen:"), rect)
			}
			v.logf("firstRect %+v -> %+v", toSpan(r), rect)
			return rect
		}},
		{Cmd: sel("doCommandBySelector:"), Fn: func(self objc.ID, _ objc.SEL, cmd objc.SEL) {
			switch cmd {
			case sel("deleteBackward:"):
				v.buf.DeleteBackward()
				v.changed(self)
			case sel("cancelOperation:"):
				class("NSApplication").Send(sel("sharedApplication")).Send(sel("terminate:"), objc.ID(0))
			}
		}},
	}
}
