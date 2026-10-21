# Flatte Examples

Samples live in the nested `cmd` module. Run them from `cmd/`:

```bash
cd cmd
go run ./flat-pages
```

## Core Runtime And Async

| Sample | Demonstrates |
|---|---|
| `flat-spike` | `flatte.Go`, async load, app-owned list selection as ID'd, `Box.Selected` rows; clicks route on `MouseEvent.Target` |
| `flat-ticker` | `flatte.Every`, periodic updates, pause/reset |
| `flat-stream` | `flatte.Stream`, cancellation-aware stream source, deterministic stream tests |
| `flat-search` | `flatte.Latest`, stale result suppression, cancellable search |
| `flat-chat` | `flatte.WithInline`, `fx.Print`/`fx.Printf`, native terminal scrollback |
| `flat-game` | Real-time Snake: `flatte.ScopeEvery` tick pipeline with `Scope.Cancel` named ticker cancellation (pause/resume/restart), seeded-RNG deterministic state, `Chrome` board/panel panes, `Overlay` pause and game-over modals, `SaveState` high score |
| `flat-landing` | Interactive WASM landing showcase: one Flatte app core running in terminal and browser, with tabs, searchable component catalog, viewport detail pane, a `Frame.Root` page that scrolls (`layout.Scroll`) when the window is shorter than the shell, keyboard/mouse input, and a gzip-compressed standard Go WASM build. The tab bar, prose, and component picker are layout nodes (each preview is the widget's own `Rows`/`Layout`, kept in state so clicks select rows and tabs, open tree nodes, place carets, and edit the text previews), so `-gui` runs it as native components; hosted apps stay terminal frames. The browser build can post to an optional shared Snake leaderboard (see `leaderboard/`) that verifies each score by replaying the run server-side |

## Inputs And Editing

| Sample | Demonstrates |
|---|---|
| `flat-form` | Multiple `flatui.TextField` values as `Input` nodes, focus; the focused field's caret places the cursor |
| `flat-settings` | A settings form of controls — `TextField.Layout`, a switch and a checkbox (`Toggle`, one disabled by the other), radio buttons, a `Slider` (click and drag via `ValueAt`), a `flatui.Select` dropdown (an overlay anchored under its button), and `Button`s with tooltips — driven by a focus ring and the mouse; `-gui` shows native controls with hover, press, and pointer shapes |
| `flat-gallery` | Sizes and text — `Points` sizes, `TextStyle.Size`, a rich-text `Label` (`Runs`) that wraps and is `Selectable` (drag and ⌘C in a window), three `layout.Image`s drawn in code with captions and a switchable `Fit` (the terminal shows their descriptions), and the icon set with terminal spellings; `-gui` draws the pictures, fonts, and icons natively |
| `flat-editor` | `flatui.Textarea`, multiline editing, soft-wrap, selection, Home/End line movement; `Textarea.Layout` sized from `LayoutEvent`, a blinking block cursor (a bar while selecting) via `Frame.CursorStyle`; click/drag via `MouseEvent.Caret`, paste, Edit (Cut/Copy/Paste/Select All with ⌘ and Ctrl shortcuts) and View (checked toggles) menus via `App.Menus`, an edit context menu via `App.ContextMenu` (native in a window, drawn by the runtime in a terminal), opt-in `-copy-on-select` (a mouse selection goes to the clipboard when the drag ends — for terminals such as Warp that never forward ⌘C), ⌘ arrows (Super, in terminals that report it), ctrl+c copy (quits with nothing selected); `-gui` runs it as a native text area (window-aware ↑/↓) |
| `flat-keys` | Raw terminal key diagnostics for terminal-specific chords |

## Navigation And Composition

| Sample | Demonstrates |
|---|---|
| `flat-pages` | Multi-screen navigation with a screen enum |
| `flat-modal` | Overlay/modal state without a modal manager, as a `Frame.Root` tree of `Label` roles and `Box` cards; the modal's `TextField.Layout` input places the cursor, with no cursor arithmetic; `-gui` runs the same app in a native window (macOS) |
| `flat-tree` | `flatui.Tree.Rows` with disclosure `Icon`s, ←/→ (h/l) collapse/expand or step to parent/child (app key policy), search filtering to matches and their ancestors (a second results `Tree`, so clearing restores the browse state), bordered panels sized by the frame, search `Input`, focus ring, keymap footer |
| `flat-workspace` | Capstone: three-pane `Frame.Root` composition (`Tree.Rows`, a search `Input` that places the cursor, `Table.Rows`, a `Meter`, a viewport) in bordered `Box`es, with focus ring, grouped help, and clicks (rows, the search caret, panes) and the wheel; `-gui` runs it as native components |

## Layout Engine

| Sample | Demonstrates |
|---|---|
| `flat-layout` | `flatui/layout` validation harness: returns a `Col`/`Row`/`Text` tree as `Frame.Root` (the runtime composes it at the terminal size and fills `MouseEvent.Target`) and toggles an `Overlay: true` modal; `-gui` runs it in a native window |
| `flat-scroll` | `layout.Scroll` panes taller than their window: a 40-row service list that keeps its selection visible and a log scrolled by wheel or PgUp/PgDn; offsets are app state clamped with `layout.ClampScroll`, rows are clicked through `MouseEvent.Target` at any offset |
| `flat-docker` | Flagship dogfood: returns one frame tree as `Frame.Root`; list heights, pane widths, and row zones are adopted from `LayoutEvent` before each paint, and pane clicks route on `MouseEvent.Target`/`TargetRect`; includes `TabBar.WithID`/`HitTest`, row/divider zones, drag-resizable panes, command bar (an `Input`), confirm modal (a `Box` with a `Title`), and draggable `ScrollBar`s; the logs and inspect viewports `Adopt` the host's line count; every pane is layout nodes (Labels, Icons, `List.Rows`, Meters, Viewports), so `-gui` runs it as native components |

## Widgets

| Sample | Demonstrates |
|---|---|
| `flat-reader` | `flatui.Viewport` (`Layout`: a `Scroll` of native text in a window) adopting its solved rect and the host's line count (`Adopt` on `LayoutEvent`), scrollable body, pinned chrome, mouse wheel; `-gui` runs it in a native window |
| `flat-list` | `flatui.List.Rows` sized from its solved height, `layout.Icon` selection marker (chevron in a window), keyboard/mouse selection by row `Target`, keep-visible scroll |
| `flat-spinner` | `flatui.Spinner` driven by `Every` |
| `flat-progress` | `flatui.Progress` sized from its solved rect (`LayoutEvent`), pause/reset |
| `flat-table` | `flatui.Table` as nodes (`HeaderLayout`, `Rows`, `Cells`: column-width Labels), selected row, click to select |
| `flat-timer` | `flatui.Timer` and `flatui.Stopwatch`, `Progress.Layout` (a native bar in a window); `-gui` runs it in a native window |
| `flat-filter` | Filtered list composition with `TextField` (an `Input` with a placeholder), `List.Rows`, `Paginator` sized from the list's solved height, and `KeyMap` |
| `flat-zones` | `flatui.ZoneMap` fed from ID'd panel rects (`LayoutEvent`), explicit hit regions, local mouse coordinates |
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
