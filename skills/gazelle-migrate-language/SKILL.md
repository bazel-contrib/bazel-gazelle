---
name: gazelle-migrate-language
description: Migrates Gazelle language extensions from v1 to v2 APIs. Use when upgrading a Gazelle plugin or language extension, migrating from github.com/bazelbuild/bazel-gazelle to github.com/bazel-contrib/bazel-gazelle/v2, renaming NewLanguage to NewV2, updating gazelle_binary to version 2, or changing extension interfaces for Gazelle v2.
---

## Background

Gazelle is a tool for generating Bazel BUILD files. It supports "language extensions" (basically plugins) written in Go.

The main update in Gazelle v2 is a change to the set of interfaces an extension may implement. The goals of the new interfaces are:

- Less tightly coupled: most interfaces are optional and have only one method.
- More extensible: arguments and results are passed through structs, so new fields may be added in the future without breaking compatibility.
- Supports cancellation: most methods accept `context.Context`, especially important when an extension runs commands or communicates on the network.
- Supports error handling: most methods now return an `error`. Prefer this over ad hoc error handling with `log.Printf` or `log.Fatalf`, which were commonly used in v1.

v1 and v2 extensions are compatible with Gazelle v1 and v2, so maintainers can upgrade when it makes sense for them without inconveniencing users of either version.

For reference, the v2 proposal is [discussion #2207](https://github.com/bazel-contrib/bazel-gazelle/discussions/2207).

## Workflow

1. **Check prerequisites.** Confirm the extension builds in Bzlmod mode. If it only works in `WORKSPACE` mode, stop. If it supports both, warn the user and proceed with caution.
2. **Update `go.mod`.** Switch the module dependency to `github.com/bazel-contrib/bazel-gazelle/v2` using `go get`.
3. **Update Go imports.** Change import paths from `github.com/bazelbuild/bazel-gazelle/...` to `github.com/bazel-contrib/bazel-gazelle/v2/...`.
4. **Rename the constructor.** Change `NewLanguage()` to `NewV2()`.
5. **Migrate methods.** Update each method to its v2 signature (see [Methods](#methods) below). Drop `language.BaseLang` embedding, remove no-op methods, and add v2 static interface assertions.
6. **Update `gazelle_binary`.** Set `version = 2` in the root `BUILD.bazel`.
7. **Update Bazel deps.** Change Gazelle `deps` from `@gazelle//...` to `@gazelle//v2/...` (run `bazel run //:gazelle` after updating Go imports).
8. **Update tests.** Migrate unit tests to v2 types; add `gazelle_generation_test` integration tests where appropriate.
9. **Verify.** Build and run tests for the extension and any `gazelle_binary` that includes it.
10. **Clean up.** Run Gazelle (`bazel run //:gazelle`) to clean up any `BUILD.bazel` files that need it, especially after deleting files or imports.

## Changes

To upgrade an extension from v1 to v2, make the following changes.

### Dependencies

No change is needed in `MODULE.bazel`. The `gazelle` Bazel module contains both v1 and v2 packages.

`WORKSPACE` is not supported by Gazelle v2. It may still work, but support will be removed soon. If an extension can only be built in `WORKSPACE` mode, STOP. If an extension can build in both module mode and `WORKSPACE` mode, WARN the user and proceed with caution.

In `go.mod`:

- Old: `require github.com/bazelbuild/bazel-gazelle v0.X.Y`
- New: `require github.com/bazel-contrib/bazel-gazelle/v2 v2.X.Y`

Use `go get` to edit `go.mod`; don't edit it directly.

In the root `BUILD.bazel`, update `gazelle_binary` to build with v2.

```bzl
load("@gazelle//:def.bzl", "gazelle_binary")

gazelle_binary(
    name = "gazelle_binary",
    languages = [...],
    version = 2,
)
```

In all `BUILD.bazel` files, update `deps` on Gazelle packages from `@gazelle//...` to `@gazelle//v2/...`. Use `bazel run //:gazelle` to do this after updating imports in `.go` files.

<!-- TODO(#2272): say the minimum BCR version to use for v2 when it's ready. -->

### Imports

- Old: `github.com/bazelbuild/bazel-gazelle/...`
- New: `github.com/bazel-contrib/bazel-gazelle/v2/...`

Nearly all v1 packages have v2 equivalents. Most v1 definitions like `label.Label` are wrappers or aliases for their v2 equivalents, so they can be used interchangeably. Ideally, a v2 extension only imports v2 packages and does not depend directly on the v1 Go module.

### Constructor

- Old: `func NewLanguage() language.Language`
- New: `func NewV2() language.Language`

The new `language.Language` here is the v2 interface, which only has a `Name() string` method.

### Methods

#### General advice

Most methods in v2 accept a `ctx context.Context` argument to support cancellation. Replace any internal uses of `context.Background()` or `context.TODO()` with this argument.

Most methods in v2 have `error` return types. Return errors instead of handling them with `log.Printf` or `log.Fatalf`.

Most methods in v2 have arguments packed in `*Args` struct types and results packed in `*Result` struct types for forward compatibility. The same values are available, though often with more descriptive names. To minimize the size of a migration diff, it's often helpful to unpack `*Args` fields into local variables with the same names as the old parameters.

Most methods in v2 are optional: most interfaces have only one method, and all interfaces beyond `language.Language` are optional. If an existing method does nothing and was only implemented to satisfy an interface, remove it.

Add static assertions to verify each interface is implemented with the correct method types for v2. Remove static assertions for v1 interfaces. If a method has the wrong type, Gazelle silently ignores it. For example:

```go
var _ language.Language = (*exampleLang)(nil)
var _ config.Configurer = (*exampleLang)(nil)
var _ resolve.Indexer = (*exampleLang)(nil)
```

Drop embedding of `language.BaseLang`. This was used to fill in no-op implementations of most methods but is no longer needed.

#### `language.Language`

- Unchanged: `Name() string`

#### `config.Configurer`

- Unchanged: `KnownDirectives() []string`
- Old: `Configure(c *Config, rel string, f *rule.File)`
- New: `Configure(context.Context, ConfigureArgs) error` — move arguments into `ConfigureArgs`.

#### `compat.FlagConfigurer` (deprecated)

In v1, these methods were in `config.Configurer`.

- Unchanged: `RegisterFlags(fs *flag.FlagSet, cmd string, c *Config)`
- Unchanged: `CheckFlags(fs *flag.FlagSet, c *Config) error`

Extensions are now discouraged from processing command-line flags. Directives should be used instead. If you keep these methods, they will still be called, but they may be removed in the future. Authors should deprecate their flags and encourage their users to migrate to directives.

#### `language.Generator`

- Unchanged: `Kinds() map[string]rule.KindInfo`
- Removed: `Loads() []rule.LoadInfo`
- Removed: `ApparentLoads(func(string) string) []rule.LoadInfo` from `language.ModuleAwareLanguage`

These methods were squashed into `Kinds`. Populate the `KindInfo.Load` and `Name` fields instead. Do not attempt to use module apparent names when they differ from action names: Gazelle does this automatically.

- Old: `GenerateRules(args GenerateArgs) GenerateResult`
- New: `Generate(context.Context, GenerateArgs) (GenerateResult, error)` — method was renamed.

#### `language.Fixer`

In v1, this was part of `language.Language`.

- Old: `Fix(*config.Config, *rule.File)`
- New: `Fix(context.Context, FixArgs) error`

#### `language.OnStarter`

In v1, this was part of `language.LifecycleManager`.

- Old: `Before(context.Context)`
- New: `OnStart(context.Context) error`

#### `language.OnResolver`

In v1, this was `language.FinishableLanguage`.

- Old: `DoneGeneratingRules()`
- New: `OnResolve(context.Context) error`

#### `language.OnFinisher`

In v1, this was part of `language.LifecycleManager`.

- Old: `AfterResolvingDeps(context.Context)`
- New: `OnFinish(context.Context) error`

#### `resolve.Indexer`

- Old: `Imports(*config.Config, *rule.Rule, *rule.File) []ImportSpec`
- Old: `Embeds(*rule.File, label.Label) []label.Label`
- New: `Imports(context.Context, ImportsArgs) (ImportsResult, error)` — `Embeds` was squashed into `Imports`; `ImportsResult` has an `Embeds` field.

#### `resolve.Finder`

- Old: `CrossResolve(*config.Config, *resolve.RuleIndex, resolve.ImportSpec, string) []FindResult`
- New: `Find(context.Context, FindArgs) ([]FindResult, error)`

#### `resolve.Resolver`

- Old: `Resolve(*config.Config, *resolve.RuleIndex, *repo.RemoteCache, *rule.Rule, any, label.Label)`
- New: `Resolve(context.Context, ResolveArgs) error`

## Testing

Update any unit tests to use the v2 types rather than v1.

Where necessary, you may use the `github.com/bazel-contrib/bazel-gazelle/v2/compat` package to adapt a v1 extension to v2 (`LanguageV2`) or to fill in default no-op implementations for unimplemented interfaces (`LanguageWithDefaults`). Exercise caution: the `compat` package is unstable, and its interface may change.

Prefer using `gazelle_generation_test` from `@gazelle//:def.bzl` for any new integration tests.
