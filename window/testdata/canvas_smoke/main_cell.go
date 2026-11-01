//go:build js && wasm && !canvas

package main

import (
	"context"

	"github.com/lunguini/flatte"
)

func main() {
	if err := flatte.Run(context.Background(), smokeApp()); err != nil {
		panic(err)
	}
}
