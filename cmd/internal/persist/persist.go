// Package persist centralizes the sample applications' state replacement
// policy. Missing state is a normal first run; unreadable existing state is
// evidence that must not be overwritten implicitly.
package persist

import (
	"errors"
	"fmt"
	"os"

	"github.com/lunguini/flatte"
)

// Load returns the decoded state and preserves missing/decode errors so Save
// can distinguish a first run from an unreadable existing snapshot.
func Load[S any](path string, defaultState S) (S, error) {
	return flatte.LoadStateWithError(path, defaultState)
}

// Save replaces path unless loadErr reports that an existing snapshot was
// unreadable. A missing file is a normal first save. Refusing the replacement
// leaves the original bytes untouched for recovery or explicit deletion.
func Save[S any](path string, state S, loadErr error) error {
	if loadErr != nil && !errors.Is(loadErr, os.ErrNotExist) {
		return fmt.Errorf("refusing to overwrite unreadable state %q: %w", path, loadErr)
	}
	return flatte.SaveState(path, state)
}
