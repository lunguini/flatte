package flatte

import (
	"encoding/gob"
	"errors"
	"os"
	"path/filepath"
)

// SaveState gob-encodes state to path. The state struct must contain only
// gob-serializable fields — paths not open file handles, queries not DB
// connections. Anything live gets reopened on boot via a rehydrate step.
//
// The file is written and synced beside the destination, then atomically
// renamed into place. This keeps a previously valid state file intact when
// encoding or writing the new state fails (for example, when a state gains an
// unsupported gob field).
func SaveState[S any](path string, state S) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	f, err := os.CreateTemp(dir, "."+base+".tmp-")
	if err != nil {
		return err
	}
	tmpPath := f.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			_ = os.Remove(tmpPath)
		}
	}()

	encodeErr := gob.NewEncoder(f).Encode(state)
	var syncErr error
	if encodeErr == nil {
		syncErr = f.Sync()
	}
	closeErr := f.Close()
	if err := errors.Join(encodeErr, syncErr, closeErr); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return err
	}
	removeTemp = false
	return syncStateDir(dir)
}

// LoadState gob-decodes state from path, returning defaultState if the file
// is missing or fails to decode. Struct shape changes during iteration will
// invalidate old state — the decode-error fallback handles this gracefully
// instead of crashing.
func LoadState[S any](path string, defaultState S) S {
	state, err := LoadStateWithError(path, defaultState)
	if err != nil {
		return defaultState
	}
	return state
}

// LoadStateWithError gob-decodes state from path while preserving I/O and
// decode errors for callers that need to distinguish a missing or corrupt
// state file from a valid file containing the zero value. The compatibility
// LoadState helper above intentionally retains its default-on-error behavior.
func LoadStateWithError[S any](path string, defaultState S) (S, error) {
	f, err := os.Open(path)
	if err != nil {
		return defaultState, err
	}
	var state S
	decodeErr := gob.NewDecoder(f).Decode(&state)
	closeErr := f.Close()
	if err := errors.Join(decodeErr, closeErr); err != nil {
		return defaultState, err
	}
	return state, nil
}
