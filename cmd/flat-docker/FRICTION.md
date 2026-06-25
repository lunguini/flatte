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

---

## Task 3 — Detail tabs (Stats / Logs / Inspect)

This was the green-field case: no existing sample demonstrates tabs *within*
a pane. The feedback listed it as a "medium" pain point and an unknown. The
dogfood verdict: **it resolved in Flatte's favor — plain state plus a
two-line render is enough, and no tab helper is justified.**

### Tabs within a pane (predicted medium — turned out fine)

- **`cmd/flat-docker/main.go` `detailTab` enum + `tab detailTab` field** —
  modeling tabs as a small iota enum stored on the screen struct is exactly
  the same pattern as the screen enum at root. Two `prevTab`/`nextTab`
  methods (each one line of modular arithmetic) handle wraparound. **No
  framework help wanted or needed.** fine.
- **`renderTabBar`** — 14 lines that look identical in structure to root's
  `renderTabBar` (which renders the screen tabs). The repetition is real but
  tiny: it's the cost of owning your own appearance, which Flatte
  guarantees. **fine** (mild "I'm rewriting the same loop again" feeling,
  but extracting a `flatui.RenderTabBar(labels, active)` helper would
  require defining a label struct, ordering rules, style hooks — more
  abstraction than the 14 lines it saves). Logged as a Task 8 maybe-candidate
  if it appears a third time.
- **Key routing for tab switching** — `]/[/h/l` routed only when
  `focus.Focused(focusDetail)`, dispatched in `handleChar`. Same shape as
  the list-movement routing added in Task 2. Cleanly composes with the
  existing focus ring. **fine**.
- **Per-tab scrolling** — `scrollActiveTab(delta)` switches on the active
  tab to call `LineUp/LineDown` on the right Viewport (`logs` or `inspect`;
  stats is not scrollable). Manual but obvious. **fine**.

### Layout (continued from Task 2)

- **Detail pane now has `detailChromeRows = 3`** (title + tab bar + blank)
  and the content area is sized as `c.detailPaneWidth-2` × `height-3`. The
  `logs` and `inspect` viewports and the `cpu`/`mem` Progress bars are all
  sized to this content rect. **This is now 6 sized widgets per screen**,
  all hand-aligned to one logical rect. Same arithmetic, multiplied. If I
  add a third scrollable tab the math reappears. **annoying** — reinforces
  the Task 2 layout finding.
- **No body-fill.** When a screen's content is shorter than the body height,
  nothing pads it to fill the terminal. Task 1's `placeholderBody` did this
  by hand; Tasks 2–3 don't, so the bottom separator moves up. Visually
  noticeable in a real terminal; logged for a future decision (root-level
  padding vs per-screen padding).

### View composition

- **`renderActiveTab(selected) string`** is a 3-way switch returning a
  string. Composes cleanly with the rest of `renderDetailPane`. The
  feedback's worry about a "giant View function" does not materialize at
  this size: per-screen methods + per-pane methods + per-tab methods
  decompose naturally. **fine**.

### Test ergonomics

- **`s.containers.logs.height` is unexported.** My first test draft reached
  into `Viewport.height` to gate a scroll assertion; replaced with
  `VisibleLines()` (the public accessor). The exported surface is right; the
  friction was me reaching past it. **fine**.
- **`TestScrollOnlyAffectsActiveTab` required long-enough inspect content.**
  The first cut had 5 inspect lines in an 18-row viewport, so scrolling
  couldn't engage and the test falsely failed. Expanded to ~25 lines of
  realistic-looking inspect data. Not Flatte's fault — just a test-data
  realism issue. **fine**.

### What this task did not yet exercise

- Scoped async cancellation (Task 4) — predicted high pain.
- Modal over complex base (Task 5).
- Mouse zones (Task 6).

### Task 3 verdict

**The green-field case the feedback was uncertain about resolved cleanly.**
Plain state + small renderers handle tabs-within-pane without framework
support. This is direct evidence *against* extracting a `flatui.Tabs` widget
at this scale: the cost is higher than the savings. The layout friction
continues to accumulate (now 6 sized widgets per screen) but is not yet
blocking. The next real test is Task 4 (scoped async cancellation), which
is the second predicted high-pain area.

---

## Task 4 — Async: Stats polling + Logs streaming + scoped cancellation

**This is the task that most changed my mind about the feedback.** I went
in expecting to confirm that Flatte's async model was a strength; I came
out confirming that **the feedback's `flatte.Scope` recommendation has real
merit**, more than I credited in my initial review. The dogfood did the
side-by-side comparison the user asked for (sidestep vs. force), and the
contrast is sharp enough that this single task may justify a post-0.1
extraction on its own.

### Setup: two patterns, side by side

Per the user's "both — sidestep then force" choice, Task 4 implements the
two async paths differently and lets the contrast speak:

- **Stats** uses the **sidestep pattern**: a single long-lived
  `flatte.Every(fx, "stats-poll", 1*time.Second, fold)` started in
  `App.Init`. The fold routes work to whichever container is currently
  selected by looking up `s.containers.selected()` and writing into a
  `statsCache map[string]containerStats`. No cancellation, no per-container
  goroutines.
- **Logs** uses the **forced-cancellation pattern**: a per-selection
  streamer is started by `startScopedLogs` whenever the selection changes.
  The previous streamer's `context.CancelFunc` is invoked before the new
  one starts. Each streamer is a hand-rolled goroutine using
  `flatte.Named` + `fx.Updates` because **`flatte.Stream` cannot be
  cancelled by name**.

### Better (positive evidence, mostly for the sidestep pattern)

- **The sidestep pattern is genuinely pleasant.** Stats ended up as 11
  lines of logic: `tickStats(_ time.Time)` reads `selected()`, updates the
  per-container map entry with a deterministic walk, and pushes the value
  to the visible `cpu`/`mem` widgets. The Every registration in `App.Init`
  is one line. No locks, no cancellation, no goroutine management.
  **fine** — and the per-container map was a natural place for the data to
  live anyway.
- **`App.Init` is the right place to start long-lived sources.** This is
  exactly what the existing `flat-stream` and `flat-ticker` samples do.
  No friction.
- **`flatte.Named` + `fx.Updates` is a usable escape hatch.** When
  `flatte.Stream` doesn't fit, an app can build the equivalent by hand and
  it still routes through the same loop. The pattern is verbose but not
  hidden.

### Worse (the predicted high-pain, confirmed and sharper than expected)

- **`flatte.Every` and `flatte.Stream` cannot be cancelled by name.**
  Only `flatte.Latest`/`Cancel` has per-name cancellation today. Every and
  Stream run for the **app's loop lifetime**, full stop. This is the
  sharpest finding of the dogfood so far: the feedback's `flatte.Scope`
  recommendation isn't a nice-to-have, it's a **real gap**, because the
  only way to get scoped streaming today is the 30-line hand-rolled
  goroutine I had to write for Logs. **blocked** (workable via the escape
  hatch, but the cost is real).
- **`Effects.context()` defaults to `context.Background()` internally but
  is unexported.** App code that wants to derive a child context has to
  defensively nil-check `fx.Context` because the zero `Effects` value
  (used everywhere in tests) has a nil Context. This bit me on the first
  test pass and required a `if parent == nil { parent = context.Background()
  }` guard. Trivial code, but it's friction the framework could remove by
  either exporting `context()` or by making the zero value default to
  Background via a getter. **annoying**.
- **The fold signature `func(*S, T)` (for Stream) and `func(*S, time.Time)`
  (for Every) cannot start new async work.** This rules out "self-chaining"
  patterns where a Latest fold kicks off the next poll. If I wanted to do
  Stats with `Latest` (cancellable) instead of `Every` (not cancellable), I
  would have to drive the polling from a long-lived goroutine anyway,
  re-implementing what `Every` should provide. **annoying** — and concrete
  evidence that the cancellation gap is not just missing API surface but a
  constraint on the existing API's composability.
- **Per-screen lifecycle isn't a concept.** `App.Init` starts app-lifetime
  work. There's no `Screen.Enter`/`Screen.Exit` hook, so async work
  specific to a screen has to be started from `Handle` when the user
  navigates there, with manual cancellation when they leave. For this
  dogfood (where async is containers-specific) the sidestep pattern
  avoided the issue by keeping the work running app-wide. For a real app
  where, say, the Images screen kicked off expensive scans, you'd want
  screen-scoped lifecycle. **annoying** at this scale; would be
  **blocked** in a larger app with expensive per-screen work.

### What would have helped (concrete extraction candidate now)

A `flatte.Scope` concept, exactly as the feedback proposed:

```go
type Scope struct {
    name    string
    parent  context.Context
    cancel  context.CancelFunc
}

func (s *Scope) Go(fx Effects[State], name string, work func(context.Context) (T, error), fold func(*State, T, error)) { ... }
func (s *Scope) Every(fx Effects[State], name string, interval time.Duration, fold func(*State, time.Time)) { ... }
func (s *Scope) Stream(fx Effects[State], name string, source func(context.Context, func(T)), fold func(*S, T)) { ... }
func (s *Scope) Cancel()  // cancels everything spawned through this scope
```

Then `startScopedLogs` becomes:
```go
s.containers.scope = flatte.NewScope(fx, "logs")
s.containers.scope.Stream(...)  // inherits the scope's cancellable context
// On selection change:
s.containers.scope.Cancel()
s.containers.scope = flatte.NewScope(fx, "logs")
s.containers.scope.Stream(...)
```

The savings would be ~25 lines per scoped source. **Task 4 alone produces
enough evidence to justify this extraction post-0.1.** Logged as the
second concrete extraction candidate (after layout vocabulary).

### What this task did not yet exercise

- Modal over a complex base (Task 5).
- Mouse zones (Task 6).

### Task 4 verdict

**This is the task that converts the feedback's `flatte.Scope` from a
maybe to a yes.** The side-by-side was decisive: Stats (sidestep, 11
lines, no friction) vs Logs (forced-cancellation, 30 lines, real
friction). The dogfood verdict on scoped cancellation flipped from my
initial review — the feedback was righter than I credited, and the
extraction candidate is now concrete and sample-driven. The closed
`Effects` struct (no public way to derive a child context from `fx`) is a
related gap that would be cheap to fix.
## Cumulative summary

(Updated each task. Predictions vs. observed.)

| Area | Prediction | Task 1 | Task 2 | Task 3 | Task 4 |
|---|---|---|---|---|---|
| Layout math | High pain | Mild. | **Confirmed** — 3 sites. | Reinforced — 6 widgets. | (unchanged) |
| Scoped cancellation | High pain | Not yet hit. | Not yet hit. | Not yet hit. | **Confirmed and sharper than predicted — `Every`/`Stream` not cancellable; only `Latest`/`Cancel`. Feedback's `flatte.Scope` now justified.** |
| Tabs within pane | Medium | Not yet hit. | Not yet hit. | **Resolved cleanly.** | (unchanged) |
| Mouse zones | Medium | Not yet hit. | Not yet hit. | Not yet hit. | Not yet hit — Task 6. |
| View composition | Medium | Mild. | Mild. | Fine. | Fine. |
| Feature-module shape | Untested | Positive. | Strong positive. | Strong positive. | Strong positive — fold logic lives next to its state. |
| Keyboard routing | Low–medium | Low. | Low. | Low. | Low. |
| Polling (`Every`) | Low | Not yet hit. | Not yet hit. | Not yet hit. | **Low for sidestep pattern; high for cancellable pattern (not natively supported).** |
| Streaming (`Stream`) | Low | Not yet hit. | Not yet hit. | Not yet hit. | **Low for sidestep pattern; high for cancellable pattern (not natively supported).** |
| Modal routing | Low–medium | Not yet hit. | Not yet hit. | Not yet hit. | Not yet hit — Task 5. |
| Per-screen lifecycle | (not predicted) | — | — | — | **Mild gap — `App.Init` is app-lifetime only; no screen enter/exit hooks.** |
| Effects zero-value | (not predicted) | — | — | — | **Annoying — `fx.Context` is nil on zero Effects; `Effects.context()` default is unexported.** |

**Running verdict:** the layout friction is confirmed and reinforced across
all layout-bearing tasks. **Task 4 flipped my opinion on scoped
cancellation** — the feedback was righter than I credited. Two concrete
extraction candidates now have sample-driven evidence: (1) layout
vocabulary, (2) `flatte.Scope` for cancellable async. Two related smaller
gaps surfaced: per-screen lifecycle hooks, and the unexported
`Effects.context()` default. Tabs-within-pane is settled in Flatte's favor.
Tasks 5–7 remain, all predicted lower pain.
