package flatte

import (
	"context"
	"strings"
	"testing"
)

func TestNotificationSequence(t *testing.T) {
	env := func(kv ...string) func(string) string {
		return func(k string) string {
			for i := 0; i+1 < len(kv); i += 2 {
				if kv[i] == k {
					return kv[i+1]
				}
			}
			return ""
		}
	}
	n := Notification{Title: "Now playing", Body: "Glass — Mara"}
	for _, c := range []struct {
		name string
		n    Notification
		env  func(string) string
		want string
	}{
		{"osc9", n, env("TERM", "xterm-ghostty"), "\x1b]9;Now playing: Glass — Mara\x07"},
		{"osc9 body only", Notification{Body: "Saved"}, env(), "\x1b]9;Saved\x07"},
		{"osc9 title only", Notification{Title: "Saved"}, env(), "\x1b]9;Saved\x07"},
		{"osc9 leading digit", Notification{Body: "4;1 left"}, env(), "\x1b]9; 4;1 left\x07"},
		{"kitty", n, env("TERM", "xterm-kitty"), "\x1b]99;i=flatte:d=0:p=title;Now playing\x1b\\\x1b]99;i=flatte:d=1:p=body;Glass — Mara\x1b\\"},
		{"kitty by window id", Notification{Body: "Saved"}, env("KITTY_WINDOW_ID", "1"), "\x1b]99;i=flatte:d=0:p=title;Saved\x1b\\"},
		{"foot", Notification{Title: "a;b", Body: "c;d"}, env("TERM", "foot"), "\x1b]777;notify;a,b;c;d\x1b\\"},
		{"tmux", Notification{Body: "x"}, env("TMUX", "/tmp/tmux,1,0"), "\x1bPtmux;\x1b\x1b]9;x\x07\x1b\\"},
		{"empty", Notification{Title: " \n", Body: "\x1b"}, env(), ""},
	} {
		if got := notificationSequence(c.n, c.env); got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
}

func TestNotificationTextIsOneSafeLine(t *testing.T) {
	if got := notificationText("  a\x1b]0;evil\x07\n\tb\u0085c  "); got != "a]0;evil b c" {
		t.Errorf("got %q", got)
	}
	long := notificationText(strings.Repeat("x", 1000))
	if r := []rune(long); len(r) != maxNotificationRunes+1 || r[len(r)-1] != '…' {
		t.Errorf("long text is %d runes", len(r))
	}
}

func TestWithHostNotificationsRoutesNotify(t *testing.T) {
	var got []Notification
	fx := WithHostNotifications(NewHarnessEffects[struct{}](context.Background(), nil, nil, nil, nil), func(n Notification) { got = append(got, n) })
	fx.Notify(Notification{Title: "t", Body: "b"})
	if len(got) != 1 || got[0] != (Notification{Title: "t", Body: "b"}) {
		t.Fatalf("got %v", got)
	}
	// Without a host, harness and zero effects stay no-ops.
	NewHarnessEffects[struct{}](context.Background(), nil, nil, nil, nil).Notify(Notification{Body: "x"})
	Effects[struct{}]{}.Notify(Notification{Body: "x"})
}

func TestRunNotifyEmitsTheSequence(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	t.Setenv("TMUX", "")
	t.Setenv("KITTY_WINDOW_ID", "")
	output := runClipboardApp(t, func(s *testState, fx Effects[testState]) {
		fx.Notify(Notification{Title: "Now playing", Body: "Glass"})
	}, "xq", nil)
	if got := strings.Count(output, "\x1b]9;Now playing: Glass\x07"); got != 1 {
		t.Fatalf("notification emitted %d time(s), want 1:\n%q", got, output)
	}
}
