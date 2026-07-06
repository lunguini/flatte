# Changelog

Generated from [Conventional Commits](https://www.conventionalcommits.org/)
by [release-please](https://github.com/googleapis/release-please); entries
below the first release are automated.

## 0.1.0 (2026-07-04)

First tagged release.

### Features

- Core runtime: `App[S]` single-writer loop over a closed event set
  (key/mouse/paste/focus/resize/clipboard), named self-applying
  `StateUpdate`s, capability effects (clipboard OSC52, exec, suspend,
  inline mode, file selection), `Frame{Content, Cursor, Title}` view
  contract with hardware cursor and window title.
- Async helpers: `Go`, `Every`, `Stream`, `Latest`, `Cancel`, and `Scope`
  for named cancellation of tickers and scoped work.
- `flatui` widget library: `TextField`, `Textarea` (soft-wrap, selection,
  grapheme-correct), `Viewport`, `List`, `Table`, `Tree`, `TabBar`,
  `FocusRing`, `KeyMap`, `Paginator`, `Progress`, `Spinner`, `Timer`,
  `Stopwatch`, `Card` helpers, `ZoneMap`/`ZoneScanner` hit regions.
- `flatui/layout` engine: `Row`/`Col`/`Text`/`Spacer` node trees, one-pass
  `SolveAndCompose` returning composed content plus per-ID rects for
  geometry-based hit-testing, centered `Overlay` nodes, custom container
  `Chrome`, symmetric and per-side padding.
- `flatest` deterministic test harness: fake-clock `Driver`
  (`Start`/`Send`/`Settle`/`Advance`), golden-frame assertions, recorder
  and replay.
- Samples: `flat-docker` (flagship), `flat-game` (real-time Snake),
  `flat-workspace` (capstone composition), `flat-layout`, and twenty
  focused single-capability samples; Bubble Tea comparison apps.
