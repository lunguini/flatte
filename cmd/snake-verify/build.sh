#!/usr/bin/env bash
# Build the headless leaderboard verifier to a bare WebAssembly module.
#
# Output: a zero-import wasm-unknown reactor module (~100 KB, ~32 KB gzip). The
# Cloudflare Worker instantiates it with an empty import object, calls
# _initialize() once, writes the payload into the buffer at input_ptr(), and
# calls verify(len). See verify.go for the ABI and payload layout.
#
# Requires TinyGo (tested with 0.41.1 / Go 1.26). Run from cmd/:
#   ./snake-verify/build.sh [output.wasm]
set -euo pipefail

out="${1:-snake-verify.wasm}"
tinygo build -target=wasm-unknown -scheduler=none -opt=z -o "$out" ./snake-verify
printf 'built %s (%s bytes, %s gzip)\n' "$out" \
  "$(wc -c <"$out" | tr -d ' ')" \
  "$(gzip -9 -c "$out" | wc -c | tr -d ' ')"
