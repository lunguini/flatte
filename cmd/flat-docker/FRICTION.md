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

---

## Task 5 — Confirm modal for stop/remove

Predicted low–medium pain; confirmed low. The routing pattern is well-
trodden (`flat-modal` dogfoods it) and the feature-module shape extended
naturally to the modal: `confirmModel` has its own `Handle`, root
delegates with one `if s.modal != nil` guard.

### Routing (predicted low–medium — confirmed low)

- **`cmd/flat-docker/main.go` root `Handle`** — the modal-first guard is
  literally `if s.modal != nil { s.modal.Handle(s, ev, fx); return }`.
  Two lines. Background async still applies through the loop while the
  modal is open (Stats Every keeps ticking; Logs streamer keeps streaming)
  because the guard only intercepts *input*, not *updates*. **fine**.
- **`confirmModel.Handle`** is a 10-line key switch (y confirms, n/Esc
  cancels). No surprises. **fine**.

### Layout (positive finding for `flatui.Overlay`)

- **`flatui.Overlay(content, renderModal(m))`** worked first try, no
  arithmetic required. The base frame shows through where the modal
  doesn't cover; the modal is centered. `OverlayOrigin` exists for
  cursor placement but isn't needed here (modal has no text input).
  **fine** — this is the kind of "do less, work better" outcome the
  feedback's "abstraction is found, not designed" rule protects.
- **Modal styling** is plain Lip Gloss: `RoundedBorder`, fixed width,
  padding. ~8 lines. No friction.

### Feature-module shape (continued positive)

- **Root `State` owns `modal *confirmModel`**, the modal owns its own
  Handle, and `applyModalAction` lives next to it. The action target is
  captured by ID at open time, so even if selection drifts the action
  applies to the right container. The action's effect mutates
  `containers.containers[]` and calls `recomputeDetailWidgets` — same
  pattern as everywhere else. **No friction.**

### What this task did not yet exercise

- Mouse zones (Task 6).
- Cross-screen structure reuse (Task 7).

### Task 5 verdict

**Modal-over-complex-base is a non-issue in Flatte.** The feedback
flagged it as a place where users would "independently reinvent" routing,
but the pattern is one line and well-documented by existing samples.
This was correctly predicted low pain and the dogfood confirms it. The
running tally of "things the feedback worried about that turned out fine"
grows by one.

---

## Task 6 — Mouse: click rows + tab headers via ZoneMap

Predicted medium pain; **confirmed medium, in exactly the way the feedback
predicted**. ZoneMap works, but the application author measures rectangles
by hand and keeps them in sync with rendering by hand. This is the same
kind of friction as Task 2 layout math: not blocking, but every resize
re-touches it.

### Mouse zones (predicted medium — confirmed, in the predicted way)

- **`cmd/flat-docker/main.go` `registerMouseZones`** — runs on every
  `layout()` call. Walks the list's visible window registering one Rect
  per row at the *frame coordinates* the row will actually render at:
  `X: 0, Y: chromeRowsTop + 2 + i, Width: listPaneWidth, Height: 1`. Then
  registers three fixed Rects for the tab headers at
  `X: listPaneWidth + 2, Y: chromeRowsTop + 1`. ~15 lines. **This is
  exactly the friction the feedback and the post-0.1 roadmap flag**: the
  app computes its layout in `layout()`, then computes the *same layout
  again* as rectangles in `registerMouseZones`, then renders from cached
  state in `View`. Two sources of truth, kept in sync manually.
  **annoying** — and a third concrete site for the layout-vocabulary
  extraction candidate (Task 2 finding reinforced again): if `layout`
  returned a `Rect` per region, mouse zones would derive from it for free.
- **`List.height` is unexported.** To compute zone rectangles I needed
  the visible-window height; the public `List` API exposes `Cursor`,
  `Count`, `Offset`, `View`, but not `Height`. I added a `listHeight int`
  field to `containersScreen` and tracked it in `layout()`. **annoying**
  (mild — workaround is one line, but it's the second time an unexported
  widget field has required a shadow field on the screen struct, after
  Task 4's `Viewport.height`).
- **Threading `fx` through mouse handling.** `handleMouse` needs `fx` to
  call `onSelectionChange` (which restarts the scoped log streamer). Not
  hard — root passes `fx` along — but it means the mouse path cannot be a
  pure state mutation. **fine**.

### Better (positive)

- **`ZoneMap.At(x, y)`** is a clean lookup; once zones are registered,
  hit testing is one method call. **fine**.
- **Mouse mode opt-in via `flatte.WithMouse(MouseModeCellMotion)`** is a
  one-line change in `main()`. **fine**.
- **Mouse events route through the same `Handle` dispatch** as keys — no
  separate mouse pipeline. Type-switching on `flatte.MouseEvent` in
  `containersScreen.Handle` is clean. **fine**.
- **Mouse motion vs press distinction** is on the event (`m.Action`), so
  filtering presses-only is one line. **fine**.

### What this task did not yet exercise

- Cross-screen structure reuse (Task 7).

### Task 6 verdict

Mouse handling works, but the rectangle-bookkeeping friction is real and
exactly as predicted. This is the **third independent reinforcement** of
the layout-vocabulary extraction candidate: Tasks 2, 3, and 6 all do
version of the same "compute pane rectangles from arithmetic" pattern,
each in a different idiom (cached fields, widget sizing, mouse zones).
A `Rect`-returning `Split*` would close all three at once. Logged as
evidence for Task 8.

---

## Task 7 — Images + Volumes screens

Predicted to confirm the feature-module shape scales; **confirmed, plus
the layout-math friction hits its clearest "found" threshold**.

### Feature-module shape (positive, scales linearly)

- **`cmd/flat-docker/main.go` `imagesScreen` and `volumesScreen`** — each
  is a self-contained struct with `Handle`/`View`/`layout`/`keyHints`/
  `selected`/`renderListPane`/`renderDetailPane`, modeled on the same
  pattern as `containersScreen` minus tabs/async/mouse. **Root code did
  not change at all** to add them: `NewState` constructs them, root
  `Handle`/`View` switches on `s.screen`, `renderFooter` asks the active
  screen for `keyHints`. The feature-module shape's payoff is clearest
  here — adding a new screen is purely additive at the screen level.
  **No friction.**
- **Screen isolation is automatic.** `TestScreensAreIsolatedFromEachOther`
  verifies that cursor state on `containers` is preserved while the user
  navigates to `images` and moves there. Each screen owns its own state,
  and because there is no shared mutation site, isolation is the default.
  **fine**.

### Layout (the "found" threshold crossed)

- **Both `imagesScreen.layout` and `volumesScreen.layout` are byte-identical
  to `containersScreen.layout`'s pane-split math** (`listPaneWidth = min(30,
  max(width/3, 16))`, `detailPaneWidth = width - listPaneWidth - 2`,
  `list.SetHeight(height - listChromeRows)`). The same lines appear three
  times now. **This crosses the project's "abstraction is found, not
  designed" threshold** — the pattern is no longer found, it is plainly
  repeated. **annoying** — and the strongest evidence yet for the layout-
  vocabulary extraction candidate.
- **The `renderListPane`/`renderDetailPane` patterns are also repeated**,
  with only the data fields differing (Container vs Image vs Volume).
  Three near-identical ~15-line render methods. A generic "list+detail
  pane" helper would close this, but that's a heavier abstraction; logged
  as a maybe-candidate.

### What this confirms

- **Feature-module shape scales linearly** to multiple screens with no
  root-code growth. Adding a screen is purely additive. **The feedback's
  "feature module" recommendation is fully validated** — this is the
  pattern's strongest evidence.
- **Layout-math extraction is now overdue by the project's own rule.**
  Three copies of the same pane-split arithmetic is "found abstraction"
  by any reasonable reading. Task 8 should propose this as the #1
  post-0.1 extraction.
- **No new friction surfaced.** The patterns repeat; they do not get
  worse. That itself is informative: the dogfood is converging on a small
  set of repeated patterns, not discovering new ones.

### Task 7 verdict

Task 7 was the lowest-friction task in the dogfood: the feature-module
shape made adding two more screens purely additive, and the only friction
was the by-now-familiar layout-math repetition. **The dogfood's findings
have converged** — Tasks 8 should write them up.

---

## Final verdict (Task 8 synthesis)

The dogfood is complete: 7 tasks, ~1,500 LOC of sample code, 49 tests
passing, three goldens. This section consolidates the findings; the full
synthesis with concrete extraction proposals lives in `.docs/d03.md`.

### What the dogfood changed (the maintainer's initial review was wrong on these)

1. **Layout vocabulary.** Initial review: "already on the post-0.1
   roadmap; mild at one site." **Dogfood verdict: confirmed and
   reinforced four times** (Tasks 2, 3, 6, 7). The pattern crossed the
   project's own "abstraction is found, not designed" threshold at
   Task 7, where `imagesScreen.layout` and `volumesScreen.layout` are
   byte-identical to `containersScreen.layout`'s pane-split math.
2. **`flatte.Scope` for cancellable async.** Initial review: "a real
   gap, but minor; sidestep patterns may suffice." **Dogfood verdict:
   sharper than predicted.** `Every`/`Stream` cannot be cancelled by
   name today; only `Latest`/`Cancel`. The 30-line hand-rolled goroutine
   for scoped Logs in Task 4 is the sharpest single piece of dogfood
   evidence in the whole exercise. The sidestep pattern works but only
   by structuring the app to not need scoped lifetime.

### What the dogfood validated (initial review was right)

3. **Feature-module shape.** Initial review: "the project already
   practices this; quick-reference.md shows the screen-enum pattern."
   **Dogfood verdict: fully validated.** Adding two screens (Task 7)
   was purely additive at the screen level — root code did not change.
   The feedback's "feature module" recommendation is the dogfood's
   strongest positive finding and deserves to be documented in
   `quick-reference.md` without any framework code change.

### What the dogfood rejected (initial review was right to be cautious)

4. **`flatui.Tabs` widget.** Task 3 evidence: plain state (iota enum +
   two one-line wraparound methods + 14-line tab-bar renderer) handles
   tabs-within-pane cleanly. A widget would be more cost than savings.
5. **Modal manager / router helper.** Task 5 evidence: one-line guard.
   Existing `flat-modal` documents the pattern.
6. **Screen lifecycle hooks.** Task 4 sidestep pattern avoided needing
   them. Defer until an app's per-screen async cost makes them
   necessary.

### Smaller frictions worth fixing cheaply

- `Effects.context()` default is unexported (Task 4).
- `List.height` is unexported (Task 6).
- `Viewport.height` was unexported (Task 3) — already worked around via
  `VisibleLines()`, but the pattern repeats.

### Numbers

- ~1,500 LOC sample, 49 tests, 3 goldens.
- Layout math is ~30% of the screen code; with the proposed vocabulary
  it would drop to ~10%.
- Stats (sidestep pattern): 11 LOC. Logs (forced-cancellation): 30 LOC.
  Difference: the entire case for `flatte.Scope`.

### What this dogfood was *not*

- Not a TTY pass. The app is unverified in a real terminal; the dogfood
  is about *architecture friction*, not render correctness. Anyone
  proposing the extractions should run `flat-docker` in a real terminal
  first to confirm there are no surprises the test harness can't see.
- Not a Bubble Tea comparison. The dogfood tests Flatte against the
  feedback's predictions; it does not re-run the head-to-head against
  BT v1/v2 that `cmd/bubble-*` already covers.
- Not evidence for *every* TUI pattern. No drag-and-drop, no animations,
  no scroll-region management, no IME input. The findings generalize to
  "multi-pane apps with async and modals"; beyond that is unknown.

---

## Glamour pass — anchor-right pane + status line + sparkline (2026-06-25)

The user correctly noted the dogfood wasn't *glamorous* — no anchor-right
pane, no anchor-bottom status line, no animations. Adding them exposed
**more friction than Tasks 5–7 suggested had converged**, and reframes
the layout-vocabulary candidate from "important" to "urgent."

### Layout (the friction got worse, not better)

- **Three-pane layout multiplies the arithmetic.** Going from 2 panes to
  3 made the `layout()` method visibly hairier:
  ```go
  c.listPaneWidth = min(26, max(width/5, 14))
  c.activityPaneWidth = 22
  c.detailPaneWidth = max(width-c.listPaneWidth-2-c.activityPaneWidth-2, 0)
  ```
  Now four cached width fields per screen (`listPaneWidth`,
  `detailPaneWidth`, `activityPaneWidth`, plus implicit gaps). A fourth
  pane would add another. **The pattern scales linearly in pain** — each
  new pane adds another term to the width equation and another cached
  field. **blocked** at three panes; would be **prohibitive** at four.
- **Anchor-bottom status line within a pane.** Adding a status line at
  the bottom of the containers body (separate from the frame footer)
  required subtracting another row from `bodyContentHeight`:
  `bodyContentHeight := max(height-statusLineRows, 0)`, then *that*
  feeds into list height, viewport heights, etc. Every chrome row added
  forces every dependent height to recompute. **annoying** — this is
  the same arithmetic as the existing chrome-subtraction pattern, just
  one more layer.
- **Mouse zones got harder.** `registerMouseZones` now has to know
  that tab headers shifted left (because listPaneWidth shrank). The
  test `TestMouseClickTabHeaderSwitchesTab` broke because I'd
  hardcoded X=36 and the new layout put logs at X=28. **This is the
  drift problem the user asked about** — manual rectangles *will* drift
  from rendering whenever layout changes. The user's question about
  auto-zones is exactly the right response; see "Auto-zones prototype"
  section below.

### Custom data viz (no widget; hand-rolled)

- **Sparkline forced a hand-rolled renderer.** Flatte ships `Progress`
  (horizontal bar) but no graph/sparkline/Chart widget. I wrote a
  12-line `sparkline(history []float64, c color.Color) string` using
  Unicode block characters (`▁▂▃▄▅▆▇█`). Works fine. But it's the **first
  piece of "I had to build a widget from scratch"** friction in the
  dogfood — every viz type not in `flatui` is do-it-yourself.
  **fine** at this complexity (12 lines) but **logged as evidence** that
  the widget library post-0.1 should consider data-viz primitives.
- **History caching repeats the per-container-map pattern.** `cpuHistory
  map[string][]float64` and `memHistory map[string][]float64` are now
  the third and fourth per-container maps on `containersScreen`
  (joining `statsCache` and `liveLogs`). Each async source creates
  another. **annoying** — pattern repeats, no convention for "per-key
  state cache."

### Status line (positive — once the chrome math is done)

- **`recomputeStatusLine()` is a clean fold.** Aggregate over all
  containers' cached stats; format; done. Called from `tickStats` (Every
  fold) and `layout`. The hard part was reserving the row; the content
  was trivial. **fine**.
- **`lipgloss.NewStyle().Background(...).Width(w).Render(line)`** for
  the styled status bar is one line. No friction.

### Animation

- **There is no animation system.** The sparkline "animates" because
  `tickStats` appends a new value each second and View re-renders — but
  there is no easing, no interpolation, no transition. For a real
  lazydocker-style animated graph (smooth curve, fade-in), I'd have to
  hand-roll an easing loop in the Every fold and store intermediate
  values. **fine for what we built; would be blocked** for anything
  requiring smooth motion. Flatte's model assumes frames are cheap and
  state-driven; smooth animation requires storing fractional state and
  stepping it per-tick, which is doable but unsupported.

### Task 9–11 verdict

The glamour pass produced **two new pieces of evidence**:

1. **Layout vocabulary is more urgent than Task 8 suggested.** Three
   panes + a body-internal status line made the arithmetic notably
   worse; four panes would be prohibitive. The case for extraction
   strengthened from "important" to "urgent." A `Split*` returning
   `Rect`s would make this a one-liner.
2. **Custom data viz is a real gap.** Not urgent (sparkline was 12
   lines), but the widget library should grow viz primitives post-0.1.

It also produced **one strong reframing** of an existing finding: the
**manual-zone drift** problem (which broke a test when layout changed)
is the cleanest argument for **output-scanning auto-zones**. The user's
instinct that "components should auto-register" is right — and the
bubblezone-style approach (markers in rendered output, scan after View)
is the cleanest fit for Flatte's pure-View contract. Prototyped next.

---

## Cumulative summary (final)

(Updated each task. Predictions vs. observed.)

| Area | Prediction | Task 1 | Task 2 | Task 3 | Task 4 | Task 5 | Task 6 | Task 7 | Glamour |
|---|---|---|---|---|---|---|---|---|---|
| Layout math | High pain | Mild. | **Confirmed** — 3 sites. | Reinforced — 6 widgets. | (unchanged) | (unchanged) | Reinforced — zones too. | Reinforced — 3 screens. | **URGENT** — 4 width fields per screen, anchor-bottom chrome multiplies. |
| Scoped cancellation | High pain | — | — | — | **Confirmed sharper.** | — | — | — | (unchanged) |
| Tabs within pane | Medium | — | — | **Resolved cleanly.** | — | — | — | — | (unchanged) |
| Mouse zones | Medium | — | — | — | — | — | **Confirmed — manual Rects drift.** | — | **Worsened — layout change broke test.** |
| View composition | Medium | Mild. | Mild. | Fine. | Fine. | Fine. | Fine. | Fine. | Fine — three panes still readable. |
| Feature-module shape | Untested | Positive. | Strong positive. | Strong positive. | Strong positive. | Strong positive. | Strong positive. | **Linear scaling.** | (unchanged) |
| Keyboard routing | Low–medium | Low. | Low. | Low. | Low. | Low. | Low. | Low. | Low. |
| Polling (`Every`) | Low | — | — | — | **Low sidestep / high force.** | — | — | — | (unchanged) |
| Streaming (`Stream`) | Low | — | — | — | **Low sidestep / high force.** | — | — | — | (unchanged) |
| Modal routing | Low–medium | — | — | — | — | **Confirmed low.** | — | — | (unchanged) |
| Per-screen lifecycle | (not predicted) | — | — | — | Mild gap. | — | — | — | (unchanged) |
| Effects zero-value | (not predicted) | — | — | — | Annoying. | — | — | — | (unchanged) |
| Custom data viz | (not predicted) | — | — | — | — | — | — | — | **New gap — hand-rolled sparkline; library should grow viz primitives.** |
| Animation system | (not predicted) | — | — | — | — | — | — | — | **New gap — no easing/interpolation; only state-driven motion.** |

**Final final verdict:** the glamour pass did not change the *list* of
extraction candidates but it did change their *priority*. Layout
vocabulary is now urgent, not just important. Auto-zones via output-
scanning is now the cleanest fix for a problem the user identified and
the dogfood independently confirmed. Custom data viz and animation are
new gaps worth tracking post-0.1 but not urgent.

---

## Auto-zones prototype — `flatui.ZoneScanner` (2026-06-25)

User asked the right question after the glamour pass broke a mouse test:
*"do we need to manually assign mouse zones? can't we build them into
the components so they automatically register?"* Per the "Both — glamour
then auto-zones" choice, I prototyped `flatui.ZoneScanner` (bubblezone-
style output-scanning) in the framework itself, then refactored
flat-docker to use it.

### What landed

- **`flatui/zonescan.go`** — new experimental package: `Mark(id, content)`
  wraps content with OSC9 markers (zero display width, terminal strips
  them); `(*ZoneScanner).Scan(frame)` walks the rendered string ANSI-aware
  (handles CSI/OSC/DCS/SOS, multibyte UTF-8, wide runes — all without
  advancing x); `At(x,y)` / `Rect(id)` / `In(id,x,y)` mirror the existing
  `ZoneMap` API. 9 unit tests, including ANSI-inside-content and
  multibyte-rune width.
- **`flatest/golden.go`** — `ansiPattern` extended from CSI-only to also
  strip OSC/DCS/SOS sequences. Existing goldens unaffected (no current
  sample emits those sequences; flatte's runtime uses OSC 2 for window
  title but that's emitted by the renderer, not View).
- **`cmd/flat-docker`** — deleted `registerMouseZones` (17 lines of
  hand-computed `Rect`s); added `flatui.Mark` wrapping in the list-row
  render callback and the tab-bar renderer (4 added lines, inline);
  added `s.containers.zones.Scan(content)` in root `View` (1 line).

### Side-by-side friction comparison

| Concern | Before (Task 6 / Task G — `ZoneMap`) | After (auto-zones — `ZoneScanner`) |
|---|---|---|
| Setup cost per clickable region | Compute `Rect{X, Y, Width, Height}` from cached layout fields (4 numbers per region) | Wrap content with `Mark(id, content)` inline at the render call site (1 call per region) |
| Drift on layout change | **Real** — Task G broke a mouse test when `listPaneWidth` shrank; the cached rect was stale until `layout()` re-ran | **Impossible** — zones are recomputed from actual rendered bytes every `View`; whatever the layout math produces is what gets clicked |
| Drift on data change | Real — if list rows change width (e.g. new container names), zones need re-registering | Impossible — content-derived |
| Cross-screen sharing | Each screen owns its own `ZoneMap` and its own register helper | One `ZoneScanner` per screen; `Scan` is called by root `View` uniformly |
| Wide-rune / multibyte correctness | Handled implicitly (you sized by 1 col per row) | Handled explicitly via `runeWidth` in `Scan` |
| Per-frame cost | None — zones computed once per resize | O(frame bytes) per `View` — walks the rendered string |
| Test ergonomics | Direct: `Handle(mouseEvent)` after `resizedState` | Must call `View` first (mirrors production flow where runtime calls View every frame) |
| Lines of code (containers screen) | `registerMouseZones` 17 LOC + 4 cached width fields | 4 inline `Mark` calls + 1 `Scan` call in root View |

### Verdict on the user's question

**Yes, components could auto-register, and the cleanest fit for Flatte's
pure-`View` contract is output-scanning.** The prototype is small (~150
LOC in `zonescan.go`, mostly the ANSI/walk logic), the API is two
functions (`Mark`, `Scan`) plus the existing `ZoneMap`-shaped accessors,
and it **eliminates the drift class entirely**. The glamour-pass test
that broke when layout changed would not have broken under `ZoneScanner`,
because the zones would have been recomputed from the new rendered
positions automatically.

### Caveats / open questions for the prototype

- **Per-frame scan cost.** `Scan` walks the whole frame string every
  `View`. For flat-docker's ~2KB frames this is trivial; for an app
  producing 100KB frames at 60fps it might matter. Production should
  probably skip the scan when no mouse mode is active, or cache.
- **Marker visibility in raw output.** OSC9 is application-defined; some
  terminals might log it or treat it as a hyperlink or notification.
  Production may want to use a more clearly-zero-width marker (DCS with
  a private-use prefix, perhaps).
- **The runtime should own the scanner, not the app.** Today the app
  calls `Scan` in its `View`, which is mildly ugly (side effect in a
  function that should be pure). The clean shape is for the runtime to
  scan automatically after every `View` and expose the result via
  `RenderContext` or `Effects`. **That's a post-0.1 API change worth
  considering.**
- **No nested zones.** Two `Mark` calls cannot enclose each other; the
  scanner uses a single open/close pair. For the prototype this is fine;
  real apps wanting nested hit regions (icon inside a button) would need
  scanner changes.

### Net change to the dogfood's findings

This prototype **promoted auto-zones from "candidate" to "validated
extraction"** — the third concrete post-0.1 candidate alongside layout
vocabulary and `flatte.Scope`. The user's instinct that the manual-zone
work is unnecessary is correct, and the prototype is small enough to
ship. **The glamour pass breaking a mouse test was the trigger that
produced this evidence — which is itself a validation of the dogfood's
"build it, hurt, then extract" methodology.**
(Updated each task. Predictions vs. observed.)

| Area | Prediction | Task 1 | Task 2 | Task 3 | Task 4 |
|---|---|---|---|---|---|
| Layout math | High pain | Mild. | **Confirmed** — 3 sites. | Reinforced — 6 widgets. | (unchanged) |
| Scoped cancellation | High pain | Not yet hit. | Not yet hit. | Not yet hit. | **Confirmed and sharper than predicted — `Every`/`Stream` not cancellable; only `Latest`/`Cancel`. Feedback's `flatte.Scope` now justified.** |
| Tabs within pane | Medium | Not yet hit. | Not yet hit. | **Resolved cleanly.** | (unchanged) |
| Mouse zones | Medium | Not yet hit. | Not yet hit. | Not yet hit. | **Confirmed medium** — manual Rect measurement per resize; third reinforcement of layout-vocabulary candidate. |
| View composition | Medium | Mild. | Mild. | Fine. | Fine. |
| Feature-module shape | Untested | Positive. | Strong positive. | Strong positive. | Strong positive — fold logic lives next to its state. |
| Keyboard routing | Low–medium | Low. | Low. | Low. | Low. |
| Polling (`Every`) | Low | Not yet hit. | Not yet hit. | Not yet hit. | **Low for sidestep pattern; high for cancellable pattern (not natively supported).** |
| Streaming (`Stream`) | Low | Not yet hit. | Not yet hit. | Not yet hit. | **Low for sidestep pattern; high for cancellable pattern (not natively supported).** |
| Modal routing | Low–medium | Not yet hit. | Not yet hit. | Not yet hit. | **Confirmed low** — one-line guard; `flatui.Overlay` works first try. |
| Per-screen lifecycle | (not predicted) | — | — | — | Mild gap — `App.Init` is app-lifetime only; no screen enter/exit hooks. **Defer per Task 8 synthesis.** |
| Effects zero-value | (not predicted) | — | — | — | **Annoying** — `fx.Context` is nil on zero Effects; `Effects.context()` default is unexported. Cheap fix post-0.1. |

**Final verdict:** the dogfood **changed my position** on the feedback.
Two of three predictions (layout, scoped async) are now sample-driven
justified. One (feature module) was already right and is formally
validated. Two implied worries (tabs, router) were correctly rejected.
Full synthesis + concrete extraction proposals in `.docs/d03.md`.
