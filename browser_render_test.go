package flatte

import (
	"strings"
	"testing"
)

func TestBrowserSpansDecodeANSIWithoutTreatingTextAsMarkup(t *testing.T) {
	lines := browserSpans("\x1b[1;3;4;38;2;1;2;3m<tag>\x1b[0m\n界x")
	if len(lines) != 2 || len(lines[0]) != 1 || len(lines[1]) != 2 {
		t.Fatalf("browserSpans shape = %#v", lines)
	}
	styled := lines[0][0]
	if styled.Text != "<tag>" {
		t.Fatalf("styled text = %q, want literal markup", styled.Text)
	}
	if styled.Foreground != "#010203" || !styled.Bold || !styled.Italic || styled.Underline == "" {
		t.Fatalf("styled span = %#v", styled)
	}
	if got := lines[1][0]; got.Text != "界" || got.Cells != 2 {
		t.Fatalf("wide span = %#v, want two-cell grapheme", got)
	}
}

func TestBrowserSpansPreserveBackgroundAttributesAndSafeLinks(t *testing.T) {
	content := "\x1b[48;2;4;5;6;9mstrike\x1b[0m " +
		"\x1b]8;;https://example.com/a?q=1\x1b\\safe\x1b]8;;\x1b\\ " +
		"\x1b]8;;javascript:alert(1)\x1b\\unsafe\x1b]8;;\x1b\\"
	lines := browserSpans(content)
	if len(lines) != 1 {
		t.Fatalf("lines = %d, want 1", len(lines))
	}
	var strike, safe, unsafe *browserSpan
	for i := range lines[0] {
		span := &lines[0][i]
		switch {
		case strings.Contains(span.Text, "strike"):
			strike = span
		case strings.Contains(span.Text, "safe") && !strings.Contains(span.Text, "unsafe"):
			safe = span
		case strings.Contains(span.Text, "unsafe"):
			unsafe = span
		}
	}
	if strike == nil || strike.Background != "#040506" || !strike.Strikethrough {
		t.Fatalf("strike span = %#v", strike)
	}
	if safe == nil || safe.Link != "https://example.com/a?q=1" {
		t.Fatalf("safe link span = %#v", safe)
	}
	if unsafe == nil || unsafe.Link != "" {
		t.Fatalf("unsafe link span = %#v", unsafe)
	}
}
