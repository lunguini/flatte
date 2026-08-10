# Changelog

Generated from [Conventional Commits](https://www.conventionalcommits.org/)
by [release-please](https://github.com/googleapis/release-please); entries
below the first release are automated.

## [0.1.1](https://github.com/lunguini/flatte/compare/v0.1.0...v0.1.1) (2026-08-10)


### Bug Fixes

* **flat-file-select:** write the selector test in the platform's shell dialect ([d4d128d](https://github.com/lunguini/flatte/commit/d4d128d909557d46ff2d4f1729bbc44c95d9f059))
* reuse the input pipeline when it cannot be stopped ([711aa9b](https://github.com/lunguini/flatte/commit/711aa9b962d3c73c37f4a1950a35968cc9caabd9))
* unblock Run when the input read cannot be cancelled, patch stdlib vuln ([93d227e](https://github.com/lunguini/flatte/commit/93d227edd04290d6446d81b729b795028f2b5563))
* unblock Run when the input read cannot be cancelled, patch stdlib vuln ([b71fd23](https://github.com/lunguini/flatte/commit/b71fd23707f6f8eda090d9a95af8cf044e72c254))

## 0.1.0 (2026-07-18)


### ⚠ BREAKING CHANGES

* **flatui:** removed flatui.Mark, ZoneScanner.Scan, TabBar.HandleMouseAt, TabBar.TotalWidth, TabBar.TabStartX, TabLabelWidth, Rect.Local; flatui.Rect field names changed from Width/Height to W/H (now = layout.Rect).
* the import path changes from github.com/lunguini/flat to github.com/lunguini/flatte and the package identifier changes from flat to flatte.

### Features

* add KeyPageUp/KeyPageDown, List.Height(), Effects.Ctx() — framework fixes from dogfood ([de41c9f](https://github.com/lunguini/flatte/commit/de41c9fe7abd1b88fcd3131b3b2f3df54d213aae))
* **cmd/flat-layout:** validation harness with colored blocks + state loop ([4fd62f8](https://github.com/lunguini/flatte/commit/4fd62f8c624553401dd2988e73cd42eb6dff23b8))
* **cmd:** add filtered list dogfood sample ([63a60b8](https://github.com/lunguini/flatte/commit/63a60b8afd6f14a7881c3952c682098bd6b601b8))
* **cmd:** add styled workspace capstone sample ([5834313](https://github.com/lunguini/flatte/commit/583431341a064b265d0d8c969b89246e212aa34c))
* dogfood stream and inline capabilities ([b0fd16e](https://github.com/lunguini/flatte/commit/b0fd16e425439117e9f6c7ce483786b70ff6cf50))
* extract flatte.Scope + flatui.SplitLayout from dogfood ([68f2495](https://github.com/lunguini/flatte/commit/68f2495099f50d7f5d22b2c34a9c05755cac22c0))
* **flat-docker:** async Stats polling + Logs streaming (sidestep vs force) ([30d9f7c](https://github.com/lunguini/flatte/commit/30d9f7cbba23f9afa603e61cff5f819cda77a342))
* **flat-docker:** borderless panes with 1-cell padding + bg-filled headers ([1ee9089](https://github.com/lunguini/flatte/commit/1ee9089f16420729c25504e268290aa77987d4ab))
* **flat-docker:** command bar — ':' prompt with history, routes output to activity ([4ce66af](https://github.com/lunguini/flatte/commit/4ce66af407e47f8c87c768e4655c5b9d00083376))
* **flat-docker:** composeHeader — title left, tabs right; no gap ([004720a](https://github.com/lunguini/flatte/commit/004720ae8094de403fc6ef54c2c716f55bbaa4de))
* **flat-docker:** confirm modal for stop/remove ([e4b439d](https://github.com/lunguini/flatte/commit/e4b439d5f15040b3423d0df092b79a51142a8026))
* **flat-docker:** containers list + filter + static detail + focus routing ([322215d](https://github.com/lunguini/flatte/commit/322215d45ecf27ddc271f720064f35ee65a7aeb5))
* **flat-docker:** detail tabs (Stats/Logs/Inspect) — green-field case ([c46b8bd](https://github.com/lunguini/flatte/commit/c46b8bd616753eb31ce11e73450305ec8d5aff03))
* **flat-docker:** glamour pass — anchor-right activity pane, status line, sparkline ([8b803db](https://github.com/lunguini/flatte/commit/8b803db336b519176570ea066291c7167ffb6a84))
* **flat-docker:** glyphSet config + FLAT_DOCKER_GLYPHS env var + tip ([0a40a18](https://github.com/lunguini/flatte/commit/0a40a18690aef23219d632e1def81f9eb2cd885d))
* **flat-docker:** header restyle + pane gap removed + scroll-border bug fix ([3378dc8](https://github.com/lunguini/flatte/commit/3378dc83dcf2568c7727348f900a71c8b7dc7bd8))
* **flat-docker:** Images + Volumes screens — feature-module scales linearly ([1756898](https://github.com/lunguini/flatte/commit/17568983ce9990d82bec6b10c23ce9e989b7aaa4))
* **flat-docker:** mouse via ZoneMap — click rows + tab headers ([1482022](https://github.com/lunguini/flatte/commit/14820226d0066a32c72cdf4fcc852672a5efafc6))
* **flat-docker:** mouse-drag resizable columns ([7eb05e9](https://github.com/lunguini/flatte/commit/7eb05e95dab2ada419b0a60d6b48292e3f981731))
* **flat-docker:** pane borders + palette — lazydocker-style boxes ([79e921a](https://github.com/lunguini/flatte/commit/79e921afc4aec6b240e201ccb0e6a26429ce3fb6))
* **flat-docker:** paneLayout abstraction — images/volumes get mouse+resize for free ([c1bf9dc](https://github.com/lunguini/flatte/commit/c1bf9dcb0648eaddda462d138d24833307d27871))
* **flat-docker:** Powerline tabs + restore tab clicks via manual ZoneMap ([38665c5](https://github.com/lunguini/flatte/commit/38665c59d6bfaebe277beed97ca14650c98f9095))
* **flat-docker:** scaffold architecture dogfood with feature-module shape ([2d278aa](https://github.com/lunguini/flatte/commit/2d278aa892c6afefa0715bfd3238b8548973fda1))
* **flat-docker:** scroll affordances — visual scrollbar, wheel, page keys ([88962df](https://github.com/lunguini/flatte/commit/88962df519edb1aada67e94a96418f727838de26))
* **flat-docker:** sleek integrated look — accent bg headers, flush tops, solid separator ([4e7d238](https://github.com/lunguini/flatte/commit/4e7d2383ffbf84ac5d7c22c065fc5f348cbc1f76))
* **flat-docker:** streaming text panel — wrap, follow-tail, ANSI colors ([793f09c](https://github.com/lunguini/flatte/commit/793f09c5f7c6270909224a1efbc6d9531417fc2b))
* **flat-docker:** styled dividers — bg-colored column + centered grip ([72654fa](https://github.com/lunguini/flatte/commit/72654fa7e37a0625d524928ba29f29f03206a8ec))
* **flat-docker:** tabBar component with mouse support + header gap removed ([626162a](https://github.com/lunguini/flatte/commit/626162aac655ce1f2b19eca7cbe38a6ed6000030))
* **flat-landing:** browser Snake leaderboard overlay ([21896b3](https://github.com/lunguini/flatte/commit/21896b346cbc10dca80fbabfcd57ed9d5f72b560))
* **flat-landing:** interactive tabbed showcase hosting real sample apps ([ca50973](https://github.com/lunguini/flatte/commit/ca50973bd4d3131b05088ac7daec126b8e053883))
* **flat-landing:** interactive WASM landing sample with redesigned page ([4f27aa1](https://github.com/lunguini/flatte/commit/4f27aa1373a9816c4387504b195da6d38c9b1d6a))
* **flat-landing:** point the browser at the live leaderboard Worker ([0b0e3e1](https://github.com/lunguini/flatte/commit/0b0e3e178f133efce40f9fff34cb3633db54f195))
* **flat-landing:** standard Go WASM build with gzip, keep root URL clean ([47c723c](https://github.com/lunguini/flatte/commit/47c723cff006cbc193f391fcc051585ce2d35d90))
* **flat:** add cursor style metadata ([8a6a10c](https://github.com/lunguini/flatte/commit/8a6a10cc8e27d0420dfcac102f27ae12d45a3488))
* **flat:** add terminal-delegated file selection ([979e835](https://github.com/lunguini/flatte/commit/979e835f1dc2602c7b45aab389ba5a9191c577ae))
* **flatui/layout:** Element interface — Layout returns Node, not string ([a0003b2](https://github.com/lunguini/flatte/commit/a0003b229763b7e3fc64d3daadc057bc9ee3ab04))
* **flatui/layout:** stateless solver + builder API + ContentBox/Render ([e21bffa](https://github.com/lunguini/flatte/commit/e21bffa8298e7189073bfc2ba3f579e5e92e98b2))
* **flatui:** add app-owned focus ring helper ([e835690](https://github.com/lunguini/flatte/commit/e8356904a810bc4f11949ee17c54bfa3f1d4c3a3))
* **flatui:** add app-owned paginator helper ([aca38f2](https://github.com/lunguini/flatte/commit/aca38f2017490a75348cb6e673e91a80d85f7396))
* **flatui:** add app-owned tree widget ([b6aa35a](https://github.com/lunguini/flatte/commit/b6aa35a9bd88d07792b3d1c40ae6e34aac2040d8))
* **flatui:** add grouped keymap rendering ([ad9c933](https://github.com/lunguini/flatte/commit/ad9c9337ffd20bdd2431bb0543efe67c91650ea4))
* **flatui:** add key binding help metadata ([ad532ac](https://github.com/lunguini/flatte/commit/ad532ac64ca14db54f58832021808022ac5ce9cd))
* **flatui:** add mouse hit regions ([17dc6cd](https://github.com/lunguini/flatte/commit/17dc6cd9725d20bf3823775858d0037aedb76423))
* **flatui:** add textarea soft wrap ([99297ec](https://github.com/lunguini/flatte/commit/99297ec4ea43fafce3c492660f7a31a9ccc9481f))
* **flatui:** add timer and stopwatch widgets ([8a98adf](https://github.com/lunguini/flatte/commit/8a98adf0f134bdc0c49b5bf4860d47df17fcede3))
* **flatui:** extract TabBar + ComposeHeader from flat-docker dogfood ([05c0645](https://github.com/lunguini/flatte/commit/05c0645dabf2ece76fbf326c2119ff0c29cd85c5))
* **flatui:** geometry-based hit-testing; one Rect type; retire string zones ([a819af8](https://github.com/lunguini/flatte/commit/a819af8d42156ebf71576276ec6f8fbf66a63718))
* **flatui:** Progress/Viewport/List expose Layout() Node ([5250fc2](https://github.com/lunguini/flatte/commit/5250fc29ca6a126c647608a676f4ccd6059da697))
* **flatui:** render paginator controls ([382b3b9](https://github.com/lunguini/flatte/commit/382b3b91238cda4e9976959bf279ccae18a2eed3))
* **flatui:** ZoneScanner — output-scanning mouse zones (auto-zones prototype) ([3ed506b](https://github.com/lunguini/flatte/commit/3ed506b9f09526316c86a29216b8caa12c6d1a35))
* improve portable file selection and line keys ([e0ed91d](https://github.com/lunguini/flatte/commit/e0ed91d91c9bd88a678cfbd325fa6d9c63267c18))
* **layout:** NodeBase.Chrome — custom container decoration ([2807732](https://github.com/lunguini/flatte/commit/2807732379ce574aad516891f19c00f7445985b4))
* **layout:** per-side padding on NodeBase ([25dfed7](https://github.com/lunguini/flatte/commit/25dfed781cd839967f11328b0d553d36ba0ff4c8))
* **leaderboard:** verified shared Snake leaderboard backend ([8e49c98](https://github.com/lunguini/flatte/commit/8e49c9843469b3415618d53413e02f996375747f))
* **samples:** flat-game — real-time Snake showcase ([50a91f9](https://github.com/lunguini/flatte/commit/50a91f943dd6efc0b46b6a27075cde06c83aa4b2))
* state save/reload (SaveState, LoadState, App.OnExit) ([313a9a0](https://github.com/lunguini/flatte/commit/313a9a00f64bb85f9aad5dffedfb4c1b8a331991))


### Bug Fixes

* **cmd:** improve workspace tree navigation ([e73cd8a](https://github.com/lunguini/flatte/commit/e73cd8a6c8c5010089076bc8fe3a95968d1a90aa))
* **flat-docker:** anchor status line + footer to bottom of terminal ([bda487a](https://github.com/lunguini/flatte/commit/bda487a01a5a49fe27396c9c80536178565016a9))
* **flat-docker:** header tab mouse zones (right-aligned) + titled pane borders ([21435b5](https://github.com/lunguini/flatte/commit/21435b58e2d6a904c3ce855c6cc8fb836febd7db))
* **flat-docker:** right divider (detail|activity) was un-draggable — off-by-one ([476e8a2](https://github.com/lunguini/flatte/commit/476e8a26defb41334f6ef4db60c67d726818cc4f))
* **flat-docker:** TTY-found frame-corruption bug — lipgloss Height() is a minimum ([fcfdaf6](https://github.com/lunguini/flatte/commit/fcfdaf6163173ea869100d0787bafa2848c43634))
* **flat-docker:** UX polish — arrow-key tabs, highlighted tab styling, avg-MEM status line ([eb48a93](https://github.com/lunguini/flatte/commit/eb48a93e2077271e78b7be62ad6faee7c914e710))
* **flat-game:** pin board pane to the grid; queue fast turns ([cd35395](https://github.com/lunguini/flatte/commit/cd35395a5ec035f3e96862e3f44603c436cafd6e))
* **flat-landing:** hide the pane hint under the leaderboard badge ([29ed518](https://github.com/lunguini/flatte/commit/29ed518d00d4c5894348a1347e3de04fe4b3a13f))
* **flatest:** make fakeClock.advance re-entrant; document Every/Cancel scope ([8ad6b52](https://github.com/lunguini/flatte/commit/8ad6b520057225f1f1ade7037a6f41df58ae6e9f))
* **layout:** clip content to the inner box so chrome survives; symmetric padding ([006ae06](https://github.com/lunguini/flatte/commit/006ae065c57caa57661c48b6c69bbf64212ba24c))
* **layout:** compose overlays through one clamped tree walk; degenerate Grow ([a9721ef](https://github.com/lunguini/flatte/commit/a9721ef880ddd2127d77e4f79c825c268dd3be8c))
* native scrollback must re-render without Erase (TTY bug) ([cfed445](https://github.com/lunguini/flatte/commit/cfed445eb4f0ba748fb1c8fb1f77ca1a56233f14))
* native scrollback walked the frame down the screen (TTY bug) ([f08efde](https://github.com/lunguini/flatte/commit/f08efdeb04df84c7c2ff0038ca8986cd899d027c))
* prefer native file picker dialogs ([01af7b5](https://github.com/lunguini/flatte/commit/01af7b59f2816aa21d318f56c46f2e904ef4e9dc))


### Miscellaneous Chores

* release flatte 0.1.0 ([ececae4](https://github.com/lunguini/flatte/commit/ececae4b3e196ebc5f467813505d3395b728d2a8))


### Code Refactoring

* rename module path and package from flat to flatte ([a152d6c](https://github.com/lunguini/flatte/commit/a152d6c3541b55c7f01d7929e5d81902fd0b85b4))

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
