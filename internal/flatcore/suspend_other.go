//go:build !unix

package flatcore

// suspendProcess is a no-op on platforms without job control: a Suspend
// effect degrades to a release/restore round trip (full repaint).
func suspendProcess() {}
