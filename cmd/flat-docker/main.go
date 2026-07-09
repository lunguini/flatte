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

	err := flatte.Run(ctx, flatte.App[dockerapp.State]{
		State:  state,
		Init:   dockerapp.Init,
		Handle: dockerapp.Handle,
		View:   dockerapp.View,
		OnExit: func(s *dockerapp.State) {
			_ = flatte.SaveState(dockerapp.SessionFile, s.Session())
		},
	}, flatte.WithMouse(flatte.MouseModeCellMotion))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
