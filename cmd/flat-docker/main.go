package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/dockerapp"
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

	session := flatte.LoadState(dockerapp.SessionFile, dockerapp.SessionState{})
	state := dockerapp.NewStateFromSession(session)

	var saveErr error
	runErr := flatte.Run(ctx, flatte.App[dockerapp.State]{
		State:  state,
		Init:   dockerapp.Init,
		Handle: dockerapp.Handle,
		View:   dockerapp.View,
		OnExit: func(s *dockerapp.State) {
			saveErr = flatte.SaveState(dockerapp.SessionFile, s.Session())
		},
	}, flatte.WithMouse(flatte.MouseModeCellMotion))
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
