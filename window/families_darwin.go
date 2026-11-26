package window

import (
	"strings"

	"github.com/ebitengine/purego/objc"
)

// fontFamilies asks CoreText for the installed families, and each one's
// regular face whether it is fixed-pitch. Hidden system families (their
// names start with a dot) are left out.
func fontFamilies() []FontFamily {
	if loadFrameworks() != nil {
		return nil
	}
	var fams []FontFamily
	withPool(func() {
		names := objc.ID(ctFontManagerCopyFamilyNames()) // a CFArray of CFStrings, toll-free an NSArray
		if names == 0 {
			return
		}
		defer cfRelease(uintptr(names))
		n := objc.Send[int](names, sel("count"))
		fams = make([]FontFamily, 0, n)
		for i := range n {
			name := names.Send(sel("objectAtIndex:"), i)
			s := goString(objc.Send[*byte](name, sel("UTF8String")))
			if s == "" || strings.HasPrefix(s, ".") {
				continue
			}
			f := FontFamily{Name: s}
			if ref := ctFontCreateWithName(uintptr(name), 12, nil); ref != 0 {
				f.Monospace = ctFontGetSymbolicTraits(ref)&ctTraitMono != 0
				cfRelease(ref)
			}
			fams = append(fams, f)
		}
	})
	return fams
}
