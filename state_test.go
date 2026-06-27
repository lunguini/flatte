package flatte

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestLoadStateMissingFileReturnsDefault(t *testing.T) {
	defaultState := struct{ X int }{X: 42}
	loaded := LoadState("/nonexistent/path/state.gob", defaultState)
	if loaded.X != 42 {
		t.Fatalf("LoadState with missing file: got %v want default %v", loaded, defaultState)
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
