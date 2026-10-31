//go:build darwin && !wasm

package appkit

import (
	"fmt"
	"runtime"
	"structs"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

const (
	frameworkCocoa = "/System/Library/Frameworks/Cocoa.framework/Cocoa"
	libSystem      = "/usr/lib/libSystem.B.dylib"
	libObjC        = "/usr/lib/libobjc.A.dylib"
)

type cgPoint struct {
	_    structs.HostLayout
	X, Y float64
}

var (
	loadOnce  sync.Once
	loadErr   error
	runMu     sync.Mutex
	runActive bool

	dispatchAsyncF      func(queue, ctx, fn uintptr)
	dispatchMainQueue   uintptr
	pthreadMainNP       func() int32
	autoreleasePoolPush func() uintptr
	autoreleasePoolPop  func(pool uintptr)
)

// BeginRun reserves the one process-wide AppKit run slot shared by the window
// host and terminal media host.
func BeginRun() error {
	runMu.Lock()
	defer runMu.Unlock()
	if runActive {
		return fmt.Errorf("window: a window or terminal host is already running")
	}
	runActive = true
	return nil
}

// EndRun releases the process-wide AppKit run slot.
func EndRun() {
	runMu.Lock()
	runActive = false
	runMu.Unlock()
}

func load() error {
	loadOnce.Do(func() {
		libs := make(map[string]uintptr, 3)
		for _, path := range []string{frameworkCocoa, libSystem, libObjC} {
			h, err := purego.Dlopen(path, purego.RTLD_GLOBAL|purego.RTLD_LAZY)
			if err != nil {
				loadErr = fmt.Errorf("appkit: dlopen %s: %w", path, err)
				return
			}
			libs[path] = h
		}
		sys, oc := libs[libSystem], libs[libObjC]
		for _, binding := range []struct {
			fn   any
			lib  uintptr
			name string
		}{
			{&dispatchAsyncF, sys, "dispatch_async_f"},
			{&pthreadMainNP, sys, "pthread_main_np"},
			{&autoreleasePoolPush, oc, "objc_autoreleasePoolPush"},
			{&autoreleasePoolPop, oc, "objc_autoreleasePoolPop"},
		} {
			if _, err := purego.Dlsym(binding.lib, binding.name); err != nil {
				loadErr = fmt.Errorf("appkit: dlsym %s: %w", binding.name, err)
				return
			}
			purego.RegisterLibFunc(binding.fn, binding.lib, binding.name)
		}
		queue, err := purego.Dlsym(sys, "_dispatch_main_q")
		if err != nil {
			loadErr = fmt.Errorf("appkit: dlsym _dispatch_main_q: %w", err)
			return
		}
		dispatchMainQueue = queue
	})
	return loadErr
}

func constant(lib uintptr, name string) (objc.ID, error) {
	addr, err := purego.Dlsym(lib, name)
	if err != nil {
		return 0, fmt.Errorf("appkit: dlsym %s: %w", name, err)
	}
	return **(**objc.ID)(unsafe.Pointer(&addr)), nil
}

func sel(name string) objc.SEL { return objc.RegisterName(name) }

func class(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

func alloc(name string) objc.ID { return class(name).Send(sel("alloc")) }

func nsString(s string) objc.ID {
	return class("NSString").Send(sel("stringWithUTF8String:"), s)
}

func plainString(text objc.ID) string {
	if text == 0 {
		return ""
	}
	if objc.Send[bool](text, sel("isKindOfClass:"), objc.GetClass("NSAttributedString")) {
		text = text.Send(sel("string"))
	}
	return goString(objc.Send[*byte](text, sel("UTF8String")))
}

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

func withPool(f func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := autoreleasePoolPush()
	defer autoreleasePoolPop(pool)
	f()
}
