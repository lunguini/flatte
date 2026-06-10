//go:build !unix

package flatcore

import "os"

// notifyResize is a no-op on platforms without SIGWINCH; the returned nil
// channel blocks forever in the select loop.
func notifyResize() (<-chan os.Signal, func()) {
	return nil, func() {}
}
