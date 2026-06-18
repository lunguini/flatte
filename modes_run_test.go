package flat

import (
	"strings"
	"testing"
)

func plainView(s *testState, ctx RenderContext) Frame {
	return Frame{Content: "x"}
}

// indexAfter returns the index of needle at or after from, or -1.
func indexAfter(s, needle string, from int) int {
	if from < 0 || from > len(s) {
		return -1
	}
	i := strings.Index(s[from:], needle)
	if i == -1 {
		return -1
	}
	return from + i
}

func TestRunEnablesBracketedPasteByDefault(t *testing.T) {
	output := runFrameApp(t, plainView, "q")

	altScreen := strings.Index(output, "\x1b[?1049h")
	set := indexAfter(output, "\x1b[?2004h", altScreen)
	if altScreen == -1 || set == -1 {
		t.Fatalf("bracketed paste not enabled after alt screen:\n%q", output)
	}
	reset := strings.Index(output, "\x1b[?2004l")
	exitAlt := strings.Index(output, "\x1b[?1049l")
	if reset == -1 || exitAlt < reset {
		t.Fatalf("bracketed paste not reset before alt-screen exit (reset=%d exit=%d):\n%q", reset, exitAlt, output)
	}
}

func TestRunWithoutBracketedPaste(t *testing.T) {
	output := runFrameApp(t, plainView, "q", WithoutBracketedPaste())

	if strings.Contains(output, "2004") {
		t.Fatalf("bracketed paste sequences present despite opt-out:\n%q", output)
	}
}

func TestRunMouseModes(t *testing.T) {
	cases := []struct {
		name    string
		mode    MouseMode
		wantSet []string
	}{
		{"cell motion", MouseModeCellMotion, []string{"\x1b[?1002h", "\x1b[?1006h"}},
		{"all motion", MouseModeAllMotion, []string{"\x1b[?1003h", "\x1b[?1006h"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			output := runFrameApp(t, plainView, "q", WithMouse(tc.mode))
			for _, want := range tc.wantSet {
				if !strings.Contains(output, want) {
					t.Fatalf("missing %q:\n%q", want, output)
				}
			}
			// All mouse modes reset on exit regardless of which was set.
			for _, want := range []string{"\x1b[?1002l", "\x1b[?1003l", "\x1b[?1006l"} {
				if !strings.Contains(output, want) {
					t.Fatalf("missing reset %q:\n%q", want, output)
				}
			}
		})
	}
}

func TestRunWithoutMouseEmitsNoMouseSequences(t *testing.T) {
	output := runFrameApp(t, plainView, "q")

	for _, seq := range []string{"1002", "1003", "1006"} {
		if strings.Contains(output, seq) {
			t.Fatalf("unexpected mouse sequence %q:\n%q", seq, output)
		}
	}
}

func TestRunReportFocus(t *testing.T) {
	output := runFrameApp(t, plainView, "q", WithReportFocus())

	if !strings.Contains(output, "\x1b[?1004h") || !strings.Contains(output, "\x1b[?1004l") {
		t.Fatalf("focus reporting not set+reset:\n%q", output)
	}
	if plain := runFrameApp(t, plainView, "q"); strings.Contains(plain, "1004") {
		t.Fatalf("focus sequences present without the option:\n%q", plain)
	}
}
