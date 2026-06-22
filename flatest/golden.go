package flatest

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/lunguini/flatte"
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
func AssertGoldenFrame(t *testing.T, path string, frame flatte.Frame) {
	t.Helper()
	AssertGolden(t, path, RenderFrame(frame))
}

// frameSeparator delimits frames in a sequence golden.
const frameSeparator = "\n───\n"

// AssertFrames compares a frame sequence against a golden: each frame
// RenderFrame'd, joined by a separator line, so an interaction can be
// regression-tested as an ordered series of frames.
func AssertFrames(t *testing.T, path string, frames []flatte.Frame) {
	t.Helper()
	parts := make([]string, len(frames))
	for i, frame := range frames {
		parts[i] = RenderFrame(frame)
	}
	AssertGolden(t, path, strings.Join(parts, frameSeparator))
}

// RenderFrame serializes a frame for golden comparison.
func RenderFrame(frame flatte.Frame) string {
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
