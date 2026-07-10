// Verifier glue for the TinyGo wasm-unknown module (cmd/snake-verify). The
// module has zero imports and is a run-to-completion reactor, so this stays
// tiny: instantiate once, call _initialize once, then for each submission write
// the payload into the shared buffer and call verify(len).
//
// makeVerifier takes a compiled WebAssembly.Module so it works both in the
// Worker (where `import mod from "./snake-verify.wasm"` yields a Module) and in
// a Node test (where WebAssembly.compile(bytes) yields one). This is the seam
// that lets the real wasm be tested off-Cloudflare.

// Wire layout, matching cmd/snake-verify/verify.go:
//   bytes 0..7  uint64 seed (little-endian)
//   then per accepted turn: int32 move (LE) + uint8 dir
const HEADER = 8;
const EVENT = 5;

export function makeVerifier(wasmModule) {
  const instance = new WebAssembly.Instance(wasmModule, {});
  const ex = instance.exports;
  if (typeof ex._initialize === "function") ex._initialize();

  const ptr = ex.input_ptr();
  const cap = ex.input_cap();

  // verifyScore returns the authoritative replayed score, or -1 if the payload
  // is malformed or too large for the module buffer. seed may be a number,
  // bigint, or decimal string (the browser sends a string because a uint64 seed
  // can exceed Number.MAX_SAFE_INTEGER).
  return function verifyScore(seed, inputs) {
    const len = HEADER + EVENT * inputs.length;
    if (len > cap) return -1;

    const view = new DataView(ex.memory.buffer);
    const bytes = new Uint8Array(ex.memory.buffer);

    let s;
    try {
      s = BigInt.asUintN(64, BigInt(seed));
    } catch {
      return -1;
    }
    view.setBigUint64(ptr, s, true);

    let off = ptr + HEADER;
    for (let i = 0; i < inputs.length; i++) {
      view.setInt32(off, inputs[i].move | 0, true);
      bytes[off + 4] = inputs[i].dir & 0xff;
      off += EVENT;
    }

    return Number(ex.verify(len));
  };
}
