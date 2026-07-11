# Flatte Snake leaderboard (Cloudflare Worker)

A shared high-score board for the Snake game in the Flatte landing demo — with
no game server. The browser sends the game **seed** and its log of **accepted
turns**; the Worker replays that log through the *same* simulation the game runs
(compiled to WebAssembly from `../cmd/snake-verify`, which imports the shared
`../cmd/internal/snakesim`) and takes the replayed score as authoritative. A
faked high score would have to be a genuinely reachable game.

## How it stays inside the free tier

Workers Free gives 100k requests/day and 10 ms CPU per request; KV Free gives
100k reads, **1,000 writes/day**, and 1 write/s per key. So:

- **Verification is a synchronous wasm replay** of a ~1 KB log — well under
  10 ms, and the module is instantiated once per isolate (zero imports).
- **The whole board is one KV value** (`board`). `GET` is edge-cached, and a
  submission only **writes** when the score actually makes the top `N`
  (`BOARD_SIZE`, default 50), so ordinary play never touches the write budget.
- **A per-isolate IP throttle** blunts rapid-fire submits. It isn't shared
  across isolates (that would need Durable Objects) — verification is the real
  abuse guard; this is a courtesy limit.

## API

| Method | Path | Body | Returns |
|---|---|---|---|
| `GET` | `/api/leaderboard` | — | `{ board: [{name, score, at}] }` (cached ~10 s) |
| `POST` | `/api/score` | `{ name, seed, inputs }` | `{ ok, score, rank, board }` |

`seed` is a **decimal string** (a uint64 seed can exceed
`Number.MAX_SAFE_INTEGER`). `inputs` is the accepted-turn log, either
`[[move, dir], …]` (compact) or `[{move, dir}, …]`; `dir` is `0=Up 1=Down
2=Left 3=Right`. The server ignores any client-sent score. Names are
NFC-normalized, stripped of control/zero-width characters, whitespace-collapsed,
and capped at 24 code points; empty becomes `anonymous`.

## Deploy (GitHub Actions — no local tooling)

Deployment runs in CI (`.github/workflows/leaderboard.yml`): the runner builds
the verifier wasm from the Go source, runs the tests, and deploys with
Cloudflare's official action. You never install npm, Node, or Wrangler locally.

One-time setup:

1. Create a KV namespace (dashboard → Workers & Pages → KV, or `wrangler kv
   namespace create LEADERBOARD_KV` from anywhere with Wrangler) and put its id
   (and a `preview_id`) into `wrangler.toml`.
2. Add repo secrets `CLOUDFLARE_API_TOKEN` (permissions: *Workers Scripts:Edit*
   and *Workers KV Storage:Edit*) and `CLOUDFLARE_ACCOUNT_ID`.

Then every push to `main` under `leaderboard/**`, `cmd/snake-verify/**`, or
`cmd/internal/snakesim/**` deploys — or trigger it manually from the Actions
tab. After the first deploy, set `window.FLATTE_LEADERBOARD_URL` in
`../cmd/flat-landing/web/index.html` to the Worker's origin.

## Local (optional)

npm is **only** a convenience for `wrangler dev` and is not required for build,
test, or deploy. Build and test need nothing but TinyGo and Node:

```bash
# build the verifier wasm (from the repo's cmd/ dir)
(cd ../cmd && ./snake-verify/build.sh "$PWD/../leaderboard/src/snake-verify.wasm")
# run the tests — Node ships `node --test`; the tests have zero dependencies
node --test
```

For a local dev server, `npm install && npm run dev` uses Wrangler's miniflare.

`test/logic.test.mjs` checks the board helpers and drives the **real** verifier
wasm through the same seam the Worker uses, asserting the replayed scores match
fixtures generated from the Go simulation (`test/vectors.json`). Regenerate the
fixtures whenever the simulation changes.

## Layout

- `src/index.js` — HTTP + KV + rate-limit shell.
- `src/board.js` — pure board logic (sanitize, validate, insert, rank).
- `src/verifier.js` — wasm glue (`makeVerifier`), dependency-injected so it runs
  under both Wrangler and Node.
- `src/snake-verify.wasm` — built artifact (gitignored); see `build:wasm`.
