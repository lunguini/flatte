//go:build !unix && !windows

package flatte

// Platforms without a portable directory fsync still receive the file-level
// sync before rename. Unix implementations additionally persist the directory
// entry in state_syncdir_unix.go.
func syncStateDir(string) error { return nil }
