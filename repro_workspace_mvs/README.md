# Repro: gazelle go_deps underresolves with go.mod graph pruning

In a multi-module workspace where one module's tidied `go.mod` has a
transitive requirement *pruned* (Go 1.21+ graph pruning omits indirect
lines that are implied by a listed require's own go.mod), gazelle's
`go_deps` extension resolves a lower version than Go's own MVS.

## Setup

- `mod_a` requires `golang.org/x/text@v0.35.0`. That module requires
  `golang.org/x/mod@v0.33.0`. After `GOWORK=off go mod tidy`,
  `mod_a/go.mod` records `x/text` but **prunes** `x/mod` (it is implied
  by `x/text@v0.35.0`'s own go.mod).
- `mod_b` requires `golang.org/x/mod@v0.31.0` directly.
- `go.work` uses both.

## Result

```
$ go list -m golang.org/x/mod
golang.org/x/mod v0.33.0

$ bazel mod show_repo @org_golang_x_mod | grep version
  version = "v0.31.0",
```

Go follows `x/text@v0.35.0`'s go.mod to discover the `x/mod@v0.33.0`
requirement. Gazelle reads only the workspace go.mod files' `require`
lines, sees `v0.31.0` from mod_b and nothing from mod_a (pruned), and
resolves `v0.31.0`.

## Why it matters

The Bazel-built binary links a different (older) `x/mod` than the
`go build`-built one. Both `mod_a/go.mod` and `mod_b/go.mod` are tidy.

## Workaround

`go work sync` writes the unpruned indirect set into each module's
go.mod, after which gazelle sees the same versions. But that churns
every workspace go.mod on every dep change.
