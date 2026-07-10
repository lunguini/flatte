//go:build !tinygo

// This stub keeps cmd/snake-verify buildable by the standard Go toolchain
// (`go build ./...`, `go vet ./...`) even though the real verifier
// (verify.go) targets TinyGo's wasm-unknown and uses //go:wasmexport, which
// the standard toolchain does not emit for this target. The verifier is built
// only via the tinygo command; see verify.go for the build line.
package main

func main() {}
