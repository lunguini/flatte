package window

import (
	"crypto/sha256"
	"errors"
	"sync"
)

// registered are the font files this process has registered, by hash:
// CoreText refuses a second registration, and a Snapshot per test would
// make one.
var registered = struct {
	sync.Mutex
	m map[[32]byte]bool
}{m: map[[32]byte]bool{}}

// registerFonts makes each font file's faces usable by family name for the
// rest of the process.
func registerFonts(files [][]byte) error {
	registered.Lock()
	defer registered.Unlock()
	for _, data := range files {
		sum := sha256.Sum256(data)
		if registered.m[sum] {
			continue
		}
		if len(data) == 0 {
			return errors.New("window: an empty font file")
		}
		cfData := cfDataCreate(0, &data[0], len(data)) // CFData copies the bytes
		provider := cgDataProviderCreateWithCFData(cfData)
		cfRelease(cfData)
		f := cgFontCreateWithDataProvider(provider)
		cfRelease(provider)
		if f == 0 {
			return errors.New("window: a font file CoreGraphics cannot read")
		}
		// False when the face is already known (installed, say): its
		// family is usable either way.
		var cfErr uintptr
		ctFontManagerRegisterGraphicsFnt(f, &cfErr)
		cfRelease(f)
		if cfErr != 0 {
			cfRelease(cfErr)
		}
		registered.m[sum] = true
	}
	return nil
}
