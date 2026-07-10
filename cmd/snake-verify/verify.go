//go:build tinygo

// Command snake-verify is the headless leaderboard verifier, compiled to a bare
// WebAssembly module for TinyGo's wasm-unknown target (no WASI, no scheduler).
// It shares the exact simulation the game plays (cmd/internal/snakesim), so a
// submitted (seed, input-log) re-derives its score with zero drift risk — the
// whole reason the sim was extracted into its own package.
//
// Build:
//
//	tinygo build -target=wasm-unknown -scheduler=none -opt=z \
//	  -o snake-verify.wasm ./snake-verify
//
// ABI (all synchronous, run-to-completion — no callbacks, so the TinyGo
// GC/asyncify deadlock cannot occur):
//
//	memory        exported linear memory
//	input_ptr() -> i32   address of the shared input buffer; the host writes
//	                     the payload there before calling verify
//	input_cap() -> i32   capacity of that buffer, so the host can reject
//	                     oversized payloads before writing
//	verify(len i32) -> i64
//	                     decodes input[:len] and returns the replayed score, or
//	                     -1 if the payload is malformed
//
// Payload layout (little-endian, produced by the Worker from the JSON body):
//
//	byte 0..7    uint64 seed
//	then repeated, one per accepted turn:
//	  4 bytes    int32  move index
//	  1 byte     uint8  direction (0=Up 1=Down 2=Left 3=Right)
package main

import (
	"encoding/binary"
	"unsafe"

	"github.com/lunguini/flatte/cmd/internal/snakesim"
)

// main is required for a wasm-unknown module but never runs meaningful work; the
// host drives the exported functions directly.
func main() {}

// stepCap bounds a replay so a malformed or adversarial log cannot spin. The
// board is 26x22, so even a perfect game is far shorter than this.
const stepCap = 1 << 20

// eventSize is the on-wire size of one accepted turn: int32 move + uint8 dir.
const eventSize = 5

// input is the shared buffer the host writes the payload into. It is a package
// global so its address is stable for the whole module lifetime; 256 KiB holds
// ~52k turns, far more than any real game.
var input [1 << 18]byte

//go:wasmexport input_ptr
func inputPtr() int32 { return int32(uintptr(unsafe.Pointer(&input[0]))) }

//go:wasmexport input_cap
func inputCap() int32 { return int32(len(input)) }

//go:wasmexport verify
func verify(length int32) int64 {
	n := int(length)
	if n < 8 || n > len(input) || (n-8)%eventSize != 0 {
		return -1 // too short for a seed, oversized, or ragged event list
	}
	buf := input[:n]

	seed := binary.LittleEndian.Uint64(buf[:8])
	rest := buf[8:]
	events := make([]snakesim.Input, len(rest)/eventSize)
	for i := range events {
		off := i * eventSize
		move := int32(binary.LittleEndian.Uint32(rest[off : off+4]))
		dir := rest[off+4]
		if dir > 3 {
			return -1 // not a valid direction
		}
		events[i] = snakesim.Input{Move: int(move), Dir: snakesim.Direction(dir)}
	}

	return int64(snakesim.Replay(seed, events, stepCap))
}
