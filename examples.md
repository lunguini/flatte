# Flatte Examples

Samples live in the nested `cmd` module. Run them from `cmd/`:

```bash
cd cmd
go run ./flat-pages
```

## Core Runtime And Async

| Sample | Demonstrates |
|---|---|
| `flat-spike` | `flatte.Go`, async load, app-owned list selection, mouse selection through zones |
| `flat-ticker` | `flatte.Every`, periodic updates, pause/reset |
| `flat-stream` | `flatte.Stream`, cancellation-aware stream source, deterministic stream tests |
| `flat-search` | `flatte.Latest`, stale result suppression, cancellable search |
| `flat-chat` | `flatte.WithInline`, `fx.Print`/`fx.Printf`, native terminal scrollback |
| `flat-game` | Real-time Snake: `flatte.ScopeEvery` tick pipeline with `Scope.Cancel` named ticker cancellation (pause/resume/restart), seeded-RNG deterministic state, `Chrome` board/panel panes, `Overlay` pause and game-over modals, `SaveState` high score |
| `flat-landing` | Interactive WASM landing showcase: one Flatte app core running in terminal and browser, with tabs, searchable component catalog, viewport detail pane, a `Frame.Root` page that scrolls (`layout.Scroll`) when the window is shorter than the shell, keyboard/mouse input, and a gzip-compressed standard Go WASM build. The browser build can post to an optional shared Snake leaderboard (see `leaderboard/`) that verifies each score by replaying the run server-side |

## Inputs And Editing

| Sample | Demonstrates |
|---|---|
| `flat-form` | Multiple `flatui.TextField` values, focus, real cursor placement |
| `flat-editor` | `flatui.Textarea`, multiline editing, soft-wrap, selection, Home/End line movement |
| `flat-keys` | Raw terminal key diagnostics for terminal-specific chords |

## Navigation And Composition

| Sample | Demonstrates |
|---|---|
| `flat-pages` | Multi-screen navigation with a screen enum |
| `flat-modal` | Overlay/modal state without a modal manager, as a `Frame.Root` tree of `Label` roles and `Box` cards; the modal's `TextField.Layout` input places the cursor, with no cursor arithmetic; `-gui` runs the same app in a native window (macOS) |
| `flat-tree` | `flatui.Tree`, search field, focus ring, keymap footer |
| `flat-workspace` | Capstone: three-pane layout-engine composition (tree, search, table, viewport, progress) with focus ring, grouped help, and a cursor placed from solved rects |

## Layout Engine

| Sample | Demonstrates |
|---|---|
| `flat-layout` | `flatui/layout` validation harness: returns a `Col`/`Row`/`Text` tree as `Frame.Root` (the runtime composes it at the terminal size and fills `MouseEvent.Target`) and toggles an `Overlay: true` modal; `-gui` runs it in a native window |
| `flat-scroll` | `layout.Scroll` panes taller than their window: a 40-row service list that keeps its selection visible and a log scrolled by wheel or PgUp/PgDn; offsets are app state clamped with `layout.ClampScroll`, rows are clicked through `MouseEvent.Target` at any offset |
| `flat-docker` | Flagship dogfood: returns one frame tree as `Frame.Root`; list heights, pane widths, and row zones are adopted from `LayoutEvent` before each paint, and pane clicks route on `MouseEvent.Target`/`TargetRect`; includes `TabBar.WithID`/`HitTest`, row/divider zones, drag-resizable panes, command bar, and confirm modal; `-gui` runs it in a native window as a cell grid |

## Widgets

| Sample | Demonstrates |
|---|---|
| `flat-reader` | `flatui.Viewport`, scrollable body, pinned chrome, mouse wheel |
| `flat-list` | `flatui.List`, keyboard/mouse selection, keep-visible scroll |
| `flat-spinner` | `flatui.Spinner` driven by `Every` |
| `flat-progress` | `flatui.Progress`, resize-owned width, pause/reset |
| `flat-table` | `flatui.Table`, aligned columns, selected row |
| `flat-timer` | `flatui.Timer` and `flatui.Stopwatch` |
| `flat-filter` | Filtered list composition with `TextField`, `List`, `Paginator`, and `KeyMap` |
| `flat-zones` | `flatui.ZoneMap`, explicit hit regions, local mouse coordinates |
| `flat-style` | Lip Gloss v2 styling, local palette, styled progress/table/card composition |

## Terminal Capabilities

| Sample | Demonstrates |
|---|---|
| `flat-capable` | Clipboard, suspend, exec, optional inline mode via `FLAT_CAPABLE_INLINE=1` |
| `flat-file-select` | `flatte.SelectFile`, platform-native file picker commands, terminal-delegated fallback |

## Bubble Tea Comparisons

These are comparison apps, not Flatte examples:

| Sample | Purpose |
|---|---|
| `bubble-modal` | Bubble Tea v1 modal comparison |
| `bubble-v2-modal` | Bubble Tea v2 modal comparison |
| `bubble-v2-search` | Bubble Tea v2 async search comparison |
