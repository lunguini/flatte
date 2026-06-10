package flatuitest

import (
	"os"
	"regexp"
	"strings"
	"testing"
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

func CleanFrame(frame string) string {
	frame = ansiPattern.ReplaceAllString(frame, "")
	return normalizeLineEndings(frame)
}

func normalizeLineEndings(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	return strings.TrimRight(value, "\n")
}
