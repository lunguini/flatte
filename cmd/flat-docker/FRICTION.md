# flat-docker Friction Log

Living record of where Flatte's current API made this app fight the framework,
and where it did not. The point is evidence for post-0.1 extraction decisions,
not complaints — every entry should be specific enough to act on.

Each entry: **location**, **what was needed**, **what would have helped**,
**severity** (blocked / annoying / fine).

Predicted high-pain areas from the original plan: layout, scoped cancellation.
Predicted low-pain: polling, streaming, modal routing. This log confirms or
refutes those predictions task by task.

---

## Task 1 — Scaffold (3-screen tab nav, shared chrome, resize)

### Layout

- **`cmd/flat-docker/main.go:78` `resize`** — body height is
  `max(height-chromeRowsTop-chromeRowsBottom, 0)`. Same arithmetic as
  `flat-workspace`'s `layout`. Fine at one site; will recur every time a
  nested pane needs the space left by its container. **annoying** (mild —
  single site for now).
- **`cmd/flat-docker/main.go:181` `placeholderBody`** — pads lines to fill
  height: `for len(lines) < height { append("") }`. Identical pattern to
  `flat-workspace`'s body padding. Every screen that wants bottom-aligned
  chrome (or just full-height fill) rewrites this loop. **annoying**.
- **`cmd/flat-docker/main.go:99` `View`** — `strings.Repeat("─", width)` for
  horizontal rules. Trivial in isolation, but every separator in every screen
  does this by hand. **fine** at this size.

What would have helped: a small layout vocabulary — `Rect`, `SplitHorizontal`,
`SplitVertical`, `Stack`, `Inset` — so the body rect is *computed* from the
frame rect minus chrome, and widgets receive bounds instead of recomputing
them. This is exactly the post-0.1 roadmap item in `.docs/d01.md:177`. The
friction is real but small at scaffold size; predicted to grow fast in Task 2
once there are real panes inside the body.

### Per-screen state shape

- **`cmd/flat-docker/main.go:130,149,165`** — `containersScreen`, `imagesScreen`,
  `volumesScreen` each carry their own `width, height int` and identical
  `layout(width, height)` method bodies. Three copies of the same two-line
  method. **annoying** (will get worse as screens gain more sized widgets).
  Could be fixed by an app-local base struct embedding, but that's exactly the
  kind of pattern a framework-supplied `Pane` or `Screen` concept would
  centralize — out of scope per ground rules; logged for Task 8 review.

### Routing

- **`cmd/flat-docker/main.go:43` `Handle`** — global keys checked first, then
  `switch s.screen` dispatches to the active screen's `Handle`. Same shape in
  `View` (`cmd/flat-docker/main.go:97`). Worked cleanly at 3 screens.
  **fine**. (Will revisit when the modal lands in Task 5 — that adds a second
  routing tier above the screen switch.)

### Feature-module shape (positive evidence)

- **`cmd/flat-docker/main.go:139,143` `containersScreen.Handle` /
  `containersScreen.View`** — the `(c *containersScreen) Handle(root *State,
  ev, fx)` signature the feedback recommended works exactly as advertised.
  Passing `root` lets a screen open a global modal or read shared state
  without coupling more than necessary. Each screen's logic stays local to its
  own file/section. **No friction** — this is positive evidence that the
  "feature module" idiom should be documented (not built into the framework)
  once Task 7 confirms it scales to multiple real screens.

### Test ergonomics

- **`cmd/flat-docker/main_test.go:12` `resizedState`** — tests have to send a
  `ResizeEvent` before asserting on `View`, because `State.height` is 0 until
  then. Not painful (helper absorbed it to one line), but every future test
  that wants non-zero body dimensions will need the same helper or repeat the
  dance. **fine**.

### Duplication that could drift

- **`cmd/flat-docker/main.go:90` `renderTabBar`** vs **`main.go:18` `Name()`** —
  screen labels live in two places (the `screen` enum's `Name()` method and
  the `renderTabBar` labels list). They agree today; nothing enforces it.
  **fine** at 3 screens; would matter more with a longer screen list.

---

## Cumulative summary

(Updated each task. Predictions vs. observed.)

| Area | Prediction | Task 1 observed |
|---|---|---|
| Layout math | High pain | Mild — single site per pattern. Will grow in Task 2. |
| Scoped cancellation | High pain | Not yet hit — Task 4. |
| Tabs within pane | Medium | Not yet hit — Task 3. |
| Mouse zones | Medium | Not yet hit — Task 6. |
| View composition | Medium | Mild — root `View` is a clean switch; screen bodies are tiny. |
| Feature-module shape | Untested | **Positive** — works as the feedback predicted. |
| Keyboard routing | Low–medium | Low — globals + screen switch is clean at 3 screens. |
| Polling (`Every`) | Low | Not yet hit — Task 4. |
| Streaming (`Stream`) | Low | Not yet hit — Task 4. |
| Modal routing | Low–medium | Not yet hit — Task 5. |

**Task 1 verdict:** the scaffold was easy. The architecture friction the
feedback worried about is not visible at this scale — but the *patterns that
will recur* (manual height math, per-screen sized-state duplication, body
padding loops) are already present and identical to `flat-workspace`. Whether
they cross from "fine" to "annoying" depends on Tasks 2–4.
