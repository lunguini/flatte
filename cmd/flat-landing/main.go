//go:build !js || !wasm

package main

import (
	"flag"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/window"
)

func main() {
	gui := false
	if window.Supported() {
		flag.BoolVar(&gui, "gui", false, "run in a native window instead of the terminal")
	}
	flag.Parse()
	runOptions := []flatte.Option{}
	if gui {
		runOptions = append(runOptions, window.Options{Title: "Flatte", Width: 100, Height: 32})
	}
	exitOnError(runApp(landingApp(), runOptions...))
}
