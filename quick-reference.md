# Flatte Quick Reference

This file is written for both humans and AI coding agents. Start here when you
know what you want to build but not which Flatte API to use.

## Imports

The module path is `github.com/lunguini/flatte` and the package identifier is
`flatte`:

```go
import "github.com/lunguini/flatte"

// Use flatte.Run, flatte.App, flatte.Frame, ...
```

Widget and test helpers are separate packages:

```go
import (
	"github.com/lunguini/flatte/flatest"
	"github.com/lunguini/flatte/flatui"
)
```

## Core Shape

Use these names for every app:

| Need | Use |
|---|---|
| App definition | `flatte.App[S]` |
| Run app | `flatte.Run(ctx, app, options...)` |
| App state | One `State` struct owned by the app |
| Input handling | `Handle(s *State, ev flatte.Event, fx flatte.Effects[State])` |
| Rendering | `View(s *State, ctx flatte.RenderContext) flatte.Frame` |
| Quit | `fx.Quit()` |
| Window size | handle `flatte.ResizeEvent` |
| Keyboard | type switch to `flatte.KeyEvent` |
| Mouse | `flatte.WithMouse(...)` plus `flatte.MouseEvent` |
| Solved layout of a `Frame.Root` | handle `flatte.LayoutEvent` |
| Inline (non-fullscreen) mode | `flatte.WithInline()` |
| Handle Ctrl+C yourself | `flatte.WithoutDefaultQuit()` |

`Run` treats context cancellation, input EOF, and `fx.Quit()` as clean
shutdown and returns `nil` for those paths. It still restores terminal state
before returning; `OnExit` runs during cleanup, before that restoration, so
applications that report persistence failures should capture them in `OnExit`
and print them after `Run` returns. A context cancellation is not returned as
`context.Canceled`, is not an update-drain barrier, and `OnExit` observes only
the updates the loop applied before selecting cancellation.

Input limitations are explicit: F1–F12 key presses are supported, but key
release events and terminal capability/keyboard-enhancement reports are not
currently delivered. IME/composite key text is reduced to its first rune.
`WithInline()` and mouse reporting are mutually exclusive and return
`flatte.ErrInlineMouseUnsupported` from `Run`. Resize events use `SIGWINCH` on
Unix and ultraviolet Windows console events on a real console; cross-compiled
or piped Windows tests do not verify live console resizing.

## Choose The Right Helper

| If you need... | Use... | Sample |
|---|---|---|
| Text input, one line | `flatui.TextField` | `cmd/flat-form`, `cmd/flat-search` |
| Text input, multiple lines | `flatui.Textarea` | `cmd/flat-editor` |
| A scrollable body | `flatui.Viewport` | `cmd/flat-reader` |
| Selection in a vertical list | `flatui.List` | `cmd/flat-list` |
| Columnar rows | `flatui.Table` | `cmd/flat-table`, `cmd/flat-workspace` |
| Expand/collapse hierarchy | `flatui.Tree` | `cmd/flat-tree`, `cmd/flat-workspace` |
| Switch focus between panels | `flatui.FocusRing` | `cmd/flat-tree`, `cmd/flat-workspace` |
| Help/footer key metadata | `flatui.KeyMap` / `flatui.KeyGroups` | `cmd/flat-tree`, `cmd/flat-workspace` |
| Pagination state | `flatui.Paginator` | `cmd/flat-filter` |
| Progress bar | `flatui.Progress` | `cmd/flat-progress` |
| Loading animation | `flatui.Spinner` | `cmd/flat-spinner` |
| Countdown / stopwatch state | `flatui.Timer`, `flatui.Stopwatch` | `cmd/flat-timer` |
| Click regions | `flatui.ZoneMap` | `cmd/flat-zones`, `cmd/flat-spike` |
| Modal overlay | `layout.NodeBase{Overlay: true}` | `cmd/flat-docker` |
| Clickable tab strip | `flatui.TabBar` | `cmd/flat-docker` |
| Basic card layout | `flatui.Card`, `flatui.CardBodyWidth`, `flatui.CardBodyHeight` | most samples |
| Styled local composition | Lip Gloss v2 + `flatui` styled methods | `cmd/flat-style`, `cmd/flat-workspace` |
| Flexbox-style frame tree | `flatui/layout` (`Row`, `Col`, `Text`, `Spacer`) | `cmd/flat-layout`, `cmd/flat-docker` |

## Layout Engine

`github.com/lunguini/flatte/flatui/layout` is a flexbox-style engine for
building a whole frame as a tree of nodes, solving it once, and reusing the
solved rectangles for hit-testing. Import it as `layout`.

### The Node contract

Every visual thing is a `layout.Node` with two methods:

```go
type Node interface {
	Size() (w, h Size) // constraints handed to the parent's distribution
	Render(r Rect) string // draw inside the already-solved rect
}
```

`Size` declares intent per axis; `Render` receives the exact `Rect` the parent
assigned and draws only inside it. A leaf may under-fill its rect; the
compositor clips overflow and leaves uncovered parent pixels visible. `Render`
never chooses its own position or size. Concrete nodes embed `NodeBase` for
shared geometry and add their own fields.

### NodeBase fields

| Field | Type | Meaning |
|---|---|---|
| `W`, `H` | `Size` | Per-axis constraint (default `Auto`). |
| `Pad` | `int` | Inner padding on all sides. |
| `PadTop`/`PadRight`/`PadBottom`/`PadLeft` | `int` | Per-side padding added on top of `Pad` (inside the border) — for asymmetric insets like "header rows + left pad only". |
| `Gap` | `int` | Space between children (containers). |
| `Bordered` | `bool` | Draw a rounded border; adds one inset cell per side. |
| `Overlay` | `bool` | Treat this node as a centered overlay layer (see below). |
| `ID` | `string` | Record this node's solved rect under `ID` for hit-testing. |
| `Chrome` | `func(Rect) string` | Custom container decoration (styled border, background, title) painted under the children in place of the default `Pad`/`Bordered` fill; layout inset still comes from `Pad`/`Bordered`. |

### Size kinds

| Constructor | Kind | Meaning on the main axis |
|---|---|---|
| `layout.Auto()` | `SizeAuto` | No explicit claim; measurable nodes report their natural size instead. |
| `layout.Fixed(n)` | `SizeFixed` | Exactly `n` cells. |
| `layout.Grow(w)` | `SizeGrow` | Weighted share of leftover space; `w` is the weight relative to sibling grows. |

`SizeContent` is produced internally when an `Auto` axis measures natural
content (e.g. `Text`); callers do not author it. On the cross axis, anything but
`Fixed` stretches to fill the parent — pin a hit target's cross axis with
`Fixed` when it must be exactly content-sized.

`Grow` weight has a second meaning for overlays: `Grow(0.6)` sizes the overlay
to 60% of the viewport on that axis, while `Grow(1)` (weight >= 1) fills the
whole viewport.

### Containers and leaves

| Node | Role |
|---|---|
| `layout.Row{Children: ...}` | Distribute children horizontally (main axis = width). |
| `layout.Col{Children: ...}` | Distribute children vertically (main axis = height). |
| `layout.Text{String: ...}` | Leaf carrying a pre-rendered string; auto-sizes to it. |
| `layout.NewSpacer()` | Leaf that grows on both axes; `.WithBackground(c)` fills it. |

### Solving a frame

The simplest path is to return the tree and let the runtime solve it:

```go
func View(s *State, _ flatte.RenderContext) flatte.Frame {
	return flatte.Frame{Root: s.tree()} // composed at the terminal size
}
```

With `Frame.Root` set, the runtime composes the tree at the terminal width
and height (alt screen) or at the root's own height (inline), ignores
`Content`, and fills `MouseEvent.Target` with the ID of the topmost ID'd node
under the pointer (and `TargetRect` with that node's solved rect) — so `View`
needs no size bookkeeping and `Handle` needs no hit-test solve. Before
painting, it delivers a `LayoutEvent` whenever the solved rects changed (see
[Geometry in state](#geometry-in-state)). `flatest.Driver` does all of this the
same way, at its width and the last `ResizeEvent` height: both go through
`flatte.SettleFrame` (and `flatte.ComposeFrame`), which an app hosting another
app can call too.

Call the solver yourself when you need the geometry somewhere else:

| Call | Returns | Use |
|---|---|---|
| `layout.Compose(root, w, h)` | `Composition{Frame, Rects, Diagnostics}` + `HitTest(x, y)` | One solve+paint walk with hit-testing in paint order: children beat parents, overlays beat the base, and an overlay area with no ID'd node absorbs the hit (returns `""`). |
| `layout.SolveAndCompose(root, w, h)` | `(content string, rects map[string]Rect)` | The frame path: one pass distributes rects and paints leaves into a cell buffer, so geometry and pixels cannot drift. Put `content` in `flatte.Frame`; keep `rects` for hit-testing. |
| `layout.Solve(root, w, h)` | `map[string]Rect` | Geometry only, no painting. Run it from event-time reflow to prepare hit-testing before the next event. |
| `layout.SolveWithDiagnostics` / `SolveAndComposeWithDiagnostics` | rects/content plus duplicate-ID reports | Opt-in diagnostics for repeated non-empty IDs; existing calls retain last-ID-wins behavior. |

`Rect` (`layout.Rect`, aliased as `flatui.Rect`) is `{X, Y, W, H}` in absolute
frame cells, with `Contains`, `Inset`, and `Intersect`.

Both solve paths return empty content/geometry for a non-positive viewport.
Negative/extreme padding and intrinsic sizes saturate, and composition clamps
padding to the resolved rectangle before calling Lip Gloss.

### Overlays

Set `Overlay: true` on a node (or a container) to make it a centered layer. It
is skipped in the base distribution, then composed on top: its rect is cleared
and the node is recursed so descendants still get rects recorded. Overlay axis
size comes from `Fixed`/`Content` (exact, clamped to the viewport), `Auto`
(fills the viewport), or `Grow` (viewport fraction, per the weight rule above).
`layout.Overlay(base, layer)` is a lower-level string compositor for tiny local
blocks. Prefer an overlay node when composing an app frame.

### Widgets in a layout tree

Several widgets expose `Layout() layout.Node`, so they slot straight into a
frame tree instead of being rendered and positioned by hand:

| Widget | `Layout()` produces |
|---|---|
| `flatui.TabBar` | A `Row` of tab leaves (or one bordered `Text`); tags rects when `WithID` is set. |
| `flatui.List` | A `Text` leaf of the visible rows via `RenderRow`. |
| `flatui.Viewport` | A `Text` leaf of the visible slice. |
| `flatui.Progress` | A `Text` leaf of the bar. |

### Geometry-based hit-testing

With `Frame.Root`, route on the target ID directly:

```go
case flatte.MouseEvent:
	if ev.Action == flatte.MousePress && ev.Target == "sidebar" {
		s.focus = paneSidebar
	}
```

`TargetRect` makes the click node-local without a rect lookup:

```go
if ev.Target == "list" {
	s.list.Select(s.list.Offset() + ev.Y - ev.TargetRect.Y)
}
```

### Geometry in state

When widget state depends on solved geometry — a list whose visible row count
comes from its pane, a detail pane's width, a hosted app sized to a pane —
handle `flatte.LayoutEvent` the way you handle `ResizeEvent`:

```go
case flatte.LayoutEvent:
	s.rects = ev.Rects // keep it if other code needs geometry later
	s.list.SetHeight(ev.Rects["list"].H - 2)
```

With `Frame.Root`, the runtime solves the tree and, if the ID'd rects differ
from the ones it last delivered, delivers them **before painting**, then
renders again — up to two times per frame, which covers the usual chain
(pane size → list rows → row rects). The frame on screen therefore already
reflects adopted sizes. Geometry still changing after two passes is painted
as-is and re-delivered on the next render, so a non-converging app cannot
stall the loop. Effects queued while handling a `LayoutEvent` run right after
that frame.

A `LayoutEvent` arrives only when geometry **changes**. If your state starts
consuming geometry without it changing — for example switching between two
screens with identical layouts — adopt the rects from the last `LayoutEvent`
you kept; unchanged, they still describe the frame.

Without `Frame.Root` (string frames), solve at event time yourself. Because
`SolveAndCompose` records every ID'd node's rect, that path is a map lookup
too — no coordinate math in app code. For tabs:

```go
// Build once; WithID tags the strip and each tab with a layout ID.
s.tabs = flatui.NewTabBar(items...).WithID("header")

// Use one tree builder for event-time geometry and read-only rendering.
func (s *State) tree() layout.Node {
	return layout.Col{Children: []layout.Node{s.tabs.Layout(), body, footer}}
}

// In Handle, after ResizeEvent or any mutation that changes geometry:
s.rects = layout.Solve(s.tree(), s.width, s.height)

// In View: compose the same prepared tree without mutating state.
content, _ := layout.SolveAndCompose(s.tree(), s.width, s.height)

// In Handle: map a click to a tab index using the solved rects.
if idx, ok := s.tabs.HitTest(s.rects, m.X, m.Y); ok {
	s.tabs.SetActive(idx)
}
```

For app-defined regions (rows, dividers), give nodes IDs and look their rects
up directly, or feed rects into `flatui.ZoneMap` (`Clear`, then `Set(id, rect)`,
then `At(x, y)`). `ZoneScanner` remains as deprecated compatibility API.

## Async And Effects

Application code should use the helpers below. The deprecated raw-channel
`Async` bridge was removed in the 2026-08 breaking stabilization increment;
use `Go` or off-loop `Effects.Send`. `NewHarnessEffects` and
`NewHarnessEffectsQueue` remain test-only construction seams for external
deterministic harnesses, not app APIs; they will be reviewed for consolidation
or relocation before 1.0.

| If you need... | Use... | Notes |
|---|---|---|
| One async request | `flatte.Go(fx, name, work, fold)` | Work runs off-loop; fold mutates state on-loop. |
| Custom state update | `fx.Send(update)` | Off-loop escape hatch; may block for backpressure and returns false when cancelled or not configured. Handle/View/folds must not call it. |
| Periodic ticks | `flatte.Every(fx, name, interval, fold)` | Returns `ErrInvalidInterval` for a non-positive interval. App owns pause/reset policy. Runs for the loop's lifetime — if it must stop before quit, use `ScopeEvery` instead. |
| Long-running source | `flatte.Stream(fx, name, source, fold)` | Source receives `context.Context` and `send(value)`. Same lifetime caveat as `Every`. |
| Latest request wins | `flatte.Latest(fx, name, work, fold)` | Cancels and drops stale results by name. |
| Cancel latest request | `flatte.Cancel(fx, name)` | Only cancels work started with `Latest`. It does **not** stop `Every`/`Stream`/`Go` — for those, use a `Scope`. |
| Cancellable group of work | `flatte.NewScope(fx, name)` + `flatte.ScopeGo`/`ScopeStream`/`ScopeEvery` | `ScopeEvery` returns `ErrInvalidInterval` for a non-positive interval. `scope.Cancel()` stops everything started through the scope (e.g. pause/restart a game tick — `cmd/flat-game`). Create a fresh scope to start again. |
| Persist state across runs | `flatte.SaveState(path, s)` / `flatte.LoadStateWithError(path, defaultState)` | Gob-encoded snapshot. Refuse or explicitly quarantine corrupt state before saving; the flagship samples preserve unreadable files. Unix syncs the containing directory after rename, while other platforms provide file sync + rename without portable directory fsync. |
| Print above inline frame | `fx.Print(...)` / `fx.Printf(...)` | Requires `flatte.WithInline()`. |
| Shell out to editor/tool | `flatte.Exec(fx, name, cmd, fold)` | Releases terminal, runs command, restores. |
| File picker | `flatte.SelectFile(fx, name, cmd, fold)` | App chooses command; `cmd/flat-file-select` shows platform picker selection. |
| Clipboard write/read | `fx.SetClipboard`, `fx.ReadClipboard`, `flatte.ClipboardEvent` | Reads are best-effort; unsupported terminals may never answer. |
| Suspend | `fx.Suspend()` | Releases and restores terminal state; Unix sends job-control suspend when supported. Apps should not assume a process stop on platforms without job control. |

## Navigation Recipes

### Multiple Pages

Use a screen enum:

```go
type screen int

const (
	screenHome screen = iota
	screenDetails
)

type State struct {
	screen screen
}
```

Switch in `Handle` and `View`. Mutate `s.screen` to navigate. See
`cmd/flat-pages`.

### Multiple Sections On One Page

Use `flatui.FocusRing`:

```go
type State struct {
	focus flatui.FocusRing
}

func NewState() *State {
	s := &State{}
	s.focus.SetCount(3)
	return s
}
```

Bind Tab/Shift-Tab to `Next`/`Prev`, then branch input by `Focused(i)`. See
`cmd/flat-workspace`.

### Scrollable Page Body

Use `flatui.Viewport`; size it from `ResizeEvent`, not from `View`:

```go
case ev := ev.(type) {
case flatte.ResizeEvent:
	s.viewport.SetSize(ev.Width, ev.Height)
}
```

`View` should only render the current state.

## Testing Recipes

| Need | Use |
|---|---|
| Test key behavior | Call `Handle` directly and assert fields |
| Test async folds | `flatest.Driver` with `Settle()` |
| Test timers | `flatest.Driver.Advance(duration)` |
| Test frames | `flatest.AssertGoldenFrame` |
| Test frame sequences | `flatest.AssertFrames` |
| Strip ANSI for assertions | `flatest.CleanFrame` |
| Record/replay an event script | `flatest.Recorder` + `flatest.Replay` |

`flatest.Driver` is a deterministic logic harness, not a terminal emulator.
It drains queued updates synchronously to completion, whereas `Run` drains a
bounded snapshot per paint; compare converged state and frames, not redraw
cadence. `Driver.Frame` uses a fixed width and a zero-value color profile, and
does not model the renderer, terminal input/signals, or capability actions such
as suspend, exec, clipboard, and inline scrollback. `Recorder` stores event and
update names, not update payloads or `Settle`/`Advance` cadence; use `Driver`
directly for supersession and timing scenarios. Its update queue is intentionally
unbounded for deterministic convergence, so a malicious or erroneous
synchronous source can allocate memory proportional to its emissions.

Widget width/height setters clamp to 0..65,536 cells. This ceiling is far above
real terminal dimensions and prevents malformed public values from reaching
unbounded allocation or repetition primitives.

## Rules Of Thumb

- Do not add messages. Use events from Flatte and mutate app state directly.
- Do not hide key policy in widgets. Apps decide what keys mean.
- Do not start goroutines inside widgets. Use `flatte.Go`, `Every`, `Stream`,
  or `Latest` from app code.
- Use `Viewport` for too-tall content. The runtime does not turn arbitrary
  frames into scrollable regions.
- Use terminal delegation for tools the terminal ecosystem already solves well:
  editors, file pickers, pagers, and external workflows.
