package window

import (
	"runtime"
	"testing"

	"github.com/lunguini/flatte"
)

var (
	_ flatte.Option = Options{}
	_ flatte.Option = MediaControls{}
)

func TestSupportedMatchesWindowBuild(t *testing.T) {
	if got, want := Supported(), runtime.GOOS == "darwin"; got != want {
		t.Fatalf("Supported() = %t on %s, want %t", got, runtime.GOOS, want)
	}
}
