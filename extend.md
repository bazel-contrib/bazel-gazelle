# Extending Gazelle

Gazelle can be extended to generate and update rules for any language. This page explains how to create a new extension.

## Write an extension library

Each extension is written as a Go package. Extensions written in other languages aren't supported yet, though there is a proposal (#2471) to support extensions written in other languages.

To create a new extension, start with a minimal .go file:

```go
// Package intercal is a Go extension for the intercal language
package intercal

import "github.com/bazel-contrib/bazel-gazelle/v2/language"

func NewV2() language.Language {
	return new(lang)
}

type lang struct{}

func (lang) Name() string { return "intercal" }
```

Three definitions here are required:

1. A **type** for your extension. It can have any underlying type and doesn't need to be exported.
2. **`Name() string` method**. It returns your extension's name, usually as a lowercase single word.
3. **`NewV2() language.Language`** constructor. It returns an instance of your extension.

## Building with `gazelle_binary`

You can build a custom Gazelle binary that includes your extension.

```starlark
# Root BUILD file
load("@gazelle//:def.bzl", "gazelle", "gazelle_binary")

gazelle(
    name = "gazelle",
    gazelle = ":gazelle_binary",
)

gazelle_binary(
    name = "gazelle_binary",
    languages = [
        "//your/extension",
    ],
    version = 2,
)
```

The `languages` attribute here is a list of extension libraries to use. The order matters: Gazelle invokes extension methods in this order, so at minimum it determines the order in which rules are added to `BUILD` files. `languages` may contain a mix of extensions satisfying the v1 and v2 interfaces; `gazelle_binary` figures out which interface to use automatically.

## Implement methods

An extension can implement the following interfaces. Everything other than `language.Language` is optional.

Most extensions should implement `language.Language`, `config.Configurer`, `language.Generator`, `resolve.Indexer`, and `resolve.Resolver`.

- [`language.Language`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Language): Basic interface for extensions.
- [`config.Configurer`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/config.Configurer): modify configuration based on `# gazelle:...` directives.
- [`language.Generator`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Generator): Generate and update rules. Delete empty rules. Say how generated and existing rules should be merged.
- [`language.Fixer`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Fixer): Make arbitrary edits to `BUILD` files to fix deprecated usage.
- [`resolve.Indexer`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Indexer): Add library rules to Gazelle's in-memory index.
- [`resolve.Finder`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Finder): Look up library rules for import strings across languages outside the index.
- [`resolve.Resolver`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Resolver): Resolve import strings to Bazel labels and set `deps` attributes on generated rules.
- [`language.OnStarter`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#OnStarter): Called when Gazelle starts.
- [`language.OnResolver`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#OnResolver): Called after rule generation, before dependency resolution.
- [`language.OnFinisher`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#OnFinisher): Called before Gazelle stops.

## Tests

Use [gazelle_generation_test](reference.md#gazelle_generation_test) to write end-to-end tests. Each test case creates an example repository, runs a custom Gazelle binary there, then checks that generated `BUILD` files match expected `BUILD.out` files. You can also check for expected failures and error messages.

## Lazy indexing

Lazy indexing lets Gazelle run quickly without needing to read all build files
in a repo while still supporting index-based dependency resolution. This allows
Gazelle to update build files for specific directories in milliseconds rather
than seconds or minutes.

Lazy indexing requires a small amount of user configuration, pointing to
directories that may contain libraries based on import strings read from source
files. It also requires support from language extensions to interpret that
configuration. Most language extensions should implement this, though the
implementation will be a bit different for each language.

As an extended example, suppose a Go user has copied their module dependency
`example.com/b` into the directory `replace/b`. From some other directory `a`,
they import the package `example.com/b/c`. They then run
`gazelle update -r=false -index=lazy a` to generate a build file for `a` with
lazy indexing enabled.

In this configuration, Gazelle doesn't automatically index `replace/b`, so
the user must add a directive to their top-level build file:

```starlark
# gazelle:go_search replace/b example.com/b
```

This directive is interpreted by the Go extension. The user is telling Gazelle
to index the directory `replace/b/<suffix>` when it sees a Go package imported
with the string `example.com/b/<suffix>`. So in our example, when Gazelle sees
`example.com/b/c`, it indexes `replace/b/c` and makes all library targets
available for dependency resolution, including non-Go libraries that happen to
be there.

To support this, the Go extension needs to:

1. Support the `go_search` directive in the `KnownDirectives` and `Configure`
methods of the
[`Configurer`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/config#Configurer)
implementation. This directive may be repeated and applies in subdirectories.
1. Convert an import string like `example.com/b/c` into a list of directory
paths like `replace/b/c`.
1. Return the directory paths through
[`GenerateResult.RelsToIndex`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#GenerateResult)
in the
[`Language.GenerateRules`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language)
method.

Other extensions should follow a similar approach, though there are likely to
be differences in how `*_search` directives and import strings are interpreted.
For example, the protobuf and C++ extensions have `proto_search` and `cc_search`
directives which are similar to each other but not to Go: both languages
import libraries by file name and have similar conventions.

## Interacting with protos

The proto extension ([//language/proto:go_default_library]) gathers metadata
from .proto files and generates `proto_library` rules based on that metadata.
Extensions that generate language-specific proto rules (e.g.,
`go_proto_library`) may use this metadata.

For API reference, see the [proto godoc].

To get proto configuration information, call [proto.GetProtoConfig]. This is
mainly useful for discovering the current proto mode.

To get information about `proto_library` rules, examine the `OtherGen`
list of rules passed to `language.GenerateRules`. This is a list of rules
generated by other language extensions, and it will include `proto_library`
rules in each directory, if there were any. For each of these rules, you can
call `r.PrivateAttr(proto.PackageKey)` to get a [proto.Package] record. This
includes the proto package name, as well as source names, imports, and options.

[Language]: https://godoc.org/github.com/bazelbuild/bazel-gazelle/language#Language
[//internal/gazellebinarytest:go_default_library]: https://github.com/bazelbuild/bazel-gazelle/tree/master/internal/gazellebinarytest
[//language/go:go_default_library]: https://github.com/bazelbuild/bazel-gazelle/tree/master/language/go
[//language/proto:go_default_library]: https://github.com/bazelbuild/bazel-gazelle/tree/master/language/proto
[gazelle]: https://github.com/bazelbuild/bazel-gazelle#bazel-rule
[go_binary]: https://github.com/bazel-contrib/rules_go/blob/master/go/core.rst#go-binary
[go_library]: https://github.com/bazel-contrib/rules_go/blob/master/go/core.rst#go-library
[proto godoc]: https://godoc.org/github.com/bazelbuild/bazel-gazelle/language/proto
[proto.GetProtoConfig]: https://godoc.org/github.com/bazelbuild/bazel-gazelle/language/proto#GetProtoConfig
[proto.Package]: https://godoc.org/github.com/bazelbuild/bazel-gazelle/language/proto#Package
