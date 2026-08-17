//go:build windows

package flatte

// Windows does not provide a portable directory fsync operation through the
// os package. The file itself is still flushed before the atomic rename.
func syncStateDir(string) error { return nil }
