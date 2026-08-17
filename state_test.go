package flatte

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSaveAndLoadState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.gob")

	type state struct {
		Name   string
		Cursor int
		Items  []string
	}

	original := state{Name: "test", Cursor: 7, Items: []string{"a", "b", "c"}}
	if err := SaveState(path, original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded := LoadState(path, state{})
	if !reflect.DeepEqual(loaded, original) {
		t.Fatalf("LoadState: got %+v want %+v", loaded, original)
	}
}

func TestSaveStateLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.gob")

	if err := SaveState(path, struct{ Value string }{Value: "saved"}); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".state.gob.tmp-") {
			t.Fatalf("temporary file left behind: %s", entry.Name())
		}
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("saved state permissions: got %04o want 0600", got)
	}
}

func TestSaveStateEncodeFailurePreservesPreviousFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.gob")
	previous := struct{ Value string }{Value: "previous"}
	if err := SaveState(path, previous); err != nil {
		t.Fatalf("initial SaveState: %v", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile before failed save: %v", err)
	}

	// Functions are not gob-serializable. Encoding may write a prefix before
	// discovering the unsupported field, which is exactly why SaveState must
	// never encode directly into the destination.
	bad := struct {
		Value string
		Bad   any
	}{Value: "replacement", Bad: func() {}}
	if err := SaveState(path, bad); err == nil {
		t.Fatal("SaveState with unsupported field succeeded")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile after failed save: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Fatal("failed save replaced the previous state file")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".state.gob.tmp-") {
			t.Fatalf("temporary file left behind after failed save: %s", entry.Name())
		}
	}
}

func TestLoadStateMissingFileReturnsDefault(t *testing.T) {
	defaultState := struct{ X int }{X: 42}
	loaded := LoadState("/nonexistent/path/state.gob", defaultState)
	if loaded.X != 42 {
		t.Fatalf("LoadState with missing file: got %v want default %v", loaded, defaultState)
	}
}

func TestLoadStateWithErrorPreservesMissingFileError(t *testing.T) {
	defaultState := struct{ X int }{X: 42}
	loaded, err := LoadStateWithError("/nonexistent/path/state.gob", defaultState)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("LoadStateWithError error = %v, want not-exist", err)
	}
	if loaded != defaultState {
		t.Fatalf("LoadStateWithError state = %v, want default %v", loaded, defaultState)
	}
}

func TestLoadStateCorruptFileReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.gob")
	if err := os.WriteFile(path, []byte("not gob data"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	defaultState := struct{ X int }{X: 99}
	loaded := LoadState(path, defaultState)
	if loaded.X != 99 {
		t.Fatalf("LoadState with corrupt file: got %v want default %v", loaded, defaultState)
	}
}

func TestLoadStateWithErrorPreservesDecodeError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corrupt.gob")
	if err := os.WriteFile(path, []byte("not gob data"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	defaultState := struct{ X int }{X: 99}
	loaded, err := LoadStateWithError(path, defaultState)
	if err == nil {
		t.Fatal("LoadStateWithError error = nil, want decode error")
	}
	if loaded != defaultState {
		t.Fatalf("LoadStateWithError state = %v, want default %v", loaded, defaultState)
	}
}

func TestLoadStateShapeChangeReturnsDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.gob")

	type oldShape struct {
		Name string
	}
	type newShape struct {
		Title string // renamed field
		Count int    // new field
	}

	old := oldShape{Name: "before"}
	if err := SaveState(path, old); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded := LoadState(path, newShape{Title: "default", Count: 0})
	if loaded.Title != "default" {
		t.Fatalf("shape change: got %+v want default", loaded)
	}
}
