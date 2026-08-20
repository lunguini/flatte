package persist

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type testState struct{ Value string }

func TestSaveRefusesToOverwriteCorruptState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.gob")
	wantBytes := []byte("corrupt evidence")
	if err := os.WriteFile(path, wantBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	state, loadErr := Load(path, testState{Value: "default"})
	if loadErr == nil {
		t.Fatal("Load error = nil, want corrupt-state error")
	}
	state.Value = "changed"
	if err := Save(path, state, loadErr); err == nil {
		t.Fatal("Save succeeded over corrupt state")
	}
	gotBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBytes) != string(wantBytes) {
		t.Fatalf("corrupt evidence changed: got %q want %q", gotBytes, wantBytes)
	}
}

func TestSaveAllowsFirstRunAndValidReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.gob")
	state, loadErr := Load(path, testState{Value: "default"})
	if !errors.Is(loadErr, os.ErrNotExist) {
		t.Fatalf("missing Load error = %v, want os.ErrNotExist", loadErr)
	}
	state.Value = "first"
	if err := Save(path, state, loadErr); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	loaded, loadErr := Load(path, testState{})
	if loadErr != nil || loaded.Value != "first" {
		t.Fatalf("valid Load = %+v, %v", loaded, loadErr)
	}
	loaded.Value = "replacement"
	if err := Save(path, loaded, loadErr); err != nil {
		t.Fatalf("replacement Save: %v", err)
	}
}
