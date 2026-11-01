package window

import (
	"os"
	"strings"
	"testing"

	"github.com/ebitengine/purego/objc"
)

func TestFileTypesAndTilde(t *testing.T) {
	if got := strings.Join(fileTypes([]string{".txt", "md"}), ","); got != "txt,md" {
		t.Errorf("fileTypes %q", got)
	}
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	home, _ := os.UserHomeDir()
	withPool(func() {
		if got := expandTilde("~/x"); got != home+"/x" {
			t.Errorf("expandTilde %q, want %q", got, home+"/x")
		}
		if got := expandTilde("/a/~b"); got != "/a/~b" {
			t.Errorf("expandTilde %q", got)
		}
	})
}

func TestURLPaths(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	withPool(func() {
		a := class("NSMutableArray").Send(sel("array"))
		for _, p := range []string{"/tmp/one.txt", "/tmp/two words.md"} {
			a.Send(sel("addObject:"), class("NSURL").Send(sel("fileURLWithPath:"), nsString(p)))
		}
		if got := strings.Join(urlPaths(a), "|"); got != "/tmp/one.txt|/tmp/two words.md" {
			t.Errorf("urlPaths %q", got)
		}
		if urlPaths(objc.ID(0)) != nil {
			t.Error("nil array")
		}
	})
}

func TestDroppedPasteboard(t *testing.T) {
	if err := loadFrameworks(); err != nil {
		t.Fatal(err)
	}
	pb := pasteboard{name: "dev.flatte.window.droptest"}
	defer withPool(func() { pb.board().Send(sel("releaseGlobally")) })
	var paths []string
	var text string
	withPool(func() {
		board := pb.board()
		board.Send(sel("clearContents"))
		urls := class("NSArray").Send(sel("arrayWithObject:"), class("NSURL").Send(sel("fileURLWithPath:"), nsString("/tmp/dropped.txt")))
		board.Send(sel("writeObjects:"), urls)
		paths, text = dropped(board)
	})
	if len(paths) == 0 && text == "" {
		t.Skip("pasteboard service unavailable (sandboxed?)")
	}
	if len(paths) != 1 || paths[0] != "/tmp/dropped.txt" || text != "" {
		t.Fatalf("files: %q %q", paths, text)
	}
	pb.set("just text")
	withPool(func() { paths, text = dropped(pb.board()) })
	if len(paths) != 0 || text != "just text" {
		t.Fatalf("text: %q %q", paths, text)
	}
}
