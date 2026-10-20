package window

import (
	"fmt"
	"math"
	"runtime"
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

func cgRect(x, y, w, h float64) CGRect {
	return CGRect{Origin: CGPoint{X: x, Y: y}, Size: CGSize{Width: w, Height: h}}
}

const (
	frameworkCocoa          = "/System/Library/Frameworks/Cocoa.framework/Cocoa"
	frameworkCoreText       = "/System/Library/Frameworks/CoreText.framework/CoreText"
	frameworkCoreGraphics   = "/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics"
	frameworkCoreFoundation = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
	libSystem               = "/usr/lib/libSystem.B.dylib"
	libObjC                 = "/usr/lib/libobjc.A.dylib"
)

// C functions and constants, bound by loadFrameworks.
var (
	cgContextSaveGState         func(ctx uintptr)
	cgContextRestoreGState      func(ctx uintptr)
	cgContextClipToRect         func(ctx uintptr, r CGRect)
	cgContextSetRGBFillColor    func(ctx uintptr, r, g, b, a float64)
	cgContextSetRGBStrokeColor  func(ctx uintptr, r, g, b, a float64)
	cgContextFillRect           func(ctx uintptr, r CGRect)
	cgContextSetLineWidth       func(ctx uintptr, w float64)
	cgContextAddPath            func(ctx, path uintptr)
	cgContextFillPath           func(ctx uintptr)
	cgContextStrokePath         func(ctx uintptr)
	cgPathCreateWithRoundedRect func(r CGRect, cw, ch float64, t *CGAffineTransform) uintptr
	cgPathRelease               func(path uintptr)
	cgContextSetTextMatrix      func(ctx uintptr, t CGAffineTransform)
	cgContextSetTextPosition    func(ctx uintptr, x, y float64)
	cgContextTranslateCTM       func(ctx uintptr, x, y float64)
	cgContextScaleCTM           func(ctx uintptr, x, y float64)
	cgColorSpaceCreateDeviceRGB func() uintptr
	cgColorSpaceRelease         func(cs uintptr)
	cgBitmapContextCreate       func(data uintptr, w, h, bpc, bpr, cs uintptr, info uint32) uintptr
	cgBitmapContextGetData      func(ctx uintptr) unsafe.Pointer
	cgBitmapContextGetBytesPer  func(ctx uintptr) uintptr
	cgContextRelease            func(ctx uintptr)

	ctFontCreateUIFontForLanguage    func(kind uint32, size float64, lang uintptr) uintptr
	ctFontCreateCopyWithSymbolicTr   func(font uintptr, size float64, m *CGAffineTransform, value, mask uint32) uintptr
	ctFontGetAscent                  func(font uintptr) float64
	ctFontGetDescent                 func(font uintptr) float64
	ctFontGetLeading                 func(font uintptr) float64
	ctFontGetUnderlinePosition       func(font uintptr) float64
	ctFontGetUnderlineThickness      func(font uintptr) float64
	ctFontGetXHeight                 func(font uintptr) float64
	ctLineCreateWithAttributedString func(attr objc.ID) uintptr
	ctLineDraw                       func(line, ctx uintptr)
	ctLineGetTypographicBounds       func(line uintptr, ascent, descent, leading *float64) float64
	ctLineGetOffsetForStringIndex    func(line uintptr, index int, secondary *float64) float64
	cfRelease                        func(ref uintptr)

	dispatchAsyncF       func(queue, ctx, fn uintptr)
	dispatchMainQueue    uintptr
	pthreadMainNP        func() int32
	autoreleasePoolPush  func() uintptr
	autoreleasePoolPop   func(pool uintptr)
	kCTFontAttributeName objc.ID
	kCTForegroundFromCtx objc.ID
)

// CTFontUIFontType values.
const (
	ctFontUserFixedPitch   = 1
	ctFontSystem           = 2
	ctFontEmphasizedSystem = 3
)

// CTFontSymbolicTraits.
const (
	ctTraitItalic = 1 << 0
	ctTraitBold   = 1 << 1
)

var (
	loadOnce sync.Once
	loadErr  error
)

// loadFrameworks dlopens the system frameworks and binds the C functions
// and constants the window uses. It is safe to call more than once.
func loadFrameworks() error {
	loadOnce.Do(func() { loadErr = bindFrameworks() })
	return loadErr
}

func bindFrameworks() error {
	libs := map[string]uintptr{}
	for _, path := range []string{frameworkCocoa, frameworkCoreText, frameworkCoreGraphics, frameworkCoreFoundation, libSystem, libObjC} {
		h, err := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_LAZY)
		if err != nil {
			return fmt.Errorf("window: dlopen %s: %w", path, err)
		}
		libs[path] = h
	}
	cg, ct, cf, sys, oc := libs[frameworkCoreGraphics], libs[frameworkCoreText], libs[frameworkCoreFoundation], libs[libSystem], libs[libObjC]
	for _, b := range []struct {
		fn   any
		lib  uintptr
		name string
	}{
		{&cgContextSaveGState, cg, "CGContextSaveGState"},
		{&cgContextRestoreGState, cg, "CGContextRestoreGState"},
		{&cgContextClipToRect, cg, "CGContextClipToRect"},
		{&cgContextSetRGBFillColor, cg, "CGContextSetRGBFillColor"},
		{&cgContextSetRGBStrokeColor, cg, "CGContextSetRGBStrokeColor"},
		{&cgContextFillRect, cg, "CGContextFillRect"},
		{&cgContextSetLineWidth, cg, "CGContextSetLineWidth"},
		{&cgContextAddPath, cg, "CGContextAddPath"},
		{&cgContextFillPath, cg, "CGContextFillPath"},
		{&cgContextStrokePath, cg, "CGContextStrokePath"},
		{&cgPathCreateWithRoundedRect, cg, "CGPathCreateWithRoundedRect"},
		{&cgPathRelease, cg, "CGPathRelease"},
		{&cgContextSetTextMatrix, cg, "CGContextSetTextMatrix"},
		{&cgContextSetTextPosition, cg, "CGContextSetTextPosition"},
		{&cgContextTranslateCTM, cg, "CGContextTranslateCTM"},
		{&cgContextScaleCTM, cg, "CGContextScaleCTM"},
		{&cgColorSpaceCreateDeviceRGB, cg, "CGColorSpaceCreateDeviceRGB"},
		{&cgColorSpaceRelease, cg, "CGColorSpaceRelease"},
		{&cgBitmapContextCreate, cg, "CGBitmapContextCreate"},
		{&cgBitmapContextGetData, cg, "CGBitmapContextGetData"},
		{&cgBitmapContextGetBytesPer, cg, "CGBitmapContextGetBytesPerRow"},
		{&cgContextRelease, cg, "CGContextRelease"},
		{&ctFontCreateUIFontForLanguage, ct, "CTFontCreateUIFontForLanguage"},
		{&ctFontCreateCopyWithSymbolicTr, ct, "CTFontCreateCopyWithSymbolicTraits"},
		{&ctFontGetAscent, ct, "CTFontGetAscent"},
		{&ctFontGetDescent, ct, "CTFontGetDescent"},
		{&ctFontGetLeading, ct, "CTFontGetLeading"},
		{&ctFontGetUnderlinePosition, ct, "CTFontGetUnderlinePosition"},
		{&ctFontGetUnderlineThickness, ct, "CTFontGetUnderlineThickness"},
		{&ctFontGetXHeight, ct, "CTFontGetXHeight"},
		{&ctLineCreateWithAttributedString, ct, "CTLineCreateWithAttributedString"},
		{&ctLineDraw, ct, "CTLineDraw"},
		{&ctLineGetTypographicBounds, ct, "CTLineGetTypographicBounds"},
		{&ctLineGetOffsetForStringIndex, ct, "CTLineGetOffsetForStringIndex"},
		{&cfRelease, cf, "CFRelease"},
		{&dispatchAsyncF, sys, "dispatch_async_f"},
		{&pthreadMainNP, sys, "pthread_main_np"},
		{&autoreleasePoolPush, oc, "objc_autoreleasePoolPush"},
		{&autoreleasePoolPop, oc, "objc_autoreleasePoolPop"},
	} {
		if _, err := purego.Dlsym(b.lib, b.name); err != nil {
			return fmt.Errorf("window: dlsym %s: %w", b.name, err)
		}
		purego.RegisterLibFunc(b.fn, b.lib, b.name)
	}
	q, err := purego.Dlsym(sys, "_dispatch_main_q")
	if err != nil {
		return fmt.Errorf("window: dlsym _dispatch_main_q: %w", err)
	}
	dispatchMainQueue = q
	for name, dst := range map[string]*objc.ID{
		"kCTFontAttributeName":                       &kCTFontAttributeName,
		"kCTForegroundColorFromContextAttributeName": &kCTForegroundFromCtx,
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
		return 0, fmt.Errorf("window: dlsym %s: %w", name, err)
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

// withPool runs f inside an autorelease pool, so objects AppKit and
// Foundation autorelease on this thread are freed when f returns. Pools are
// per thread, so the goroutine stays on one thread from push to pop (the
// lock nests with the main thread's).
func withPool(f func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := autoreleasePoolPush()
	defer autoreleasePoolPop(pool)
	f()
}
