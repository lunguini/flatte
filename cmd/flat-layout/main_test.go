package main

import (
	"path/filepath"
	"testing"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/flatui/layout"
)

func TestStateSurvivesSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-state.gob")

	original := State{Cursor: 42, ShowModal: true, Width: 100, Height: 30}
	if err := flatte.SaveState(path, original); err != nil {
		t.Fatalf("SaveState: %v", err)
	}

	loaded := flatte.LoadState(path, State{})
	if loaded.Cursor != 42 {
		t.Fatalf("Cursor: got %d want 42", loaded.Cursor)
	}
	if !loaded.ShowModal {
		t.Fatalf("ShowModal: got false want true")
	}
	if loaded.Width != 100 || loaded.Height != 30 {
		t.Fatalf("Width/Height: got %d/%d want 100/30", loaded.Width, loaded.Height)
	}
}

func TestStateMissingFileReturnsDefault(t *testing.T) {
	loaded := flatte.LoadState("/nonexistent/state.gob", State{})
	if loaded.Cursor != 0 || loaded.ShowModal {
		t.Fatalf("default state: got %+v want {Cursor:0 ShowModal:false}", loaded)
	}
}

func TestLayoutSolvesCorrectly(t *testing.T) {
	s := &State{Width: 80, Height: 24, ShowModal: true}
	tree := buildTree(s)
	rects := layout.Solve(tree, 80, 24)

	header := rects["header"]
	if header.W != 80 || header.H != 3 {
		t.Fatalf("header: %+v want 80x3", header)
	}

	footer := rects["footer"]
	if footer.W != 80 || footer.H != 1 {
		t.Fatalf("footer: %+v want 80x1", footer)
	}

	sidebar := rects["sidebar"]
	if sidebar.W != 20 || sidebar.H != 20 {
		t.Fatalf("sidebar: %+v want 20x20", sidebar)
	}

	content := rects["content"]
	if content.X != 20 || content.W != 60 || content.H != 20 {
		t.Fatalf("content: %+v want {20,_,60,20}", content)
	}

	modal := rects["modal"]
	if modal.W != 40 || modal.H != 10 {
		t.Fatalf("modal: %+v want 40x10", modal)
	}
	// Centered: x=(80-40)/2=20, y=(24-10)/2=7
	if modal.X != 20 || modal.Y != 7 {
		t.Fatalf("modal position: %+v want (20,7)", modal)
	}
}

func TestNoModalWhenHidden(t *testing.T) {
	s := &State{Width: 80, Height: 24, ShowModal: false}
	tree := buildTree(s)
	rects := layout.Solve(tree, 80, 24)

	if _, ok := rects["modal"]; ok {
		t.Fatalf("modal should not be in solved rects when hidden")
	}
}
