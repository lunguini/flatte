package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

const quietWindow = 80 * time.Millisecond

func main() {
	appModes := flag.Bool("app-modes", false, "enter alt-screen and bracketed-paste modes like flatte.Run")
	flag.Parse()
	if err := run(os.Stdin, os.Stdout, *appModes); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(in *os.File, out *os.File, appModes bool) error {
	oldState, err := term.MakeRaw(int(in.Fd()))
	if err != nil {
		return err
	}
	defer term.Restore(int(in.Fd()), oldState)
	if appModes {
		fmt.Fprint(out, enterAppModes())
		defer fmt.Fprint(out, exitAppModes())
	}

	fmt.Fprint(out, "flat-keys: press keys to inspect; ctrl-c/ctrl-d exits\r\n\r\n")
	bytes := readBytes(in)
	for {
		seq, ok := readSequence(bytes, quietWindow)
		if !ok || isExitSequence(seq) {
			fmt.Fprint(out, "\r\n")
			return nil
		}
		fmt.Fprintf(out, "%s\r\n", describeSequence(seq))
	}
}

func enterAppModes() string {
	return ansi.SetModeAltScreenSaveCursor + ansi.HideCursor + ansi.SetModeBracketedPaste
}

func exitAppModes() string {
	return ansi.ResetModeBracketedPaste + ansi.ShowCursor + ansi.ResetModeAltScreenSaveCursor
}

func readBytes(in *os.File) <-chan byte {
	out := make(chan byte)
	go func() {
		defer close(out)
		var buf [1]byte
		for {
			n, err := in.Read(buf[:])
			if err != nil {
				return
			}
			if n == 1 {
				out <- buf[0]
			}
		}
	}()
	return out
}

func readSequence(bytes <-chan byte, quiet time.Duration) ([]byte, bool) {
	first, ok := <-bytes
	if !ok {
		return nil, false
	}
	seq := []byte{first}
	timer := time.NewTimer(quiet)
	defer timer.Stop()
	for {
		select {
		case b, ok := <-bytes:
			if !ok {
				return seq, true
			}
			seq = append(seq, b)
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			timer.Reset(quiet)
		case <-timer.C:
			return seq, true
		}
	}
}

func isExitSequence(seq []byte) bool {
	return len(seq) == 1 && (seq[0] == 0x03 || seq[0] == 0x04)
}

func describeSequence(seq []byte) string {
	return fmt.Sprintf("bytes=%s quoted=%s events=%s", formatHex(seq), strconv.QuoteToASCII(string(seq)), formatEvents(decodeEvents(seq)))
}

func formatHex(seq []byte) string {
	parts := make([]string, len(seq))
	for i, b := range seq {
		parts[i] = fmt.Sprintf("%02x", b)
	}
	return strings.Join(parts, " ")
}

func decodeEvents(seq []byte) []uv.Event {
	var decoder uv.EventDecoder
	var events []uv.Event
	for len(seq) > 0 {
		n, event := decoder.Decode(seq)
		if n <= 0 {
			events = append(events, uv.UnknownEvent(seq[:1]))
			seq = seq[1:]
			continue
		}
		if event != nil {
			events = append(events, event)
		}
		seq = seq[n:]
	}
	return events
}

func formatEvents(events []uv.Event) string {
	if len(events) == 0 {
		return "[]"
	}
	parts := make([]string, len(events))
	for i, event := range events {
		parts[i] = fmt.Sprintf("%#v", event)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
