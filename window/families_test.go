package window

import (
	"runtime"
	"slices"
	"testing"
)

func TestSortFamiliesIgnoresCase(t *testing.T) {
	fams := []FontFamily{{Name: "menlo"}, {Name: "Arial"}, {Name: "Menlo", Monospace: true}, {Name: "avenir"}}
	sortFamilies(fams)
	want := []FontFamily{{Name: "Arial"}, {Name: "avenir"}, {Name: "Menlo", Monospace: true}, {Name: "menlo"}}
	if !slices.Equal(fams, want) {
		t.Fatalf("got %v, want %v", fams, want)
	}
}

func TestFontFamiliesEmptyOffMacOS(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("macOS lists its fonts")
	}
	if fams := FontFamilies(); len(fams) != 0 {
		t.Fatalf("got %d families, want none", len(fams))
	}
}
