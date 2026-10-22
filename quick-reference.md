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
| A scrollable block of text | `flatui.Viewport` | `cmd/flat-reader` |
| Layout taller than its window (a pane or the whole page) | `layout.Scroll` | `cmd/flat-scroll`, `cmd/flat-landing` |
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
| Basic card layout | `flatui.CardLayout(children...)` as `Frame.Root` (a plain `layout.Col` with Card's look; fills the frame); string frames: `flatui.Card`, `flatui.CardBodyWidth`, `flatui.CardBodyHeight` | most samples |
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
| `Bordered` | `bool` | Draw a rounded border; adds one inset cell per side. Shorthand for `Box{Border: BorderRounded}`. |
| `Overlay` | `bool` | Treat this node as a centered overlay layer (see below). An overlay does not size its parent. |
| `Anchor` | `string` | For an `Overlay`: place it just below the node with this ID, left-aligned, above it when there is no room, inside the frame (dropdowns, context menus). An anchor not laid out centers it. |
| `Tooltip` | `string` | Help text a window shows while the pointer rests on the node (the innermost one with a tooltip); the terminal ignores it. |
| `ID` | `string` | Record this node's solved rect under `ID` for hit-testing. |
| `Chrome` | `func(Rect) string` | Custom container decoration (styled border, background, title) painted under the children in place of the default `Pad`/`Box` fill; layout inset still comes from `Pad` and the border. Terminal-only: prefer `Box` when it can say what you need. |
| `Box` | `Box` | Semantic decoration: `Border` (`BorderNone`, `BorderRounded`, `BorderNormal`), `BorderColor`, `Background`, `Selected`, `Round`, `Title`, `TitleStyle`. A border adds one inset cell per side; `Box.Border` wins over `Bordered`. `Background` is behind the content: leaf cells without their own background keep it. `Selected` makes a window highlight the node; the terminal adds nothing (the app's markers show selection). `Round` asks a window to round a background's corners (a pill when one line tall); the terminal draws it square. `Title` labels a border: the terminal writes ` Title ` in `TitleStyle` into the top rule, a window sets it in the stroke. |

### Size kinds

| Constructor | Kind | Meaning on the main axis |
|---|---|---|
| `layout.Auto()` | `SizeAuto` | No explicit claim; measurable nodes report their natural size instead. |
| `layout.Fixed(n)` | `SizeFixed` | Exactly `n` cells. |
| `layout.Grow(w)` | `SizeGrow` | Weighted share of leftover space; `w` is the weight relative to sibling grows. |
| `layout.Points(n)` | `SizePoints` | Exactly `n` points in a window; a terminal reads it at a nominal 8×16pt cell (`PointsPerColumn`, `PointsPerLine`), rounding up — `Points(200)` is 25 columns. For layouts designed for a window. |

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
| `layout.Label{Text: ..., Style: ...}` | Leaf of plain text with a semantic `TextStyle` (role + overrides); auto-sizes to it. Prefer it to `Text` for app text. `Wrap: true` wraps it by words at its width (cells in the terminal, the font in a window): it fills an Auto width, and its Auto height is the wrapped line count at the width it gets. `Runs: []layout.Run{{Text, Style}, ...}` makes it rich text (replacing `Text`; a run with a zero `Style` takes the Label's), wrapping across runs. `Selectable: true` lets a window user drag-select it and copy with ⌘C — the window draws the selection; the app still gets the clicks. |
| `layout.Icon{Name: ..., Text: ..., Style: ...}` | A small symbol: the terminal shows `Text` in `Style`; a window draws the named shape (`IconChevronRight`, `IconDisclosureClosed`/`Open`, `IconDot`, `IconCircle`; `IconBlank` draws nothing — terminal-only decoration) in the same columns. Larger symbols — `IconCheck`, `IconClose`, `IconPlus`, `IconMinus`, `IconSearch`, `IconGear`, `IconInfo`, `IconWarning`, `IconError`, `IconArrowUp`/`Down`/`Left`/`Right`, `IconEllipsis`, `IconFolder`, `IconFile` — are 0.7 of a line, centered: give them two columns (`"✓ "`). `IconChevronLeft`/`Up`/`Down` match `IconChevronRight`. Names are shapes; the app decides what they mean. `IconNone` draws `Text` everywhere. |
| `layout.Image{Source: img, Text: ..., Fit: ...}` | A picture: a window draws `Source` (`ImageContain`, `ImageCover`, `ImageStretch`), one pixel to a point on Auto axes; the terminal shows `Text`, subtle and wrapped — describe the picture. Size it with `Points`. Keep images in state: a window reads `Source` once, so give a new image to change it. |
| `layout.Button{Label: ..., Default: ..., Disabled: ..., Focused: ...}` | A push button: `[ Label ]` in the terminal (bold when `Default`, label reversed when `Focused`, subtle when `Disabled`), a native bezel in a window (the accent when `Default`). |
| `layout.Toggle{Kind: ..., Label: ..., Checked: ...}` | A checkbox (`[x]`), switch (`[ ●]`/`[○ ]`), or radio button (`(•)`), with its label; native indicators in a window. One-of-many for radios is the app's. |
| `layout.Slider{Value: ..., Fill: ...}` | A 0..1 value on a track: `━━━●───` in the terminal, a native slider in a window; fills the width on Auto. `ValueAt(x, width)` maps a press or drag. |
| `layout.Meter{Value: ..., Fill: ..., Track: ...}` | A horizontal gauge filled to `Value` (0..1): `█`/`░` cells in the terminal, a native bar in a window; fills the width on Auto, one line tall. |
| `layout.Input{Value: ..., Caret: ..., Focused: ...}` | One-line field: shows a value and caret (usually from `TextField.Layout`); fills the width, one line tall. The focused Input places the cursor. `Multiline: true` makes it a text area (usually from `Textarea.Layout`): `Wrap` soft-wraps, `SelStart`/`SelEnd` select bytes (shown in `SelectionStyle`, reverse video when zero), `OffsetX`/`OffsetY` are the widget's scroll, and its Auto height grows. |
| `layout.ScrollBar{Offset: ..., Visible: ..., Total: ..., Color: ...}` | Where a scrolled window is: `█` thumb on `░` track in the terminal (blank when everything fits), a thin native thumb in a window. One column wide, fills the height. |
| `layout.Text{String: ...}` | Leaf carrying a pre-rendered (ANSI) string; auto-sizes to it. The escape hatch for anything the primitives don't cover. |
| `layout.NewSpacer()` | Leaf that grows on both axes; `.WithBackground(c)` fills it. |
| `layout.Scroll{OffsetY: n, Child: ...}` | Shows `Child`, laid out at its natural height, through a window starting at row `OffsetY`. See [Scrolling](#scrolling). |

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

### Scrolling

`layout.Scroll` shows a child taller than the space it gets. Its inner box
(inside `Pad`/`Bordered`) is the window, and the child is laid out at its
natural height behind it: a `Fixed` height is exact, measured content is at
least the window (so wrapping a layout that fits changes nothing), and
`Grow`/`Auto` is the window. Painting and hit-testing are clipped to the
window, so rows scrolled out of view are neither drawn nor hittable. Overlays
inside a Scroll stay centered on the frame.

The offset is app state. Rects of nodes inside a Scroll arrive shifted by the
offset (rows above the window have negative `Y`), and the content size is the
child's own rect, so give the Scroll and its child IDs and clamp with the same
rule the layout paints with:

```go
layout.Scroll{
	NodeBase: layout.NodeBase{ID: "log"},
	OffsetY:  s.logY,
	Child:    layout.Text{NodeBase: layout.NodeBase{ID: "log.content"}, String: s.log},
}

case flatte.MouseEvent:
	if ev.Button == flatte.MouseWheelDown && ev.Target == "log.content" {
		s.logY = layout.ClampScroll(s.logY+3, s.rects["log"].H, s.rects["log.content"].H)
	}
case flatte.LayoutEvent:
	s.rects = ev.Rects
	s.logY = layout.ClampScroll(s.logY, s.rects["log"].H, s.rects["log.content"].H)
```

Clamping in state matters: an offset left past the end makes scrolling back
appear stuck. Put chrome (borders, padding) on a wrapping container so the
Scroll's rect is exactly its window. To keep a row visible, compare its
content-relative position (`row.Y - content.Y`, the same at every offset) with
the offset and window height. A whole page scrolls the same way, with a Scroll
as the root. `cmd/flat-scroll` shows panes; `cmd/flat-landing` shows a page.

### Text roles

`layout.TextStyle{Role: ...}` says what text is for; the backend decides how
it looks. Roles are `TextBody` (default), `TextTitle`, and `TextSubtle`;
`Fg`, `Bg`, `Bold`, `Italic`, `Underline`, and `Strikethrough` override the
role. `Size` sets the font size in points in a window (0: the role's size;
lines grow to fit it); terminals ignore it. `TextStyle.Render(s)` is the terminal look, and `flatui.Title` /
`flatui.Subtle` are those roles for string frames, so both paths match.

### Controls

Give controls IDs: clicks arrive as `MouseEvent`s targeting them, and keys
are the app's (Enter or Space on the focused control, a focus ring for Tab).
Controls hold no interaction state: `Checked`, `Value`, `Focused`, and
`Disabled` come from app state. In a window the host shows hover, press,
the pointer shape (a hand over controls, an I-beam over inputs), and
tooltips itself — no events, no app state. `cmd/flat-settings` uses every
control.

```go
layout.Toggle{NodeBase: layout.NodeBase{ID: "notify"}, Kind: layout.ToggleSwitch,
	Label: "Notifications", Checked: s.notify, Focused: s.focus.Focused(fNotify)}
...
case flatte.MouseEvent:
	if ev.Target == "notify" && ev.Action == flatte.MousePress {
		s.notify = !s.notify
	}
```

### Inputs and the cursor

Put a field in the tree with `TextField.Layout(id, focused)`. With
`Frame.Root` set and `Frame.Cursor` nil, the runtime places the cursor at the
caret of the topmost focused `layout.Input` that is visible — no origin or
label-width arithmetic in `View`:

```go
layout.Row{NodeBase: layout.NodeBase{Gap: 1}, Children: []layout.Node{
	layout.Label{Text: "name:"},
	s.name.Layout("name", s.focus.Focused(0)),
}}
```

A focused Input scrolls horizontally to keep its caret visible.
`Frame.CursorStyle` styles that cursor (for example a steady bar for an
editor). Set `Frame.Cursor` yourself when you need a cursor somewhere else;
it wins.
`cmd/flat-modal` shows the shape.

### Widgets in a layout tree

Several widgets expose `Layout() layout.Node`, so they slot straight into a
frame tree instead of being rendered and positioned by hand:

| Widget | `Layout()` produces |
|---|---|
| `flatui.TabBar` | A `Row` of tabs, each a `Row` of blank-in-a-window cap `Icon`s around a filled, padded `Label` on a `Round` box (the same cells in the terminal; spaced pills in a window), or one bordered `Text`; tags rects when `WithID` is set. |
| `flatui.List` | `ScrollTo(offset)` scrolls the window, keeping the cursor in it (for a scroll bar); `Scroll(rows)` moves the window alone, leaving the selection (for a wheel; the next move brings it back). `Rows(id, row)`: the visible rows as nodes — each wrapped in a `Row` with ID `id:index` (click `Target`) and `Box.Selected` on the selected one; the returned Col is the app's: give it an ID and `H: layout.Grow(1)` to size the list from its solved height. `Layout()`: a `Text` leaf of the visible rows via `RenderRow`. |
| `flatui.Table` | `HeaderLayout(style)` and `Cells(cells, style)`: Rows of column-width Labels one column apart (they line up in a window too); `Rows(id, row)`: body rows like `List.Rows`. |
| `flatui.Tree` | `Rows(id, row)`: the visible rows like `List.Rows`, each ID'd `id:<node ID>` (stable as the tree expands). |
| `flatui.Viewport` | A `layout.Scroll` over its lines at the current offset, as tall as the visible slice: a `Label` when plain (for `SetWrappedContent`, a wrapping `Label` of the raw text, so a window wraps it in its font), a `Text` when styled. Wrapped content breaks at words. Call `Adopt(rects, id)` with each `LayoutEvent` (id: the node holding `Layout()`) instead of `SetSize`: it sizes the viewport from that node and scrolls over the lines the host laid the text out in, so a window's scroll range, `AtBottom`, and `ScrollPercent` are its own; a viewport sent to the bottom stays there. |
| `flatui.Progress` | A `layout.Meter` of the bar's width and the percent label: the same text in the terminal, a native bar in a window. |
| `flatui.TextField` | `Layout(id, focused)`: a `layout.Input` showing the value and caret. |
| `flatui.Select` | A dropdown: `Layout(id, focused)` is a `Button` showing the value and ▾ plus, while open, its options as an overlay anchored under it (rows `id:option:i`); `Click(target, id)` opens/closes or chooses; `Open`/`MoveUp`/`MoveDown`/`Confirm`/`Close` for keys. |
| `flatui.Textarea` | `Layout(id, focused)`: a multi-line `layout.Input` with the value, cursor, selection, soft-wrap, and scroll — in the terminal the same cells as `ViewWithSelection`. Size it from the node's rect (`LayoutEvent`). In a window it wraps by words in the UI font; use the events' carets for clicks and ↑/↓ (`MoveTo`, `MoveToSelecting`), `InsertText` for pastes, `SelectAll`. |

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

Over an ID'd `layout.Input`, `MouseEvent.HasCaret` is set and `Caret` is the
byte offset into its value under the pointer, as the host laid the text out
(exact in cells, and in a window's proportional font) — pass it to
`Textarea.MoveTo` / `MoveToSelecting` for click and drag. In a window, a drag
that began on an Input keeps targeting it until release. While a multi-line
Input has focus, a window also sets `KeyEvent.HasCaret`/`Caret` on ↑/↓ to the
caret one of its visual rows away; terminals leave it unset (the widget's
rows are the terminal's):

```go
case flatte.KeyDown:
	if key.HasCaret {
		s.ta.MoveTo(key.Caret)
	} else {
		s.ta.MoveDown()
	}
```

`layout.ScrollBar.OffsetAt(y, height)` maps a press or drag on a bar to the
offset that centers its thumb there (`List.ScrollTo`, `Viewport.ScrollToLine`).

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

## Menus

Declare menus beside `View`, derived from state; choices arrive as
`MenuEvent{ID, Target}`:

```go
app := flatte.App[State]{State: s, Handle: Handle, View: View, Menus: Menus, ContextMenu: ContextMenu}

func Menus(s *State) []flatte.Menu {
	return []flatte.Menu{{Title: "Edit", Items: []flatte.MenuItem{
		{ID: "cut", Label: "Cut", Disabled: !s.hasSelection,
			Keys: []flatte.KeyEvent{{Key: flatte.KeyCharacter, Rune: 'x', Mod: flatte.ModSuper},
				{Key: flatte.KeyCharacter, Rune: 'x', Mod: flatte.ModCtrl}}},
		{Separator: true},
		{ID: "wrap", Label: "Wrap Lines", Checked: s.wrap},
	}}}
}

func ContextMenu(s *State, target string) []flatte.MenuItem {
	if target != "editor" {
		return nil // no menu: the right-click is delivered as usual
	}
	return editItems(s)
}
```

- `Keys` are shortcuts: every host delivers a matching key as the item's
  `MenuEvent` instead of a `KeyEvent` (a disabled item's key does nothing).
  List a ⌘ chord for windows and a Ctrl chord for terminals. A terminal
  can send only a plain Ctrl+letter — no Shift, and not Ctrl+I/M/H/[ (Tab,
  Enter, Backspace, Esc). Avoid plain letters — they would be taken from
  text fields too. `MenuShortcut` finds
  an item by key.
- A window shows `Menus` as its menu bar and `ContextMenu` as a native popup
  (right-click or Control-click). A terminal shows no bar; it draws the
  context menu over the frame at the pointer (↑/↓, Enter, →/← for submenus,
  Esc, mouse). An item with `Items` is a submenu; `Checked` shows a check.
- `flatest.Driver` routes menus as `Run` does, so tests can send keys and
  right-clicks and assert the `MenuEvent`, and goldens show the popup.
  `cmd/flat-editor` has Edit and View menus and an edit context menu.

## Dialogs And Drops

Dialogs are effects whose answers fold into state, like `Go`:

```go
case cmdOpen:
	flatte.OpenFile(fx, "open", flatte.OpenDialog{Title: "Open", Types: []string{"txt", "md"}},
		func(s *State, paths []string, err error) {
			if len(paths) > 0 { // none: cancelled; err: ErrNoDialogs on a host without dialogs
				load(s, fx, paths[0]) // capture fx to chain the next effect
			}
		})
```

`SaveFile(fx, name, SaveDialog{Name: "untitled.txt"}, fold(s, path, err))`
answers "" when cancelled. `Alert(fx, name, AlertDialog{Title, Message,
Buttons}, fold(s, button))` answers the button index (the first is the
default; Esc picks a button labelled "Cancel"; -1 when dismissed). A window
shows the system panels and alerts; a terminal draws them over the frame (a
path prompt with a directory listing and Tab completion; an alert with
buttons). In tests, `flatest.Driver.Dialog()` shows what the app asked for
and `d.Answer(flatte.DialogResult{Paths: ...})` / `{Button: i}` answers it.

A window delivers files or text dropped on it as `DropEvent{Paths, Text}`,
located (`X`, `Y`, `Target`, `TargetRect`) like a mouse event. A terminal
pastes a dropped file's path instead (`PasteEvent`).

## Windows

Declare secondary windows from state, like menus; `View` draws each one:

```go
app := flatte.App[State]{..., View: View, Windows: Windows}

func Windows(s *State) []flatte.Window {
	if !s.showStats {
		return nil // not declared: closed
	}
	return []flatte.Window{{ID: "stats", Width: 30, Height: 7, Autosave: "my-app.stats"}}
}

func View(s *State, ctx flatte.RenderContext) flatte.Frame {
	if ctx.Window == "stats" {
		return statsView(s) // its Frame.Title titles the window
	}
	return mainView(s)
}
```

- `WindowEvent{Window, Kind}`: `WindowFocused` says which window ("" the
  main one) now gets keys and pastes — keys are not tagged, so track it and
  route keys yourself; `WindowCloseRequest` is the close button: stop
  declaring the window to close it (or keep it to ask first).
- `MouseEvent`, `DropEvent`, and `LayoutEvent` carry `Window`; secondary
  windows get no `ResizeEvent` (their rects arrive as `LayoutEvent`s).
- A desktop window opens real windows (`MinWidth`/`MinHeight`, `Autosave`
  restores position and size; the Window menu has Minimize, Zoom, Full
  Screen); `window.Options` has the same fields for the main window. A
  terminal draws them as boxes over the main frame — `[×]` asks to close,
  a click focuses. `flatest.Driver.WindowFrame(id)` is a window's frame.

## Native Window

The `window` module (`github.com/lunguini/flatte/window`, macOS, no cgo) runs
the same `App[S]` in a desktop window; choose the host at the call site:

```go
err := window.Run(ctx, app, window.Options{Title: "My App", Width: 80, Height: 24})
```

`Run` must be called from the main goroutine. The window draws `Label`s in
the system font by role (titles semibold), `Box` borders as rounded strokes
(an overlay's box as a raised surface; a `Title` in the stroke), `Input`s as
outlined text fields with input-method support (multi-line ones as text
areas), and `ScrollBar`s as thin thumbs; ANSI `Text`, `Chrome`, and string frames show as
a monospace cell grid with line-drawing, block, and Powerline characters
drawn as shapes, so every app runs. Layout runs in points — `Fixed` sizes
are columns and lines (a line is the UI font's height plus line spacing),
`Pad`/`Gap` are 8pt steps, and a border takes a 12pt band with its stroke
down the middle, as a terminal border takes a cell — while `ResizeEvent`, `LayoutEvent`, and `MouseEvent` stay in
cells (`X - TargetRect.X` is exact inside the target). Ctrl-C goes to
`Handle`; quit with ⌘Q or by closing the window. ⌘←/⌘→ arrive as
`KeyHome`/`KeyEnd` (Shift kept), ⌘V as a `PasteEvent`, and other ⌘ chords
the menu does not own (⌘Q, ⌘W) as keys with `flatte.ModSuper` — copy, cut,
and select-all bindings are the app's. Fields' carets blink and stay inside
their fields. `window.Snapshot` renders
the first frame to an image for visual tests. Backend authors draw the ops
of `layout.LayoutScene`.

`Options.Theme` sets the look: colors for what the app leaves uncolored
(`Background`, `Surface`, `Text`, `Subtle`, `Title`, `Border`, `Field`,
`Accent` for focus, carets, and meters, `Selection`, `Track`), `Font` and
`MonoFont` families, `FontSize`, `TitleSize`, `LineSpacing`, and `Radius`
(negative: square corners). Start from `window.Dark()` or `window.Light()`
and change fields; zero fields keep the base's value. Nil follows the
system's light/dark appearance live; `Snapshot` draws `Dark` when nil.

```go
th := window.Light()
th.Accent = color.RGBA{0x2e, 0x7d, 0x32, 0xff}
err := window.Run(ctx, app, window.Options{Theme: &th})
```

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
| Clipboard write/read | `fx.SetClipboard`, `fx.ReadClipboard`, `flatte.ClipboardEvent` | Reads are best-effort; unsupported terminals may never answer. A window uses the system pasteboard (hosts route it with `flatte.WithHostClipboard`). Pastes arrive as `PasteEvent` (⌘V in a window). |
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

For a body of text, use `flatui.Viewport`; size it from `ResizeEvent` (or,
in a `Frame.Root` tree, with `Adopt` on `LayoutEvent`), not from `View`:

```go
case ev := ev.(type) {
case flatte.ResizeEvent:
	s.viewport.SetSize(ev.Width, ev.Height)
}
```

`View` should only render the current state. For a layout tree taller than the
window, use `layout.Scroll` (see [Scrolling](#scrolling)).

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
