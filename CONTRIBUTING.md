# Contributing to Flatte

Thanks for contributing. Flatte aims to be production-grade software teams
can depend on, and the workflow below is what keeps that bar.

## Commit messages: Conventional Commits

Every commit message follows
[Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): subject
```

- Use a scope when a useful one exists (`fix(layout): …`,
  `feat(flatui): …`, `refactor(flat-docker): …`); otherwise `type: subject`.
- Standard types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`,
  `build`, `ci`, `chore`.
- Breaking changes: mark with `!` after the type/scope **and/or** a
  `BREAKING CHANGE:` footer describing exactly what broke.

Releases and `CHANGELOG.md` are generated from these messages by
release-please — a `feat` commit produces a minor bump, a `fix` a patch,
and a breaking change a major bump (pre-1.0: minor). Write the subject as
the changelog line a user should read.

## Repository layout

Two Go modules:

- Root (`github.com/lunguini/flatte`): the library — `flatte` runtime,
  `flatui` widgets, `flatui/layout` engine, `flatest` test harness.
- `cmd/` (`github.com/lunguini/flatte/cmd`): sample and comparison apps,
  kept out of the library's dependency graph.

The `cmd/bubble-modal`, `cmd/bubble-v2-modal`, and `cmd/bubble-v2-search`
apps are Bubble Tea comparison benchmarks: keep them compiling, but do not
"improve" them beyond parity with their Flatte counterparts.

## Quality bar (every change)

```bash
go test ./...                  # library, includes golden snapshots
go test -race ./...
go vet ./...
GOOS=windows go vet ./...      # windows must keep compiling
gofmt -l .                     # must print nothing

cd cmd                         # samples are a separate module
go test ./...
go vet ./... && GOOS=windows go vet ./...
```

- Tests first (red → green) for behavior changes; every code path tested.
- The public API is locked by a snapshot test
  (`api_snapshot_test.go` / `testdata/public-api.golden` — types, methods,
  functions, and struct fields). Intentional API changes regenerate it with
  `FLAT_REGEN_API_SNAPSHOT=1 go test . -run TestPublicAPISnapshot` and the
  diff is part of the review.
- Terminal-conditional behavior additionally needs a pass in a real
  terminal (`cd cmd && go run ./flat-<sample>`) — goldens are ANSI-stripped
  and cannot see everything.

## Golden snapshots

Goldens have **no auto-update flag** by design. When a view change is
intentional, hand-edit the golden under the sample's `testdata/` (or
rewrite it from the test's "actual" output after eyeballing it) and say in
the PR why the pixels moved. Keep views deterministic: no wall-clock time
and no unseeded randomness in any `View`.

## Design principles (the short version)

Read `AGENTS.md` before larger changes. The load-bearing rules:

- Single source of truth; `View` is a pure function of state.
- No policy in core: the framework never decides what a key means.
- Apps never define message types; async results are named `StateUpdate`s.
- All mutation on the loop goroutine.
- Abstraction is found, not designed: extract a helper only after the
  pattern repeats in real samples.
