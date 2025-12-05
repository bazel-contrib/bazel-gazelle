# Gazelle build file generator

Gazelle is a build file generator for Bazel projects. It can create new BUILD.bazel files for a project that follows language conventions, and it can update existing build files to include new sources, dependencies, and options. Gazelle natively supports Go and protobuf, and it may be [extended](extend.md) to support new languages and custom rule sets.

Gazelle may be run by Bazel using the [`gazelle` rule](#bazel-rule) or it may be installed and run as a command line tool. Gazelle can also generate build files for external repositories as part of the [`go_repository`](reference.md#go_repository) rule.

*Gazelle is under active development. Its interface and the rules it generates may change. Gazelle is not an official Google product.*

Mailing list: [bazel-go-discuss](https://groups.google.com/forum/#!forum/bazel-go-discuss)

Slack: [#go on Bazel Slack](https://bazelbuild.slack.com/archives/CDBP88Z0D), [#bazel on Go Slack](https://gophers.slack.com/archives/C1SCQE54N)

*rules_go and Gazelle are getting community maintainers! If you are a regular
user of either project and are interested in helping out with development,
code reviews, and issue triage, please drop by our Slack channels (linked above)
and say hello!*

**See also:**

* [Architecture of Gazelle](Design.md)
* [`go_repository`](reference.md#go_repository)
* [Extending Gazelle](extend.md)
* [Avoiding conflicts with proto rules](https://github.com/bazelbuild/rules_go/blob/master/proto/core.rst#avoiding-conflicts)

## Supported languages

Gazelle can generate Bazel BUILD files for many languages:

* **Go:** Go supported is included here in bazel-gazelle, see below.
* **Haskell:**  Tweag's [rules_haskell](https://github.com/tweag/rules_haskell) has two extensions: [gazelle_cabal](https://github.com/tweag/gazelle_cabal), for generating rules from Cabal files, and [gazelle_haskell_modules](https://github.com/tweag/gazelle_haskell_modules) for even more fine-grained build definitions.
* **Java:** bazel-contrib's [rules_jvm](https://github.com/bazel-contrib/rules_jvm) extensions include [a gazelle extension](https://github.com/bazel-contrib/rules_jvm/tree/main/java/gazelle) for generating `java_library`, `java_binary`, `java_test`, and `java_test_suite` rules.
* **JavaScript / TypeScript:** Aspect provides [JavaScript and TypeScript Support](https://github.com/aspect-build/aspect-cli/blob/main/gazelle/js/) in aspect-cli (also usable separately). BenchSci's [rules_nodejs_gazelle](https://github.com/benchsci/rules_nodejs_gazelle) supports generating `ts_project`, `js_library`, `jest_test`, and `web_asset` rules, and is able to support module bundlers like Webpack and Next.js.
* **Kotlin:** Aspect Build provides some [Kotlin Support](https://github.com/aspect-build/aspect-cli/blob/main/gazelle/kotlin/) in the repo of their aspect-cli (also usable separately). Still under development, please check the README for currently available features.
* **Protocol Buffers:** Support for the `proto_library` rule, as well as `go_proto_library` is in this repository, see below. Other language-specific proto rules are not supported here. [stackb/rules_proto](https://github.com/stackb/rules_proto) is a good resource for these rules.
* **Python:** [rules_python](https://github.com/bazel-contrib/rules_python) has an extension for generating `py_library`, `py_binary`, and `py_test` rules.
* **R:** [rules_r](https://github.com/grailbio/rules_r) has an extension for generating rules for R package builds and tests.
* **Rust:** [gazelle_rust](https://github.com/Calsign/gazelle_rust) is an extension for generating [rules_rust](https://github.com/bazelbuild/rules_rust) targets.
* **Starlark:** [bazel-skylib](https://github.com/bazelbuild/bazel-skylib) has an extension for generating `bzl_library` rules. See [bazel_skylib/gazelle/bzl](https://github.com/bazelbuild/bazel-skylib/tree/main/gazelle/bzl).
* **Swift:** [swift_gazelle_plugin](https://github.com/cgrindel/swift_gazelle_plugin) has an extension for generating `swift_library`, `swift_binary`, and   `swift_test` rules. It also includes facilities for resolving, downloading and building external Swift packages for a Bazel workspace.
* **C/C++:** [gazelle_cc](https://github.com/EngFlow/gazelle_cc) has an extension for `cc_*` rules.

If you know of an extension which could be linked here, please [open a PR](https://github.com/bazel-contrib/bazel-gazelle/edit/master/README.rst)!

More languages can be added by [Extending Gazelle](extend.md). Chat with us in the `#gazelle` channel on [Bazel Slack](https://slack.bazel.build) if you'd like to discuss your design.

If you've written your own extension, please consider open-sourcing it for use by the rest of the community. Note that such extensions belong in a language-specific repository, not in bazel-gazelle. See discussion in [#1030](https://github.com/bazelbuild/bazel-gazelle/issues/1030).

## Setup

### Bzlmod

See the [Go Bzlmod docs](https://github.com/bazel-contrib/rules_go/blob/master/docs/go/core/bzlmod.md).

The full documentation for the `go_deps` extension is in [extensions.md](extensions.md#go_deps).

### WORKSPACE

To use Gazelle in a new project, add the `bazel_gazelle` repository and its dependencies to your WORKSPACE file and call `gazelle_dependencies`. It should look like this:

```bzl
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    integrity = "sha256-M6zErg9wUC20uJPJ/B3Xqb+ZjCPn/yxFF3QdQEmpdvg=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.48.0/rules_go-v0.48.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    integrity = "sha256-12v3pg/YsFBEQJDfooN6Tq+YKeEWVhjuNdzspcvfWNU=",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.37.0/bazel-gazelle-v0.37.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.37.0/bazel-gazelle-v0.37.0.tar.gz",
    ],
)


load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")

############################################################
# Define your own dependencies here using go_repository.
# Else, dependencies declared by rules_go/gazelle will be used.
# The first declaration of an external repository "wins".
############################################################

go_rules_dependencies()

go_register_toolchains(version = "1.20.5")

gazelle_dependencies()
```

`gazelle_dependencies` supports optional argument `go_env` (dict-mapping)
to set project specific go environment variables. If you are using a
`WORKSPACE.bazel` file, you will need to specify that using:

```bzl
gazelle_dependencies(go_repository_default_config = "//:WORKSPACE.bazel")
```

Add the code below to the BUILD or BUILD.bazel file in the root directory
of your repository.

**Important:** For Go projects, replace the string after `prefix` with
the portion of your import path that corresponds to your repository.

```bzl
load("@bazel_gazelle//:def.bzl", "gazelle")

# gazelle:prefix github.com/example/project
gazelle(name = "gazelle")
```

After adding this code, you can run Gazelle with Bazel.

```
bazel run //:gazelle
```

This will generate new BUILD.bazel files for your project. You can run the same command in the future to update existing BUILD.bazel files to include new source files or options.

You can write other `gazelle` rules to run alternate commands like `update-repos`.

```bzl
gazelle(
    name = "gazelle-update-repos",
    args = [
        "-from_file=go.mod",
        "-to_macro=deps.bzl%go_dependencies",
        "-prune",
    ],
    command = "update-repos",
)
```

You can also pass additional arguments to Gazelle after a `--` argument.

```
bazel run //:gazelle -- update-repos -from_file=go.mod -to_macro=deps.bzl%go_dependencies
```

After running `update-repos`, you might want to run `bazel run //:gazelle` again, as the `update-repos` command can affect the output of a normal run of Gazelle.

To verify that all BUILD files are update-to-date, you can use the `gazelle_test` rule.

```
load("@bazel_gazelle//:def.bzl", "gazelle_test")

gazelle_test(
    name = "gazelle_test",
    workspace = "//:BUILD.bazel", # a file in the workspace root, where the gazelle will be run
)
```

However, please note that gazelle_test cannot be cached.

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

### Reference

For a full reference on Gazelle's configuration directives, flags, and rules, see the following pages:

- [Configuration and command line reference](gazelle-reference.md)
- [Go reference](language/go/reference.md)
- [Proto reference](language/proto/reference.md)
- [Rule reference](reference.md) (for `gazelle` and `gazelle_binary` rules)

Extensions defined outside this repo provide their own references.

### Lazy indexing in `fix` and `update`

By default, `fix` and `update` read all build files in a repo to build an index of library rules (see [Dependency resolution](#dependency-resolution)) when Gazelle starts. This can take a long time on a large repo. To avoid this problem, Gazelle can lazily index specific directories, with help from extensions that support lazy indexing.

To configure lazy indexing with Go, add `go_search` directives like this:

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

To use Gazelle with lazy indexing, run with `-r=false -index=lazy`, and pass the directories to update on the command line.

```bzl
gazelle -r=false -index=lazy path/to/dir1 path/to/dir2
```

You can configure your `gazelle` Bazel target to pass these flags automatically:

```bzl
load("@gazelle//:def.bzl", "gazelle", "gazelle_binary")

gazelle(
    name = "gazelle",
    command = "fix",
    extra_args = ["-r=false", "-index=lazy"],
    gazelle = ":gazelle_binary",
)

gazelle_binary(
    name = "gazelle_binary",
    ...
)
```

## Compatibility with Go

Gazelle is compatible with supported releases of Go, per the [Go Release Policy](https://golang.org/doc/devel/release.html#policy). The Go Team officially supports the current and previous minor releases. Older releases are not supported and don't receive bug fixes or security updates.

Gazelle may use language and library features from the oldest supported release.

## Compatibility with rules_go

Gazelle generates build files that use features in newer versions of `rules_go`. Newer versions of Gazelle *may* generate build files that work with older versions of `rules_go`, but check the table below to ensure you're using a compatible version.

| Gazelle version     | Minimum rules_go version     | Maximum rules_go version |
| ------------------- | ---------------------------- | ------------------------ |
| 0.8                 | 0.8                          | n/a                      |
| 0.9                 | 0.9                          | n/a                      |
| 0.10                | 0.9                          | 0.11                     |
| 0.11                | 0.11                         | 0.24                     |
| 0.12                | 0.11                         | 0.24                     |
| 0.13                | 0.13                         | 0.24                     |
| 0.14                | 0.13                         | 0.24                     |
| 0.15                | 0.13                         | 0.24                     |
| 0.16                | 0.13                         | 0.24                     |
| 0.17                | 0.13                         | 0.24                     |
| 0.18                | 0.19                         | 0.24                     |
| 0.19                | 0.19                         | 0.24                     |
| 0.20                | 0.20                         | 0.24                     |
| 0.21                | 0.20                         | 0.24                     |
| 0.22                | 0.20                         | 0.24                     |
| 0.23                | 0.26                         | 0.28                     |
| 0.24                | 0.29                         | 0.40                     |
| 0.25                | 0.29                         | 0.40                     |
| 0.26                | 0.29                         | 0.40                     |
| 0.27                | 0.29                         | 0.40                     |
| 0.28                | 0.35                         | 0.40                     |
| 0.29                | 0.35                         | 0.40                     |
| 0.30                | 0.35                         | 0.40                     |
| 0.31                | 0.35                         | 0.40                     |
| 0.32                | 0.41                         | n/a                      |
| 0.33                | 0.41                         | n/a                      |
| 0.34                | 0.41                         | n/a                      |
| 0.35                | 0.41                         | n/a                      |
| 0.36                | 0.41                         | n/a                      |
| 0.37                | 0.41                         | n/a                      |

## Dependency resolution

One of Gazelle's most important jobs is resolving library import strings (like `import "golang.org/x/sys/unix"`) to Bazel labels (like `@org_golang_x_sys//unix:go_default_library`). Gazelle follows the rules below to resolve dependencies:

1. If the import to be resolved is part of a standard library, no explicit dependency is written. For example, in Go, you don't need to declare that you depend on `"fmt"`.
1. If a `# gazelle:resolve` directive matches the import to be resolved, the label at the end of the directive will be used.
1. If proto rule generation is enabled, special rules will be used when importing certain libraries. These rules may be disabled by adding `# gazelle:proto disable_global` to a build file (this will affect subdirectories, too) or by passing `-proto disable_global` on the command line.
    1. Imports of Well Known Types are mapped to rules in `@io_bazel_rules_go//proto/wkt`.
    1. Imports of `github.com/golang/protobuf/ptypes`, `descriptor`, and `jsonpb` are mapped to special rules in `@com_github_golang_protobuf`. See [Avoiding conflicts with proto rules](https://github.com/bazel-contrib/rules_go/blob/master/proto/core.rst#avoiding-conflicts).
1. If the import to be resolved is in the library index, the import will be resolved to that library. If `-index=all`, Gazelle builds an index of library rules in the current repository before starting dependency resolution. This can take a while, since Gazelle visits every directory in the repository. If `-index=lazy`, then language extensions may hint at specific directories to visit, which can be much faster.
    1. For Go, the match is based on the `importpath` attribute.
    1. For proto, the match is based on the `srcs` attribute.
1. If `-index=none` and a package is imported that has the current `go_prefix` as a prefix, Gazelle generates a label following a convention. For example, if the build file in `//src` set the prefix with `# gazelle:prefix example.com/repo/foo`, and you import the library `"example.com/repo/foo/bar`, the dependency will be `"//src/foo/bar:go_default_library"`.
1. Otherwise, Gazelle will use the current `external` mode to resolve the dependency.
    1. In `external` mode (the default), Gazelle will transform the import string into an external repository label. For example, `"golang.org/x/sys/unix"` would be resolved to `"@org_golang_x_sys//unix:go_default_library"`. Gazelle does not confirm whether the external repository is actually declared in WORKSPACE, but if there *is* a `go_repository` in WORKSPACE with a matching `importpath`, Gazelle will use its name. Gazelle does not index rules in external repositories, so it's possible the resolved dependency does not exist.
    1. In `static` mode, Gazelle has the same behavior as `external` mode, except that it will not call out to the network for resolution when no matching import is found within WORKSPACE. Instead, it will skip the unknown import. This is the default mode for `go_repository` rules.
    1. In `vendored` mode, Gazelle will transform the import string into a label in the vendor directory. For example, `"golang.org/x/sys/unix"` would be resolved to `"//vendor/golang.org/x/sys/unix:go_default_library"`. This mode is usually not necessary, since vendored libraries will be indexed and resolved using rule 4.
