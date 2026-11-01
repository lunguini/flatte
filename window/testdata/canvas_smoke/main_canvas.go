//go:build js && wasm && canvas

package main

import (
	"context"

	"github.com/lunguini/flatte"
	"github.com/lunguini/flatte/window"
)

func main() {
	theme := window.Dark()
	theme.Font, theme.FontSize = "system-ui", 13
	if err := flatte.Run(context.Background(), smokeApp(), window.Options{Theme: &theme}); err != nil {
		panic(err)
	}
}
