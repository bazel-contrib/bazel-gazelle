# Gazelle build file generator

Gazelle generates and updates Bazel `BUILD` files. It can create new `BUILD` files for a project that follows language conventions, and it can update existing `BUILD` files to include new sources, dependencies, and options. Gazelle supports Go and protobuf natively, and many more languages and rule sets through extensions.

Gazelle may be run by Bazel using the [`gazelle` rule](#bazel-rule) or it may be installed and run as a command line tool. Gazelle can also generate `BUILD` files for external repositories as part of the [`go_repository`](reference.md#go_repository) rule.

Slack: [#gazelle on Bazel Slack](https://bazelbuild.slack.com/archives/C01HMGN77Q8), [#go on Bazel Slack](https://bazelbuild.slack.com/archives/CDBP88Z0D), [#bazel on Go Slack](https://gophers.slack.com/archives/C1SCQE54N)

## Documentation

**References:**

- [Configuration and command line reference](gazelle-reference.md)
- [Go reference](language/go/reference.md)
- [Proto reference](language/proto/reference.md)
- [Rule reference](reference.md) (for `gazelle` and `gazelle_binary` rules)

**Explanations and tutorials:**

* [Gazelle v2](v2.md)
* [How Gazelle Works](how-gazelle-works.md)
* [`go_repository`](reference.md#go_repository)
* [Extending Gazelle](extend.md)
* [Avoiding conflicts with proto rules](https://github.com/bazel-contrib/rules_go/blob/master/proto/core.rst#avoiding-conflicts)

## Supported languages

Gazelle can generate Bazel `BUILD` files for many languages:

* **Go:** Go supported is included here in bazel-gazelle, see below.
* **Haskell:**  Tweag's [rules_haskell](https://github.com/tweag/rules_haskell) has two extensions: [gazelle_cabal](https://github.com/tweag/gazelle_cabal), for generating rules from Cabal files, and [gazelle_haskell_modules](https://github.com/tweag/gazelle_haskell_modules) for even more fine-grained build definitions.
* **Java:** bazel-contrib's [rules_jvm](https://github.com/bazel-contrib/rules_jvm) extensions include [a gazelle extension](https://github.com/bazel-contrib/rules_jvm/tree/main/java/gazelle) for generating `java_library`, `java_binary`, `java_test`, and `java_test_suite` rules.
* **JavaScript / TypeScript:** Aspect provides [JavaScript and TypeScript Support](https://github.com/aspect-build/aspect-gazelle/tree/main/language/js). BenchSci's [rules_nodejs_gazelle](https://github.com/benchsci/rules_nodejs_gazelle) supports generating `ts_project`, `js_library`, `jest_test`, and `web_asset` rules, and is able to support module bundlers like Webpack and Next.js.
* **Kotlin:** Aspect Build provides some [Kotlin Support](https://github.com/aspect-build/aspect-gazelle/tree/main/language/kotlin). Still under development, please check the README for currently available features.
* **Protocol Buffers:** Support for the `proto_library` rule. Language-specific rules like `go_proto_library` are supported by other language extensions.
* **Python:** [rules_python](https://github.com/bazel-contrib/rules_python) has an extension for generating `py_library`, `py_binary`, and `py_test` rules.
* **R:** [rules_r](https://github.com/grailbio/rules_r) has an extension for generating rules for R package builds and tests.
* **Rust:** [gazelle_rust](https://github.com/Calsign/gazelle_rust) is an extension for generating [rules_rust](https://github.com/bazelbuild/rules_rust) targets.
* **Starlark:** [bazel-skylib](https://github.com/bazelbuild/bazel-skylib) has an extension for generating `bzl_library` rules. See [bazel_skylib/gazelle/bzl](https://github.com/bazelbuild/bazel-skylib/tree/main/gazelle/bzl).
* **Swift:** [swift_gazelle_plugin](https://github.com/cgrindel/swift_gazelle_plugin) has an extension for generating `swift_library`, `swift_binary`, and   `swift_test` rules. It also includes facilities for resolving, downloading and building external Swift packages for a Bazel workspace.
* **C/C++:** [gazelle_cc](https://github.com/EngFlow/gazelle_cc) has an extension for `cc_*` rules.

If you know of an extension which could be linked here, please [open a PR](https://github.com/bazel-contrib/bazel-gazelle/edit/master/README.md)!

More languages can be added by [Extending Gazelle](extend.md). Chat with us in the `#gazelle` channel on [Bazel Slack](https://slack.bazel.build) if you'd like to discuss your design.

If you've written your own extension, please consider open-sourcing it for use by the rest of the community. Note that such extensions belong in a language-specific repository, not in bazel-gazelle. See discussion in [#1030](https://github.com/bazelbuild/bazel-gazelle/issues/1030).

## Setup

### Quick start for Gazelle only

Replace versions with the latest versions available on the [BCR](https://registry.bazel.build/modules/gazelle).

```bzl
# MODULE.bazel
bazel_dep(name = "gazelle", version = "0.54.0")
```

```bzl
# Root BUILD file
load("@gazelle//:def.bzl", "gazelle", "gazelle_binary")

gazelle(
    name = "gazelle",
    gazelle = ":gazelle_binary",
)

gazelle_binary(
    name = "gazelle_binary",
    # Populate this list with the extensions you want to use.
    languages = [
        "@bazel_skylib//gazelle/bzl",
        "@gazelle_cc//language/cc",
    ],
)
```

### Quick start for Go

```bzl
# MODULE.bazel
bazel_dep(name = "rules_go", version = "0.64.2")
bazel_dep(name = "gazelle", version = "0.54.0")

go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
go_sdk.download(version = "1.27.2")

go_deps = use_extension("@gazelle//:extensions.bzl", "go_deps")
go_deps.from_file(go_mod = "//:go.mod")

# Run 'bazel mod tidy' to populate the list below
use_repo(
    go_deps,
    "org_golang_x_net",
    "org_golang_x_tools",
)
```

```bzl
# Root BUILD file
load("@gazelle//:def.bzl", "gazelle")

# Without a custom gazelle_binary, the proto and go extensions are used.
gazelle(name = "gazelle")

# Configure Gazelle with directive comments like the one below.

# gazelle:prefix example.com/my/module/path
```

See the [Go Bzlmod docs](https://github.com/bazel-contrib/rules_go/blob/master/docs/go/core/bzlmod.md).

The full documentation for the `go_deps` extension is in [extensions.md](extensions.md#go_deps).

### WORKSPACE

See [`WORKSPACE` setup](workspace.md).

## Usage

### Command line

In most cases, you'll invoke Gazelle through Bazel using the `gazelle` rule:

```
bazel run //:gazelle
```

To run Gazelle in specific directories, or with additional flags:

```
bazel run //:gazelle -- [flags...] [directories...]
```

If you build and install a Gazelle binary, you can also invoke it directly without `bazel run`.

```
gazelle [fix|update] [flags...] [directories...]
```

To print changes Gazelle would make and exit non-zero if changes are needed:

```
bazel run //:gazelle -- -mode=diff
```

Or alternatively, you can define a `gazelle_test` to be used with `bazel test`. Note that this rule runs locally and cannot be cached.

```
load("@gazelle//:def.bzl", "gazelle_test")

gazelle_test(
    name = "gazelle_test",
    workspace = "//:BUILD.bazel", # a file in the workspace root, where the gazelle will be run
)
```

### Configuration directives

Gazelle can be configured with *directives*, which are written as top-level comments in build files. Most options that can be set on the command line can also be set using directives. Some options can only be set with directives.

Directive comments have the form `# gazelle:key value`. For example:

```bzl
load("@io_bazel_rules_go//go:def.bzl", "go_library")

# gazelle:prefix github.com/example/project
# gazelle:build_file_name BUILD,BUILD.bazel

go_library(
    name = "go_default_library",
    srcs = ["example.go"],
    importpath = "github.com/example/project",
    visibility = ["//visibility:public"],
)
```

Directives apply in the directory where they are set *and* in subdirectories. This means, for example, if you set `# gazelle:prefix` in the build file in your project's root directory, it affects your whole project. If you set it in a subdirectory, it only affects rules in that subtree.

### Lazy indexing

Gazelle parses source code and resolves import strings like `github.com/bazel-contrib/bazel-gazelle/v2/rule` to Bazel labels like `//v2/rule`. Gazelle does this by building an in-memory index of library targets that could be imported, including both generated and existing targets.

The index is populated from directories that Gazelle updates and their parent directories, so you may see different results depending on whether you run Gazelle in specific directories or across the full repo. To force Gazelle to index all directories use the `-index=all` flag. This may take a long time for large repos.

Each language extension handles dependency resolution differently, following language-specific conventions. Many extensions allow you to configure additional locations where Gazelle can search for libraries.

For Go, add `go_search` directives like this:

```bzl
# gazelle:go_search third_party/go
# gazelle:go_search replace/b example.com/b
```

These directives point to directories that contain Go code outside the current module, with an optional package prefix. `go_search` directives are not necessary if you're following regular Go module conventions or are using a Go `vendor` directory.

To configure lazy indexing with protobuf, add `proto_search` directives like this:

```bzl
# gazelle:proto_search third_party/proto api
```

The two arguments are a prefix to remove from the import path and a prefix to add. These correspond to the [`strip_import_prefix`](https://docs.bazel.build/versions/master/be/protocol-buffer.html#proto_library.strip_import_prefix) and [`import_prefix`](https://docs.bazel.build/versions/master/be/protocol-buffer.html#proto_library.import_prefix) attributes of [`proto_library`](https://bazel.build/reference/be/protocol-buffer#proto_library). They tell Gazelle how to transform an import path read from a .proto source file into a repo-root-relative path to a directory that may contain the imported file.

## Compatibility with Go

Gazelle is compatible with supported releases of Go, per the [Go Release Policy](https://golang.org/doc/devel/release.html#policy). The Go Team officially supports the current and previous minor releases. Older releases are not supported and don't receive bug fixes or security updates.

Gazelle may use language and library features from the oldest supported release.
