#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_root="$(mktemp -d "${TMPDIR:-/tmp}/flatte-wasm-check.XXXXXX")"
mkdir -p "$tmp_root/go" "$tmp_root/tinygo"
trap 'rm -rf "$tmp_root"' EXIT

command -v go >/dev/null
command -v tinygo >/dev/null
command -v node >/dev/null
command -v gzip >/dev/null
command -v brotli >/dev/null

build_root="$repo_root/cmd"
(
	cd "$build_root"
	GOOS=js GOARCH=wasm go build -o "$tmp_root/go/flat-landing.wasm" ./flat-landing
	tinygo build -target wasm -o "$tmp_root/tinygo/flat-landing.wasm" ./flat-landing
)

(
	cd "$repo_root/window"
	GOOS=js GOARCH=wasm go build -o "$tmp_root/go/cell.wasm" ./testdata/canvas_smoke
	GOOS=js GOARCH=wasm go build -tags canvas -o "$tmp_root/go/canvas.wasm" ./testdata/canvas_smoke
	tinygo build -target wasm -o "$tmp_root/tinygo/cell.wasm" ./testdata/canvas_smoke
	tinygo build -tags canvas -target wasm -o "$tmp_root/tinygo/canvas.wasm" ./testdata/canvas_smoke
)

print_sizes() {
	local tool="$1"
	local variant="$2"
	local wasm="$tmp_root/$tool/$variant.wasm"
	local gzip_file="$tmp_root/$tool/$variant.wasm.gz"
	local brotli_file="$tmp_root/$tool/$variant.wasm.br"
	gzip -n -9 -c "$wasm" > "$gzip_file"
	brotli -q 11 -c "$wasm" > "$brotli_file"
	local raw_size gzip_size brotli_size
	raw_size="$(wc -c < "$wasm" | tr -d ' ')"
	gzip_size="$(wc -c < "$gzip_file" | tr -d ' ')"
	brotli_size="$(wc -c < "$brotli_file" | tr -d ' ')"
	printf '%s/%s raw=%s gzip=%s brotli=%s bytes\n' "$tool" "$variant" "$raw_size" "$gzip_size" "$brotli_size"
}

print_sizes go flat-landing
print_sizes tinygo flat-landing
print_sizes go cell
print_sizes go canvas
print_sizes tinygo cell
print_sizes tinygo canvas

node "$repo_root/testdata/run_browser_test.cjs" "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$tmp_root/go/flat-landing.wasm" landing
node "$repo_root/testdata/run_browser_test.cjs" "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" "$tmp_root/tinygo/flat-landing.wasm" landing
node "$repo_root/testdata/run_browser_test.cjs" "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$tmp_root/go/canvas.wasm" canvas
node "$repo_root/testdata/run_browser_test.cjs" "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" "$tmp_root/tinygo/canvas.wasm" canvas

GOOS=js GOARCH=wasm go test -c -run '^(TestBrowser(Run|HostConformance)|TestRunSelectsHostAndOwnsSessionLifecycle)$' -o "$tmp_root/go/flatte.test.wasm" "$repo_root"
node "$repo_root/testdata/run_browser_test.cjs" "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$tmp_root/go/flatte.test.wasm"

(
	cd "$repo_root/window"
	GOOS=js GOARCH=wasm go test -c -run '^(TestSupportedMatchesWindowBuild|TestBrowserWindowSupported|TestCanvasHostConformance|TestCanvasFontsLoadBeforeCanvasStarts)$' -o "$tmp_root/go/window.test.wasm" .
)
node "$repo_root/testdata/run_browser_test.cjs" "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$tmp_root/go/window.test.wasm" window-test

(
	cd "$repo_root"
	tinygo build -target wasm -o "$tmp_root/tinygo/browser-smoke.wasm" ./testdata/browser_smoke
)
node "$repo_root/testdata/run_browser_test.cjs" "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" "$tmp_root/tinygo/browser-smoke.wasm" smoke

(
	cd "$repo_root"
	tinygo build -target wasm -o "$tmp_root/tinygo/host-conformance.wasm" ./testdata/host_conformance
)
node "$repo_root/testdata/run_browser_test.cjs" "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" "$tmp_root/tinygo/host-conformance.wasm" conformance
