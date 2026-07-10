# flat-landing web build

This page hosts the same Flatte app used by `go run ./flat-landing`, compiled to
WebAssembly. The Go app core is shared; only `main_js.go` wires browser events
and DOM rendering. `index.html` + `styles.css` are the static shell.

The build artifacts (`flat-landing.wasm.gz`, `wasm_exec.js`) are **gitignored** —
build them locally with the commands below before serving.

## Build

From `cmd/`, with the standard Go toolchain:

```bash
GOOS=js GOARCH=wasm go build -o flat-landing/web/flat-landing.wasm ./flat-landing
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" flat-landing/web/wasm_exec.js
gzip -9 -f flat-landing/web/flat-landing.wasm
```

This is the exact sequence the Pages workflow runs. The standard Go compiler is
required: its garbage collector is solid under the browser's asyncify scheduler,
whereas TinyGo's GC deadlocks the scheduler when a hosted app allocates during a
JS callback (which the App tab does on every frame). The `.wasm` is shipped
gzip-compressed (~1.5 MB) and inflated in the browser via `DecompressionStream`,
so the transfer stays small regardless of the host's own compression.

## Serve

```bash
cd flat-landing/web
python3 -m http.server 8080
```

Open `http://localhost:8080`.

## Leaderboard (optional)

The Game tab can post scores to a shared Snake leaderboard. It's **off by
default**: the page runs unchanged with no badge and no network. To enable it,
deploy the Cloudflare Worker in [`../../../leaderboard`](../../../leaderboard)
and set its origin in `index.html`:

```js
window.FLATTE_LEADERBOARD_URL = "https://flatte-leaderboard.<subdomain>.workers.dev";
```

The browser submits only the game **seed** and its log of accepted turns; the
Worker replays that log through the same simulation the game runs and takes the
replayed score as authoritative, so scores can't be faked. The UI is a
browser-only DOM overlay (`leaderboard_js.go`) — the shared Flatte core never
touches the network.

## Notes

- **Loading / fallback:** the frame shows a boot line until the module runs. If
  the browser lacks WebAssembly or `DecompressionStream`, the frame shows a
  `go run` fallback instead of hanging.
- **Deep links:** `#game`, `#app`, `#what`, `#layout`, and `#ui` select the
  matching tab on load. The root page stays at `/` — the fragment is only written
  once the visitor actually switches tabs.
- **Cache busting:** `index.html` fetches the `.wasm.gz` and `wasm_exec.js` with a
  `?v=` query string; bump it when you rebuild so browsers don't serve a stale
  module.
