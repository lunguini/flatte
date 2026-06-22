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
