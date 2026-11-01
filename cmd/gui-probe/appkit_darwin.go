package main

import (
	"fmt"
	"math"
	"structs"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// Geometry mirrors the CoreGraphics structs. purego derives Objective-C type
// encodings from Go type names, so the names match the C struct tags.
type CGPoint struct {
	_    structs.HostLayout
	X, Y float64
}

type CGSize struct {
	_             structs.HostLayout
	Width, Height float64
}

type CGRect struct {
	_      structs.HostLayout
	Origin CGPoint
	Size   CGSize
}

type CGAffineTransform struct {
	_                structs.HostLayout
	A, B, C, D, X, Y float64
}

// _NSRange is named after its C struct tag for the same encoding reason.
type _NSRange struct {
	_                structs.HostLayout
	Location, Length uint
}

type NSRange = _NSRange

// nsNotFound is NSNotFound (NSIntegerMax).
const nsNotFound = uint(math.MaxInt64)

func toSpan(r NSRange) span {
	if r.Location >= nsNotFound {
		return noSpan
	}
	return span{Loc: int(r.Location), Len: int(min(r.Length, uint(math.MaxInt)))}
}

func fromSpan(s span) NSRange {
	if s.Missing || s.Loc < 0 {
		return NSRange{Location: nsNotFound}
	}
	return NSRange{Location: uint(s.Loc), Length: uint(max(s.Len, 0))}
}

const (
	frameworkCocoa          = "/System/Library/Frameworks/Cocoa.framework/Cocoa"
	frameworkCoreText       = "/System/Library/Frameworks/CoreText.framework/CoreText"
	frameworkCoreGraphics   = "/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics"
	frameworkCoreFoundation = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
)

var (
	loadOnce sync.Once
	loadErr  error
)

// loadFrameworks dlopens the system frameworks and binds the C functions
// and constants the probe uses. It is safe to call more than once.
func loadFrameworks() error {
	loadOnce.Do(func() { loadErr = bindFrameworks() })
	return loadErr
}

func bindFrameworks() error {
	libs := map[string]uintptr{}
	for _, path := range []string{frameworkCocoa, frameworkCoreText, frameworkCoreGraphics, frameworkCoreFoundation} {
		h, err := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_LAZY)
		if err != nil {
			return fmt.Errorf("dlopen %s: %w", path, err)
		}
		libs[path] = h
	}
	cg, ct, cf := libs[frameworkCoreGraphics], libs[frameworkCoreText], libs[frameworkCoreFoundation]
	purego.RegisterLibFunc(&cgContextSetRGBFillColor, cg, "CGContextSetRGBFillColor")
	purego.RegisterLibFunc(&cgContextFillRect, cg, "CGContextFillRect")
	purego.RegisterLibFunc(&cgContextSetTextMatrix, cg, "CGContextSetTextMatrix")
	purego.RegisterLibFunc(&cgContextSetTextPosition, cg, "CGContextSetTextPosition")
	purego.RegisterLibFunc(&ctLineCreateWithAttributedString, ct, "CTLineCreateWithAttributedString")
	purego.RegisterLibFunc(&ctLineDraw, ct, "CTLineDraw")
	purego.RegisterLibFunc(&ctLineGetTypographicBounds, ct, "CTLineGetTypographicBounds")
	purego.RegisterLibFunc(&ctLineGetOffsetForStringIndex, ct, "CTLineGetOffsetForStringIndex")
	purego.RegisterLibFunc(&cfRelease, cf, "CFRelease")
	for name, dst := range map[string]*objc.ID{
		"kCTFontAttributeName":                       &kCTFontAttributeName,
		"kCTForegroundColorFromContextAttributeName": &kCTForegroundColorFromContextAttributeName,
		"kCTUnderlineStyleAttributeName":             &kCTUnderlineStyleAttributeName,
	} {
		v, err := constant(ct, name)
		if err != nil {
			return err
		}
		*dst = v
	}
	return nil
}

// constant reads an exported `CFStringRef const` symbol.
func constant(lib uintptr, name string) (objc.ID, error) {
	addr, err := purego.Dlsym(lib, name)
	if err != nil {
		return 0, fmt.Errorf("dlsym %s: %w", name, err)
	}
	// Reinterpret the address word as a pointer without a uintptr ->
	// unsafe.Pointer conversion; the symbol lives in a system image.
	return **(**objc.ID)(unsafe.Pointer(&addr)), nil
}

func sel(name string) objc.SEL { return objc.RegisterName(name) }

func class(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

func alloc(name string) objc.ID { return class(name).Send(sel("alloc")) }

// nsString returns an autoreleased NSString copy of s.
func nsString(s string) objc.ID {
	return class("NSString").Send(sel("stringWithUTF8String:"), s)
}

// plainString unwraps the NSString or NSAttributedString AppKit passes to
// the text-input methods.
func plainString(text objc.ID) string {
	if text == 0 {
		return ""
	}
	if objc.Send[bool](text, sel("isKindOfClass:"), objc.GetClass("NSAttributedString")) {
		text = text.Send(sel("string"))
	}
	return goString(objc.Send[*byte](text, sel("UTF8String")))
}

// goString copies a NUL-terminated C string.
func goString(p *byte) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*byte)(unsafe.Add(unsafe.Pointer(p), n)) != 0 {
		n++
	}
	return string(unsafe.Slice(p, n))
}
