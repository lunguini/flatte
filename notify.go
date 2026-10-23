package flatte

import (
	"os"
	"strings"
	"unicode"
)

// Notification is a message for the user outside the app's frame: a
// system notification in a window; in a terminal, the terminal's own
// notification where it shows them (iTerm2, Ghostty, WezTerm, kitty, foot,
// and through tmux with allow-passthrough on). Title is its heading and
// Body its text; either may be empty.
type Notification struct {
	Title, Body string
}

// WithHostNotifications returns fx with Notify routed to show: a host with
// system notifications of its own (a window) posts them. Like
// WithHostClipboard it is host plumbing, not application API.
func WithHostNotifications[S any](fx Effects[S], show func(Notification)) Effects[S] {
	fx.notify = show
	return fx
}

// Notify shows n outside the frame, and is fire and forget: there is no
// answer, and hosts and terminals without notifications ignore it. The
// framework never decides when to notify — an app that wants to only when
// it is not in front can track FocusEvent (WithReportFocus in a terminal;
// a window always reports it). Loop-goroutine-only, like Quit. Safe on a
// zero Effects value.
func (fx Effects[S]) Notify(n Notification) {
	if fx.notify != nil {
		fx.notify(n)
		return
	}
	if fx.enqueue != nil {
		if seq := notificationSequence(n, os.Getenv); seq != "" {
			fx.enqueue(action{write: seq})
		}
	}
}

// maxNotificationRunes bounds a notification's title and body each: they
// are a line or two on screen, and terminals cap escape sequences.
const maxNotificationRunes = 240

// notificationSequence is n as the terminal named by env understands it:
// OSC 99 for kitty, OSC 777 for foot and rxvt, OSC 9 for the rest (iTerm2,
// Ghostty, WezTerm, and others that ignore it when unsupported), wrapped
// for tmux when inside it. "" when n has no text.
func notificationSequence(n Notification, env func(string) string) string {
	title, body := notificationText(n.Title), notificationText(n.Body)
	if title == "" && body == "" {
		return ""
	}
	term := env("TERM")
	var seq string
	switch {
	case env("KITTY_WINDOW_ID") != "" || term == "xterm-kitty":
		if title == "" {
			title, body = body, ""
		}
		seq = "\x1b]99;i=flatte:d=0:p=title;" + title + "\x1b\\"
		if body != "" {
			seq += "\x1b]99;i=flatte:d=1:p=body;" + body + "\x1b\\"
		}
	case strings.HasPrefix(term, "foot") || strings.HasPrefix(term, "rxvt"):
		seq = "\x1b]777;notify;" + strings.ReplaceAll(title, ";", ",") + ";" + body + "\x1b\\"
	default:
		text := title
		switch {
		case title == "":
			text = body
		case body != "":
			text = title + ": " + body
		}
		if text[0] >= '0' && text[0] <= '9' {
			text = " " + text // "9;4;…" and the like are other OSC 9 commands
		}
		seq = "\x1b]9;" + text + "\x07"
	}
	if env("TMUX") != "" {
		seq = "\x1bPtmux;" + strings.ReplaceAll(seq, "\x1b", "\x1b\x1b") + "\x1b\\"
	}
	return seq
}

// notificationText is s on one line — runs of spaces and line breaks one
// space — with no control characters (which would end or corrupt the
// escape sequence), at most maxNotificationRunes runes.
func notificationText(s string) string {
	var b strings.Builder
	n := 0
	space := false
	for _, r := range s {
		if unicode.IsSpace(r) { // newlines and tabs too: one line
			space = b.Len() > 0
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		if n == maxNotificationRunes {
			b.WriteRune('…')
			break
		}
		if space {
			b.WriteByte(' ')
			n++
			space = false
		}
		b.WriteRune(r)
		n++
	}
	return b.String()
}
