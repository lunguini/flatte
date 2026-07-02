package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lunguini/flatte"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	defer signal.Stop(sigCh)

	session := flatte.LoadState(sessionFile, SessionState{})
	state := newStateFromSession(session)

	err := flatte.Run(ctx, flatte.App[State]{
		State:  state,
		Init:   initAsync,
		Handle: Handle,
		View:   View,
		OnExit: func(s *State) {
			_ = flatte.SaveState(sessionFile, s.toSession())
		},
	}, flatte.WithMouse(flatte.MouseModeCellMotion))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func initAsync(s *State, fx flatte.Effects[State]) {
	flatte.Every(fx, "stats-poll", 1*time.Second, func(s *State, now time.Time) {
		s.containers.tickStats(now)
	})
	s.containers.startScopedLogs(s, fx)
	s.containers.pushActivity(tipForGlyphs(pickGlyphs()))
}

// tipForGlyphSet returns a one-time hint that explains the glyph choice
// and how to switch. Detection of "did the glyphs render" is not possible
// from inside a terminal — there is no feedback channel from the terminal
// back to the app about font coverage. The honest workaround is to inform
// the user once and let them pick via env var.
