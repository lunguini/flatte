package main

import (
	"context"
	"syscall/js"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/internal/hostconformance"
)

func main() {
	dom := js.Global().Get("__flatteTest")
	dom.Set("hostExit", false)
	app, _, observer := hostconformance.New()
	observer.OnExit = func() { dom.Set("hostExit", true) }
	if err := flatte.Run(context.Background(), app, flatte.WithMouse(flatte.MouseModeCellMotion)); err != nil {
		panic(err)
	}
}
