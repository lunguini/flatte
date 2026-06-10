# Flatte Development Guide

Flatte (working name; package `flat`, directory rename pending) is a
full-frame functional TUI foundation for Go: single mutable state struct,
`View` as a pure function of state, direct mutation instead of TEA message
dispatch, async funneled through named `StateUpdate`s onto a single-writer
loop. It is the deliberate inverse of Bubble Tea while reusing Charm's MIT
substrate (lipgloss today; `x/input` and the cell-buffer renderer planned).

## Document map (read in this order for context)

| Document | Role |
|---|---|
| `.docs/design.md` | The design: thesis, principles, architecture, phased roadmap. Direction lives here. |
| `.docs/STATUS.md` | **Living implementation tracker** — what exists, known bugs, what's next per phase. Truth lives here. |
| `.docs/evaluation.md` | Evidence log from dogfooding — what extracted cleanly, what's worse or unproven. Append-only in spirit. |
| `.docs/plans/` | Implementation plans for individual pieces of work. |

## The `.docs/` folder (single home for project docs; sensitive, git-crypt)

**All project documents live in `.docs/`** — design, status tracker,
evaluation log, plans. There is no other docs location. Rules:

- **Rely on it.** Before designing, implementing, or claiming anything
  about project state, read the relevant `.docs` file first. `.docs/STATUS.md`
  is the authoritative answer to "what's implemented and what's left" —
  trust it over assumptions, and verify it against code when in doubt.
- **Update it.** Any change that affects what these documents say (new
  feature, fixed bug, design decision, new finding) updates the matching
  `.docs` file in the same commit. New plans go to `.docs/plans/`,
  dated like `2026-06-09-line-diff-renderer.md`.
- `.docs/**` and `.specs/**` are encrypted with **git-crypt** (see
  `.gitattributes`). Sensitive or internal material goes here, never in
  public files or code comments.
- Do not copy `.docs` content into public files, commit messages, or
  README-style docs.
- If `.docs/` files look like binary garbage, the repo is **locked** — stop
  and ask the user to run `git-crypt unlock`. Never overwrite an encrypted
  blob with plaintext.

## Commands

```bash
go test ./...        # full test suite, includes golden snapshots
go vet ./...
go run ./cmd/flat-modal        # run a sample app (needs a real TTY)
```

Golden snapshots have **no auto-update flag**: when a view change is
intentional, edit the golden file under the sample's `testdata/` by hand
(or rewrite it from the test's "actual" output) and re-run the test.
Goldens are ANSI-stripped and pinned to a fixed render width — keep them
deterministic (no wall-clock, no randomness in `View`).

## Architecture

- `internal/flatcore` — the runtime: `App[S]`, `Run`, event parsing,
  `StateUpdate[S]`/`Async`, `Tracer`, diff renderer. No app policy here.
- `internal/flatui` — opt-in widget state structs and layout helpers
  (`TextField`, `Card`, `Title`, `Subtle`, `Overlay`). Widgets own no
  goroutines, no hidden focus policy; apps store them in their own state.
- `internal/flatuitest` — golden-test helpers.
- `cmd/flat-*` — dogfood sample apps; each has tests + goldens.
- `cmd/bubble-modal`, `cmd/bubble-v2-modal` — Bubble Tea v1/v2 comparison
  apps. They are the benchmark; keep them compiling, don't "improve" them
  beyond parity with their Flatte counterparts.

## Principles (the short version — full rationale in the design doc)

- **Single source of truth.** State lives in one place; one write site per
  piece of state. If you find yourself mirroring a value, the design is
  wrong.
- **View is pure.** `state -> frame`, no retained render state in app code.
- **No policy in core.** The framework never decides what `q` or `j` mean.
  Key parsing is neutral; bindings are app code. (This was violated twice
  and both were bugs.)
- **No messages.** Apps never define message types. Async results are
  named, self-applying `StateUpdate`s.
- **All mutation on the loop goroutine.** Async work is a goroutine that
  sends one named update back. No mutexes on state, no
  goroutine-per-component.
- **Abstraction is found, not designed.** Extract a helper only after the
  pattern repeats across samples; record the evidence in
  `.docs/evaluation.md`. Every roadmap phase is a decision gate.
- **Agent-tractability is a design constraint.** A feature must be a
  local, compiler-visible edit. If adding something requires tracking
  non-local coupling, redesign it.

## Working conventions

- **Update `.docs/STATUS.md` in the same commit** as any change that adds,
  fixes, or removes something it lists. It is the answer to "what's done
  and what's left" — keep it honest, including the *Known bugs / debt*
  section.
- New findings from dogfooding (good or bad) get appended to
  `.docs/evaluation.md`, including what got worse. Honesty over advocacy.
- Tests are the verification path: `Handle` + field asserts, `StateUpdate`
  applies, golden views. Don't build a TTY harness to test logic.
- The Bubble Tea comparison apps exist to keep the claims honest — when a
  Flatte API changes shape, check whether the comparison table in
  `.docs/evaluation.md` is still true.
