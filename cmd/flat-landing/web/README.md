# flat-landing web build

This page hosts the same Flatte app used by `go run ./flat-landing`, compiled to
WebAssembly. The Go app core is shared; only `main_js.go` wires browser events
and DOM rendering. `index.html` + `styles.css` are the static shell.

The two build artifacts (`flat-landing.wasm`, `wasm_exec.js`) are **gitignored** —
build them locally with one of the commands below before serving.

## TinyGo first (smaller binary)

From `cmd/`:

```bash
tinygo build -target=wasm -o flat-landing/web/flat-landing.wasm ./flat-landing
cp "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" flat-landing/web/wasm_exec.js
```

TinyGo produces a noticeably smaller module (~3.2 MB vs ~5.3 MB for the standard
Go build in a recent local check) and faster first paint. `wasm_exec.js` is
compiler-specific — always copy the one that matches the compiler you built with.

## Standard Go fallback

From `cmd/`:

```bash
GOOS=js GOARCH=wasm go build -o flat-landing/web/flat-landing.wasm ./flat-landing
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" flat-landing/web/wasm_exec.js
```

## Serve

```bash
cd flat-landing/web
python3 -m http.server 8080
```

Open `http://localhost:8080`.

## Notes

- **Loading / fallback:** the frame shows a boot line until the module runs. If
  the browser lacks WebAssembly or the module fails to load, the frame shows a
  `go run` fallback instead of hanging.
- **Deep links:** `#overview`, `#components`, and `#architecture` select the
  matching tab on load and stay in sync with in-app navigation.
- **Cache busting:** `index.html` fetches the `.wasm` with a `?v=` query string;
  bump it when you rebuild so browsers don't serve a stale module.
