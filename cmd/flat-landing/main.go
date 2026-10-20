//go:build !js || !wasm

package main

import (
	"context"
	"flag"

	"github.com/lunguini/flatte/window"
)

func main() {
	gui := flag.Bool("gui", false, "run in a native window instead of the terminal")
	flag.Parse()
	if *gui {
		exitOnError(window.Run(context.Background(), landingApp(), window.Options{Title: "Flatte", Width: 100, Height: 32}))
		return
	}
	exitOnError(runTerminal())
}
