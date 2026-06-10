//go:build unix

package flatcore

import (
	"os"
	"os/signal"
	"syscall"
)

// notifyResize delivers terminal size-change notifications on the returned
// channel until stop is called.
func notifyResize() (<-chan os.Signal, func()) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	return ch, func() { signal.Stop(ch) }
}
