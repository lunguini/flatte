# flat-landing web build

This page hosts the same Flatte app used by `go run ./flat-landing`, compiled to
WebAssembly with either the standard Go toolchain or TinyGo 0.41. Both builds
call `flatte.Run`; by default its js/wasm backend mounts the `data-flatte`
element and renders the cell grid with text nodes. Apps that pass
`window.Options` select the GUI canvas host instead. `main_js.go` contains only
this page's deep-link and leaderboard integration. `index.html` + `styles.css`
are the static shell.

The build artifacts (`flat-landing.wasm.gz`, `wasm_exec.js`) are **gitignored** —
build them locally with the commands below before serving.

## Build

From `cmd/`, build with standard Go:

```bash
GOOS=js GOARCH=wasm go build -o flat-landing/web/flat-landing.wasm ./flat-landing
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" flat-landing/web/wasm_exec.js
gzip -9 -f flat-landing/web/flat-landing.wasm
```

Or build with TinyGo 0.41:

```bash
tinygo build -target wasm -o flat-landing/web/flat-landing.wasm ./flat-landing
cp "$(tinygo env TINYGOROOT)/targets/wasm_exec.js" flat-landing/web/wasm_exec.js
gzip -9 -f flat-landing/web/flat-landing.wasm
```

Use the `wasm_exec.js` supplied by the same toolchain that built the `.wasm`.
The Pages workflow continues to publish the standard Go build. From the repo
root, `scripts/wasm-check.sh` builds both toolchains, prints raw, gzip, and
Brotli sizes for the cell and canvas variants of the same smoke app, runs Go
browser tests under Node, and runs TinyGo cell and canvas smokes through the
DOM shim.

This landing page stays in cell-grid mode, so `RenderContext.Graphical` is
false. The canvas host is available to browser apps through `window.Options{}`
and reports `Graphical: true`; it measures with Canvas fonts, displays GUI
scene images, and uses a high-DPI backing store. Clipboard access depends on
browser support and permission. Notifications are sent only when the Web
Notifications API is already permitted; otherwise they are ignored.

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
