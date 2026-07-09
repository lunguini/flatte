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

	err := flatte.Run(context.Background(), flatte.App[snakeapp.State]{
		State:  s,
		Init:   snakeapp.Init,
		Handle: snakeapp.Handle,
		View:   snakeapp.View,
		OnExit: func(s *snakeapp.State) {
			_ = flatte.SaveState(stateFile, *s)
		},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
