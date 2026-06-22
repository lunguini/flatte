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
| `flat-modal` | Overlay/modal state without a modal manager |
| `flat-tree` | `flatui.Tree`, search field, focus ring, keymap footer |
| `flat-workspace` | Capstone composition: tree, search, table, viewport, progress, focus, grouped help |

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
