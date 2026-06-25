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

## Task 2 — Containers list + filter + static detail, focus routing

This is where the layout friction the feedback predicted actually showed up.
The state shape and routing from Task 1 held up cleanly; the geometry math
is where the time went.

### Layout (predicted high pain — confirmed)

- **`cmd/flat-docker/main.go` `containersScreen.layout`** — every dimension
  is hand-computed: `listPaneWidth = min(30, max(width/3, 16))`,
  `detailPaneWidth = width - listPaneWidth - 2`, and `list.SetHeight(height -
  listChromeRows)` where `listChromeRows = 3` is a magic constant for
  "filter line + blank + blank". This is **exactly the pattern the feedback
  flagged** and exactly the pattern `flat-workspace:90` uses. It works. It
  also takes a non-trivial fraction of the screen code and will be **re-done
  for every screen that has multi-pane layout** (Tasks 5, 7). **annoying**,
  predicted to cross to **blocked** if a fourth nested pane is added.
- **`containersScreen.renderListPane` / `renderDetailPane`** — each pane is
  padded to its width via `lipgloss.NewStyle().Width(w).Render(content)`.
  Without that, `lipgloss.JoinHorizontal` aligns to the longest line in each
  pane and the columns drift. The pattern works but the application author
  has to know to do it — there is no "render this content into this Rect"
  helper. **annoying**.
- **The two-pane horizontal join itself is fine** — `lipgloss.JoinHorizontal(
  lipgloss.Top, listPane, "  ", detailPane)` reads cleanly. The pain is
  everything *around* it (sizing the widgets, padding the panes).
- **No `Rect` plumbing.** Each pane's width is a separate cached field on the
  struct (`listPaneWidth`, `detailPaneWidth`), recomputed in `layout`. If I
  added a third nested pane, I'd add another cached width field. The feedback
  was right that a `SplitHorizontal` / `SplitVertical` returning two `Rect`s
  would centralize this. **annoying** — single concrete extraction candidate
  now visible from this dogfood (logged for Task 8).

What would have helped: `body, _ := flatui.SplitVertical(bounds, footerH)`,
`list, detail := flatui.SplitHorizontal(body, 0.35)`, then `c.list.Resize(
list)` and `renderInto(detail, content)`. None of that exists today. This is
the post-0.1 layout vocabulary, now evidenced by a second sample.

### Focus routing (predicted low–medium — confirmed low)

- **`containersScreen.Handle`** switches on `key.Key` and dispatches by
  `c.focus.Focused(focusXxx)`. Clean and local. Tab/Shift-Tab/up/down/
  editing keys all routed correctly the first time. **fine**.
- **Key collision avoidance is the app's job** — `j` on list-focus moves the
  cursor, `j` on filter-focus edits the filter. Both branches live in the
  same `Handle`, which is correct but means the binding table is implicit in
  a switch statement, not declarative. **fine** at this scale; would become
  **annoying** with mode-specific bindings (vim insert/normal style).

### Feature-module shape (continued positive evidence)

- **`containersScreen.Handle(root, ev, fx)` + `View(root)`** continued to
  scale well even as the screen grew real widget state (FocusRing,
  TextField, List, filtered slice). The screen file/section is self-
  contained — root code did not need to change to add Task 2 features. The
  only root change was the footer pulling hints via a new `keyHints()`
  method. **No friction.**
- **`(c *containersScreen) keyHints() string`** is the cleanest example yet
  of the feature-module shape paying off: context-help for the whole screen
  lives next to the screen's input handling, not in a global help registry.

### Test ergonomics

- **`keyTab` vs `keyChar('\t')`** — Tab has to be sent as
  `flatte.KeyEvent{Key: flatte.KeyTab}`, not as a KeyCharacter with rune
  `\t`. This bit me in the first test pass (six tests failed with one root
  cause). The closed event set's distinction between `KeyTab` and
  `KeyCharacter` is correct, but there is no test helper that makes the
  common key spells terse. **annoying** (mild — `keyTab(shift bool)` is a
  4-line helper, but every new sample will rewrite it).
- **Focus order vs Tab direction.** I declared `focusFilter=0,
  focusList=1, focusDetail=2`, then in tests assumed Tab from list goes to
  filter — actually it goes to detail. Not Flatte's fault; just a reminder
  that focus-ring order is a UX decision the app owns, and getting it wrong
  silently is easy. A `FocusRing.Debug()` or some kind of order assertion
  might help, but is probably overkill. **fine**.

### What this task did not yet exercise

- Tabs within a pane (Task 3) — predicted medium pain.
- Scoped async cancellation (Task 4) — predicted high pain.
- Modal over a complex base (Task 5).
- Mouse zones (Task 6).

### Task 2 verdict

The layout friction the feedback predicted is real and **now sample-driven**
— three concrete sites (`layout`'s width math, per-pane padding via
`lipgloss.Width`, separate cached width fields per pane). The feature-module
shape continues to scale well; nothing in root code had to change to add
real widget state to one screen. Focus routing is clean. **Net: Task 2 took
more layout code than interaction code, which is the signal the feedback was
sending.**

## Cumulative summary

(Updated each task. Predictions vs. observed.)

| Area | Prediction | Task 1 | Task 2 |
|---|---|---|---|
| Layout math | High pain | Mild — single site per pattern. | **Confirmed** — three concrete sites (width split, pane padding, cached width fields). |
| Scoped cancellation | High pain | Not yet hit. | Not yet hit — Task 4. |
| Tabs within pane | Medium | Not yet hit. | Not yet hit — Task 3. |
| Mouse zones | Medium | Not yet hit. | Not yet hit — Task 6. |
| View composition | Medium | Mild — root `View` is a clean switch. | Mild — `JoinHorizontal` is fine; pain is the sizing around it. |
| Feature-module shape | Untested | Positive. | **Strong positive** — added widget state without touching root code. |
| Keyboard routing | Low–medium | Low. | Low — `switch key.Key` + `focus.Focused` reads cleanly. |
| Polling (`Every`) | Low | Not yet hit. | Not yet hit — Task 4. |
| Streaming (`Stream`) | Low | Not yet hit. | Not yet hit — Task 4. |
| Modal routing | Low–medium | Not yet hit. | Not yet hit — Task 5. |

**Running verdict:** the layout friction the feedback predicted is now
sample-driven and concrete (three sites after Task 2). The feature-module
shape scaled cleanly to a real screen with widget state. **Net so far: more
layout code than interaction code, which is the signal the feedback was
sending.** Task 3 (tabs within the detail pane) is the next unknown.
