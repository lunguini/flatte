package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/cmd/internal/snakeapp"
)

const stateFile = ".flat-game-state.gob"

func main() {
	loaded := flatte.LoadState(stateFile, snakeapp.State{})
	s := snakeapp.NewGame(uint64(time.Now().UnixNano()))
	s.HighScore = loaded.HighScore

	var saveErr error
	runErr := flatte.Run(context.Background(), flatte.App[snakeapp.State]{
		State:  s,
		Init:   snakeapp.Init,
		Handle: snakeapp.Handle,
		View:   snakeapp.View,
		OnExit: func(s *snakeapp.State) {
			saveErr = flatte.SaveState(stateFile, *s)
		},
	})
	// Run has restored the terminal before returning, so persistence
	// diagnostics cannot be overwritten by alt-screen cleanup.
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
