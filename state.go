package flatte

import (
	"encoding/gob"
	"os"
)

// SaveState gob-encodes state to path. The state struct must contain only
// gob-serializable fields — paths not open file handles, queries not DB
// connections. Anything live gets reopened on boot via a rehydrate step.
func SaveState[S any](path string, state S) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewEncoder(f).Encode(state)
}

// LoadState gob-decodes state from path, returning defaultState if the file
// is missing or fails to decode. Struct shape changes during iteration will
// invalidate old state — the decode-error fallback handles this gracefully
// instead of crashing.
func LoadState[S any](path string, defaultState S) S {
	f, err := os.Open(path)
	if err != nil {
		return defaultState
	}
	defer f.Close()
	var state S
	if err := gob.NewDecoder(f).Decode(&state); err != nil {
		return defaultState
	}
	return state
}
