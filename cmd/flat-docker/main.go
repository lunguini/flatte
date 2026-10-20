package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/dockerapp"
	"github.com/lunguini/flatte/cmd/internal/persist"
	"github.com/lunguini/flatte/window"
)

func main() {
	gui := flag.Bool("gui", false, "run in a native window instead of the terminal")
	flag.Parse()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		cancel()
	}()
	defer signal.Stop(sigCh)

	session, loadErr := persist.Load(dockerapp.SessionFile, dockerapp.SessionState{})
	state := dockerapp.NewStateFromSession(session)

	var saveErr error
	app := flatte.App[dockerapp.State]{
		State:  state,
		Init:   dockerapp.Init,
		Handle: dockerapp.Handle,
		View:   dockerapp.View,
		OnExit: func(s *dockerapp.State) {
			saveErr = persist.Save(dockerapp.SessionFile, s.Session(), loadErr)
		},
	}
	var runErr error
	if *gui {
		runErr = window.Run(ctx, app, window.Options{Title: "flat-docker", Width: 100, Height: 30})
	} else {
		runErr = flatte.Run(ctx, app, flatte.WithMouse(flatte.MouseModeCellMotion))
	}
	// Run has restored the terminal before returning; report persistence
	// failures only after that cleanup and fail the process visibly.
	if runErr != nil {
		fmt.Fprintln(os.Stderr, runErr)
	}
	if saveErr != nil {
		fmt.Fprintf(os.Stderr, "save state: %v\n", saveErr)
	}
	if runErr != nil || saveErr != nil {
		os.Exit(1)
	}
}
