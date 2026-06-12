package flatuitest

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/lunguini/flat/internal/flatcore"
)

var ansiPattern = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func AssertGolden(t *testing.T, path string, frame string) {
	t.Helper()

	got := CleanFrame(frame)
	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v\nactual:\n%s", path, err, got)
	}

	want := normalizeLineEndings(string(wantBytes))
	if got != want {
		t.Fatalf("snapshot mismatch for %s\nwant:\n%s\n\ngot:\n%s", path, want, got)
	}
}

// AssertGoldenFrame compares a Frame against a golden: cleaned content,
// then metadata footer lines — only when metadata is set, so frames
// without cursor or title keep their existing goldens byte-identical.
func AssertGoldenFrame(t *testing.T, path string, frame flatcore.Frame) {
	t.Helper()
	AssertGolden(t, path, RenderFrame(frame))
}

// RenderFrame serializes a frame for golden comparison.
func RenderFrame(frame flatcore.Frame) string {
	out := CleanFrame(frame.Content)
	if frame.Cursor != nil {
		out += fmt.Sprintf("\n[cursor %d,%d]", frame.Cursor.X, frame.Cursor.Y)
	}
	if frame.Title != "" {
		out += "\n[title " + frame.Title + "]"
	}
	return out
}

func CleanFrame(frame string) string {
	frame = ansiPattern.ReplaceAllString(frame, "")
	return normalizeLineEndings(frame)
}

func normalizeLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimRight(value, "\n")
}
