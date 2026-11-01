//go:build unix

package flatte

import "os"

// syncStateDir persists the rename itself on filesystems that expose
// directory fsync. The temporary file is synced before rename; syncing the
// containing directory closes the remaining durability window.
func syncStateDir(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
