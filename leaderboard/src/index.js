// Cloudflare Worker: the shared Snake leaderboard for the Flatte landing demo.
//
// Trust model: the client never sends a score. It sends the seed and the log of
// accepted turns; the Worker replays that log through the SAME simulation the
// game runs (compiled to wasm from cmd/snake-verify, which imports the shared
// cmd/internal/snakesim) and takes the replayed score as authoritative. A faked
// high score would have to be a genuinely reachable game, which is the point.
//
// Free-tier discipline (Workers Free: 100k req/day, 10 ms CPU; KV Free: 100k
// reads, 1k writes/day, 1 write/s per key):
//   - Verification is a synchronous wasm replay of a ≤~1 KB log — well under
//     10 ms, and the module is instantiated once per isolate.
//   - The whole board is one KV value; GET is edge-cached, and a submission
//     only WRITES when the score actually qualifies for the top N, so ordinary
//     play doesn't burn the daily write budget.
//   - A best-effort per-isolate IP throttle blunts rapid-fire submits. It is
//     not shared across isolates (that would need Durable Objects); verification
//     is the real abuse guard, this is only a courtesy limit.

import wasmModule from "./snake-verify.wasm";
import { makeVerifier } from "./verifier.js";
import {
  cleanName,
  normalizeInputs,
  insertScore,
  qualifies,
  rankOf,
  MAX_SCORE,
} from "./board.js";

const BOARD_KEY = "board";
const RATE_WINDOW_MS = 3000; // min gap between accepted submits from one IP
const GET_CACHE_S = 10; // edge + browser cache seconds for the board

// Per-isolate throttle. Reset on cold start; bounded so it can't grow forever.
const lastSubmit = new Map();

// Lazily instantiated verifier, cached for the isolate's lifetime.
let verify = null;
function verifier() {
  return (verify ??= makeVerifier(wasmModule));
}

function throttled(ip) {
  const now = Date.now();
  const prev = lastSubmit.get(ip) || 0;
  if (now - prev < RATE_WINDOW_MS) return true;
  lastSubmit.set(ip, now);
  if (lastSubmit.size > 10000) lastSubmit.clear(); // crude cap; correctness-neutral
  return false;
}

const CORS = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Methods": "GET, POST, OPTIONS",
  "Access-Control-Allow-Headers": "Content-Type",
};

function json(body, status = 200, extra = {}) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json", ...CORS, ...extra },
  });
}

async function readBoard(env) {
  const raw = await env.LEADERBOARD_KV.get(BOARD_KEY);
  if (!raw) return [];
  try {
    const parsed = JSON.parse(raw);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

async function handleGet(env) {
  const board = await readBoard(env);
  return json(
    { board },
    200,
    { "Cache-Control": `public, max-age=${GET_CACHE_S}` },
  );
}

async function handlePost(request, env) {
  const ip = request.headers.get("CF-Connecting-IP") || "unknown";
  if (throttled(ip)) return json({ error: "slow down" }, 429);

  let body;
  try {
    body = await request.json();
  } catch {
    return json({ error: "invalid JSON" }, 400);
  }

  const inputs = normalizeInputs(body?.inputs);
  if (inputs === null) return json({ error: "invalid inputs" }, 400);
  if (body?.seed === undefined || body?.seed === null) {
    return json({ error: "missing seed" }, 400);
  }

  const score = verifier()(body.seed, inputs);
  if (score < 0 || score > MAX_SCORE) {
    return json({ error: "unverifiable score" }, 400);
  }

  const name = cleanName(body?.name);

  // Read-modify-write on the single board key. KV is eventually consistent, so
  // two writes racing within a second can drop one entry — acceptable for a
  // demo board, and the 1-write/s guidance is respected because we only write
  // qualifying scores.
  const board = await readBoard(env);
  const rank = rankOf(board, score);
  if (qualifies(board, score)) {
    const next = insertScore(board, { name, score, at: Date.now() });
    await env.LEADERBOARD_KV.put(BOARD_KEY, JSON.stringify(next));
    return json({ ok: true, score, rank: rankOf(next, score), board: next });
  }

  // Not a top-N score: report it without a write.
  return json({ ok: true, score, rank, board });
}

export default {
  async fetch(request, env) {
    const { pathname } = new URL(request.url);

    if (request.method === "OPTIONS") {
      return new Response(null, { status: 204, headers: CORS });
    }
    if (pathname === "/api/leaderboard" && request.method === "GET") {
      return handleGet(env);
    }
    if (pathname === "/api/score" && request.method === "POST") {
      return handlePost(request, env);
    }
    if (pathname === "/" || pathname === "/api" || pathname === "/api/") {
      return json({ ok: true, service: "flatte-leaderboard" });
    }
    return json({ error: "not found" }, 404);
  },
};
