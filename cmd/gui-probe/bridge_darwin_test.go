package main

import (
	"testing"

	"github.com/ebitengine/purego/objc"
)

// TestStructCallbacksRoundTrip proves the risky part of the binding layer
// without a window: Objective-C methods implemented in Go that take and
// return NSRange/CGRect/CGPoint by value, invoked through the ObjC runtime.
func TestStructCallbacksRoundTrip(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	cls, err := objc.RegisterClass("FlatProbeBridgeTest", objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{
		{Cmd: sel("shiftRange:"), Fn: func(_ objc.ID, _ objc.SEL, r NSRange) NSRange {
			return NSRange{Location: r.Location + 1, Length: r.Length * 2}
		}},
		{Cmd: sel("growRect:by:"), Fn: func(_ objc.ID, _ objc.SEL, r CGRect, d float64) CGRect {
			r.Origin.X -= d
			r.Origin.Y -= d
			r.Size.Width += 2 * d
			r.Size.Height += 2 * d
			return r
		}},
		{Cmd: sel("indexOf:"), Fn: func(_ objc.ID, _ objc.SEL, p CGPoint) uint { return uint(p.X*10 + p.Y) }},
		// Mirrors setMarkedText:selectedRange:replacementRange: — an object
		// followed by two ranges must arrive in order and intact.
		{Cmd: sel("sum:first:second:"), Fn: func(_ objc.ID, _ objc.SEL, o objc.ID, a, b NSRange) uint {
			return uint(o) + a.Location + a.Length*10 + b.Location*100 + b.Length*1000
		}},
		{Cmd: sel("fillActual:actual:"), Fn: func(_ objc.ID, _ objc.SEL, r NSRange, actual *NSRange) bool {
			if actual == nil {
				return false
			}
			*actual = r
			return true
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	obj := objc.ID(cls).Send(sel("new"))
	defer obj.Send(sel("release"))

	if got, want := objc.Send[NSRange](obj, sel("shiftRange:"), NSRange{Location: 4, Length: 3}), (NSRange{Location: 5, Length: 6}); got != want {
		t.Errorf("shiftRange: = %+v, want %+v", got, want)
	}
	in := CGRect{Origin: CGPoint{X: 10, Y: 20}, Size: CGSize{Width: 30, Height: 40}}
	want := CGRect{Origin: CGPoint{X: 8, Y: 18}, Size: CGSize{Width: 34, Height: 44}}
	if got := objc.Send[CGRect](obj, sel("growRect:by:"), in, 2.0); got != want {
		t.Errorf("growRect:by: = %+v, want %+v", got, want)
	}
	if got := objc.Send[uint](obj, sel("indexOf:"), CGPoint{X: 3, Y: 4}); got != 34 {
		t.Errorf("indexOf: = %d, want 34", got)
	}
	if got := objc.Send[uint](obj, sel("sum:first:second:"), objc.ID(7), NSRange{Location: 1, Length: 2}, NSRange{Location: 3, Length: 4}); got != 4328 {
		t.Errorf("sum:first:second: = %d, want 4328", got)
	}
	var actual NSRange
	if !objc.Send[bool](obj, sel("fillActual:actual:"), NSRange{Location: 9, Length: 1}, &actual) || actual != (NSRange{Location: 9, Length: 1}) {
		t.Errorf("fillActual:actual: wrote %+v", actual)
	}
	if objc.Send[bool](obj, sel("fillActual:actual:"), NSRange{}, (*NSRange)(nil)) {
		t.Error("fillActual:actual: with NULL reported a write")
	}
}

// TestViewImplementsTextInputClient checks every required NSTextInputClient
// instance method is implemented, so input methods never hit a missing
// selector at runtime.
func TestViewImplementsTextInputClient(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	cls, err := registerViewClass()
	if err != nil {
		t.Fatal(err)
	}
	proto := objc.GetProtocol("NSTextInputClient")
	if proto == nil {
		t.Fatal("NSTextInputClient protocol not found")
	}
	if !objc.Send[bool](objc.ID(cls), sel("conformsToProtocol:"), proto) {
		t.Error("FlatProbeView does not conform to NSTextInputClient")
	}
	required := proto.CopyMethodDescriptionList(true, true)
	if len(required) == 0 {
		t.Fatal("no required methods reported")
	}
	for _, m := range required {
		if !objc.Send[bool](objc.ID(cls), sel("instancesRespondToSelector:"), sel(m.Name())) {
			t.Errorf("missing required method %s", m.Name())
		}
	}
}

func TestRangeConversion(t *testing.T) {
	for _, tc := range []struct {
		in   NSRange
		want span
	}{
		{NSRange{Location: 2, Length: 3}, span{Loc: 2, Len: 3}},
		{NSRange{Location: nsNotFound}, noSpan},
		{NSRange{Location: nsNotFound + 1}, noSpan},
		{NSRange{Location: 1, Length: ^uint(0)}, span{Loc: 1, Len: int(nsNotFound)}},
	} {
		if got := toSpan(tc.in); got != tc.want {
			t.Errorf("toSpan(%+v) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
	for _, tc := range []struct {
		in   span
		want NSRange
	}{
		{span{Loc: 2, Len: 3}, NSRange{Location: 2, Length: 3}},
		{noSpan, NSRange{Location: nsNotFound}},
		{span{Loc: -1, Len: 2}, NSRange{Location: nsNotFound}},
		{span{Loc: 1, Len: -2}, NSRange{Location: 1}},
	} {
		if got := fromSpan(tc.in); got != tc.want {
			t.Errorf("fromSpan(%+v) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}
