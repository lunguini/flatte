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
| Modal overlay | `flatui.Overlay` | `cmd/flat-modal` |
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

`Size` declares intent per axis; `Render` fills the exact `Rect` the parent
assigned. `Render` never chooses its own position or size. Concrete nodes embed
`NodeBase` for shared geometry and add their own fields.

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

| Call | Returns | Use |
|---|---|---|
| `layout.SolveAndCompose(root, w, h)` | `(content string, rects map[string]Rect)` | The frame path: one pass distributes rects and paints leaves into a cell buffer, so geometry and pixels cannot drift. Put `content` in `flatte.Frame`; keep `rects` for hit-testing. |
| `layout.Solve(root, w, h)` | `map[string]Rect` | Geometry only, no painting. Use to warm hit-test rects for a subtree that is not the frame this pass. |

`Rect` (`layout.Rect`, aliased as `flatui.Rect`) is `{X, Y, W, H}` in absolute
frame cells, with `Contains`, `Inset`, and `Intersect`.

### Overlays

Set `Overlay: true` on a node (or a container) to make it a centered layer. It
is skipped in the base distribution, then composed on top: its rect is cleared
and the node is recursed so descendants still get rects recorded. Overlay axis
size comes from `Fixed`/`Content` (exact, clamped to the viewport), `Auto`
(fills the viewport), or `Grow` (viewport fraction, per the weight rule above).
`layout.Overlay(base, layer)` is the standalone string compositor that centers
one rendered block over another with correct ANSI handling.

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

Because `SolveAndCompose` records every ID'd node's rect, mouse routing is a
map lookup — no coordinate math in app code. For tabs:

```go
// Build once; WithID tags the strip and each tab with a layout ID.
s.tabs = flatui.NewTabBar(items...).WithID("header")

// In View: the tab strip is a real child of the frame tree.
tree := layout.Col{Children: []layout.Node{s.tabs.Layout(), body, footer}}
content, rects := layout.SolveAndCompose(tree, w, h)
s.rects = rects // stash for Handle

// In Handle: map a click to a tab index using the solved rects.
if idx, ok := s.tabs.HitTest(s.rects, m.X, m.Y); ok {
	s.tabs.SetActive(idx)
}
```

For app-defined regions (rows, dividers), give the nodes IDs and look their
rects up in the returned map directly, or feed rects into a `flatui.ZoneScanner`
(`Reset`, then `Set(id, rect)` each frame, then `At(x, y)`) or `flatui.ZoneMap`.

## Async And Effects

| If you need... | Use... | Notes |
|---|---|---|
| One async request | `flatte.Go(fx, name, work, fold)` | Work runs off-loop; fold mutates state on-loop. |
| Periodic ticks | `flatte.Every(fx, name, interval, fold)` | App owns pause/reset policy. |
| Long-running source | `flatte.Stream(fx, name, source, fold)` | Source receives `context.Context` and `send(value)`. |
| Latest request wins | `flatte.Latest(fx, name, work, fold)` | Cancels and drops stale results by name. |
| Cancel latest request | `flatte.Cancel(fx, name)` | Use when input clears or screen changes. |
| Print above inline frame | `fx.Print(...)` / `fx.Printf(...)` | Requires `flatte.WithInline()`. |
| Shell out to editor/tool | `flatte.Exec(fx, name, cmd, fold)` | Releases terminal, runs command, restores. |
| File picker | `flatte.SelectFile(fx, name, cmd, fold)` | App chooses command; `cmd/flat-file-select` shows platform picker selection. |
| Clipboard write/read | `fx.SetClipboard`, `fx.ReadClipboard`, `flatte.ClipboardEvent` | Reads are best-effort; unsupported terminals may never answer. |
| Suspend | `fx.Suspend()` | Unix job-control when supported; no-op elsewhere. |

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

## Rules Of Thumb

- Do not add messages. Use events from Flatte and mutate app state directly.
- Do not hide key policy in widgets. Apps decide what keys mean.
- Do not start goroutines inside widgets. Use `flatte.Go`, `Every`, `Stream`,
  or `Latest` from app code.
- Use `Viewport` for too-tall content. The runtime does not turn arbitrary
  frames into scrollable regions.
- Use terminal delegation for tools the terminal ecosystem already solves well:
  editors, file pickers, pagers, and external workflows.
