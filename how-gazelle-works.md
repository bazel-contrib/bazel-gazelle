# How Gazelle Works

This page explains how Gazelle generates and updates `BUILD` files. It's intended to help Gazelle developers, extension authors, and users wishing to understand how (and why) Gazelle makes its changes.

See [Configuration and command line reference](gazelle-reference.md) for details on specific directives and flags.

## Terminology

Within Gazelle, a *rule* is a declaration in a `BUILD` file for something you can build with Bazel. A rule is an instance of a *rule kind*. The example below shows a rule named `lib` with the kind `go_binary`.

```bzl
go_binary(
    name = "lib",
    srcs = ["lib.go"],
)
```

Gazelle matches internal terminology within Bazel's source code, but it unfortunately doesn't match the terms used outside of Bazel. Bazel documentation calls this example a *target* named `lib`, which is an instance of the *rule* `go_binary`.

We regret this difference in terminology, but fixing it would require significant breaking changes to Gazelle's extension API, so we continue to make the distinction.

## Overview

Gazelle updates `BUILD` files in the following stages. Each is described in detail below.

1. [Load](#load): Parse `BUILD` files and load directory metadata.
1. [Generate](#generate): Generate new rules and build an index of existing library rules.
1. [Resolve](#resolve): Map imports in source files to Bazel labels in `deps` attributes.
1. [Write](#write): Format and save modified `BUILD` files.

All of Gazelle's language-specific functionality is implemented in plugins called [*extensions*](#extensions).

### Load

When Gazelle starts, it begins traversing the directory tree. This process runs in parallel with later stages as an optimization.

In each directory it visits, Gazelle parses the `BUILD` or `BUILD.bazel` file if present and makes a list of files and subdirectories, excluding those matched by `# gazelle:exclude` directives or `.bazelignore` files. This metadata is cached in memory so that later stages may access it quickly without requiring additional I/O.

Gazelle may or may not visit a directory based on directives and command line flags.

- Gazelle always visits directories named with positional arguments on the command line. If no arguments are specified, Gazelle visits the repository root directory.
- If recursion is enabled (with `-r=true`, enabled by default), Gazelle recursively visits subdirectories.
- If eager indexing is enabled (with `-index=all`, enabled by default), Gazelle visits *all* directories.
- If lazy indexing is enabled (with `-index=lazy`), Gazelle visits directories requested by language extensions in [`GenerateResult.RelsToIndex`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#GenerateResult.RelsToIndex). These directories are loaded lazily during the *Generate* stage.
- If indexing is disabled (with `-index=none`), Gazelle does not visit additional directories.
- Gazelle visits parent directories within the repository in addition to other directories it visits. This is necessary to apply `# gazelle:exclude` directives, which may tell Gazelle to act as if a subdirectory does not exist.

The Load stage is implemented in [`walk.walker.populateCache`](https://github.com/bazel-contrib/bazel-gazelle/blob/028c500e9f911a73683b6ec390f3e59e8f31fccc/walk/dirinfo.go#L104), which is called from [`walk.Walk2`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/walk#Walk2). No extension methods are called during this stage.

### Generate

As Gazelle visits each directory, it calls extension methods to apply configuration, fix deprecated usage, generate rules, combine generated rules with existing rules, and index libraries. Most of Gazelle's work happens during this stage.

Not all extension methods are called in each directory Gazelle visits. Methods that generate or modify rules (`Fix`, `GenerateRules`, and `MergeFile`) are only called in directories where Gazelle was asked to update the `BUILD` file (directories named with positional command line arguments and their subdirectories if the `-r` flag is enabled).

1. [`Configure`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/config#Configurer.Configure) is called in each directory Gazelle visits. Each extension can read directives from the `BUILD` file (if there is one) to decide what to do. Most directives apply to the directory they appear in and to subdirectories.
    - `Configure` is called in each directory in pre-order. All other methods in this stage are called in post-order. Sibling directories are visited sequentially (not concurrently) in lexicographic order.
    - `Configure` is called in each directory Gazelle visits, even if Gazelle won't update the `BUILD` file.
1. [`Fix`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.Fix) is called in each directory that has an existing `BUILD` file. The purpose of this method is to fix deprecated rule usage, so extensions can make any necessary transformations here.
1. [`GenerateRules`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.GenerateRules) is called in each directory. This method returns rules that should be present in the `BUILD` file, and rules that should be removed. Unlike `Fix`, this method must not actually modify the rules parsed from the `BUILD` file.
1. [`merger.MergeFile`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/merger#MergeFile) is called to combine the generated rules with existing rules.
    - `MergeFile` attempts to match each generated rule with an existing rule. If a rule is not matched, it's added to the end of the `BUILD` file. Usually the matching is based on the rule kind (a `go_library` named `client`), but it can be influenced by other heuristics. 
    - Each attribute is merged separately. An attribute can be *mergeable* or not. If an attribute is mergeable, it's expected to be managed by Gazelle, so the merger can overwrite existing values (except for values marked with `# keep`). If an attribute is not mergeable, Gazelle may set an initial value, but won't overwrite it later.
    - `MergeFile` also merges rules from the empty list returned by `GenerateRules`'. This can delete existing rules if they're not marked with `# keep`. For example, this allows Gazelle to delete a `go_test` rule after all the `_test.go` files were removed.
    - Extensions don't directly participate in the merging process, though they can influence matching and merging by returning [`rule.KindInfo`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/rule#KindInfo) for each generated rule kind from the [`Kinds`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.Kinds) method.
1. [`Imports`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.47.0/resolve#Resolver.Imports) is called on each rule after the merge to build an in-memory index for dependency resolution. This table maps import strings and language names to Bazel labels. `Imports` is not called if indexing is disabled with `-index=none`.

The Generate stage is implemented in the [callback function](https://github.com/bazel-contrib/bazel-gazelle/blob/028c500e9f911a73683b6ec390f3e59e8f31fccc/cmd/gazelle/fix-update.go#L322) passed to [`walk.Walk2`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/walk#Walk2) as part of the [`runFixUpdate`](https://github.com/bazel-contrib/bazel-gazelle/blob/028c500e9f911a73683b6ec390f3e59e8f31fccc/cmd/gazelle/fix-update.go#L263) function.

### Resolve

Gazelle resolves dependencies as a separate stage so it can use the index built by calling `Imports` during the Generate stage.

1. [`Resolve`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/resolve#Resolver.Resolve) is called on each generated rule.
    - The extension that generated the rule should resolve dependencies, often using the index, then set `deps` and any related attributes.
    - To avoid redundant I/O, an extension may return information about import strings found in source files through [`GenerateResult.Imports`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#GenerateResult.Imports) when returning from [`GenerateRules`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.GenerateRules). This value is opaque to Gazelle. It's passed back to `Resolve`.
1. [`merger.MergeFile`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/merger#MergeFile) is called again to merge changes with existing rules in `BUILD` files.

### Write

At this point, Gazelle has made all necessary changes to `BUILD` files in memory. It then formats these files with [`build.Format`](https://pkg.go.dev/github.com/bazelbuild/buildtools/build#Format) and writes them back to disk or prints them, depending on the `-mode` flag (see [Flags](gazelle-reference.md#flags).

## Extensions

This section explains how Gazelle uses extensions. See [Extending Gazelle](https://github.com/bazel-contrib/bazel-gazelle/blob/master/extend.md) for a guide to writing a new extension.

Gazelle provides a language-agnostic framework for generating rules and updating `BUILD` files. All the language-specific functionality is implemented in *extensions*. An extension is an implementation of the [`language.Language`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language) interface, which requires `GenerateRules` and a few other methods. `language.Language` also embeds [`config.Configurer`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.47.0/config#Configurer) and [`resolve.Resolver`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/resolve#Resolver).

Gazelle can't dynamically load extensions at run-time: the Gazelle binary must be built with all extensions the user might need. The [`gazelle_binary`](https://github.com/bazel-contrib/bazel-gazelle/blob/master/reference.md#gazelle_binary) rule makes this easy: the user lists packages built with `go_library` that contain necessary extensions. Each package must contain a `New()` function that returns a value implementing the extension interface. `gazelle_binary` generates a source file that calls each of those `New()` functions, then compiles that together with the rest of Gazelle.

```bzl
load("@gazelle//:def.bzl", "gazelle_binary")

gazelle_binary(
    name = "gazelle_binary",
    languages = [
        "@gazelle//language/proto",
        "@gazelle//language/go",
        "@gazelle_cc//language/cc",
    ],
)
```

## Manipulating the syntax tree

Gazelle uses the [`build` package](https://pkg.go.dev/github.com/bazelbuild/buildtools/build) (from buildifier and buildozer) to parse, edit, and format `BUILD` files. This package provides a low-level interface to the syntax tree. For more convenient editing and merging, Gazelle provides its own [`merger`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/merger) and [`rule`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/rule) packages.

The `rule` package lets extensions create, update, and delete "rules" (which become call expressions in the underlying syntax tree) and read or write their attributes using simple values rather than syntax tree nodes. For example, an extension can create a new rule as follows:

```go
r := rule.NewRule("go_binary", "server")
r.SetAttr("importpath", "example.com/hello/server")
r.SetAttr("srcs", []string{"main.go", "server.go"})
```

To add a new rule to a `BUILD` file, Gazelle must add it to [`File.Rules`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/rule#File), then call [`File.Save`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/rule#File.Save), which syncs changes to the syntax tree, formats the syntax tree to bytes, then writes the file. Extensions do not call `File.Save` directly: Gazelle does this once for each file, after all extensions have run.

### Merging changes to the syntax tree

`BUILD` files often contain a mix of human-written and machine-generated rules and attributes. Updating the machine-generated parts while preserving the human-written portion is a delicate process, so extensions do not directly modify the syntax tree. 

Instead, each extension's [`GenerateRules`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.GenerateRules) method creates and returns two lists: a `Gen` list of rules the `BUILD` file should contain, and an `Empty` list of rules that should be deleted from the `BUILD` file, if they're present. This is often enough information to regenerate the `BUILD` file from scratch.

After calling `GenerateRules`, Gazelle calls [`merger.MergeFile`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/merger#MergeFile) to merge the `Gen` and `Empty` lists with the rules that are already present in the `BUILD` file. `MergeFile` is language-neutral, but the way it handles rule attributes is controlled by the map returned by each extension's [`Kinds`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.Kinds) method. 

`MergeFile` processes each rule as follows:

1. `MergeFile` attempts to match the rule with an existing rule. An existing rule matches if:
    - It has the same kind and name (a `go_binary` with `name = "server"`).
    - One of its *matchable attributes* (determined by the `Kinds` map) has the same value (a `go_library` with `importpath = "example.com/hello/server"`).
    - If the rule kind's `MatchAny` flag is set in the `Kinds` map, then any rule of that kind can match. This is useful when only one rule is expected per directory.
1. If `MergeFile` doesn't find a match, then it either adds the rule if it was from the `Gen` list or ignores the rule if it was from the `Empty` list.
1. If `MergeFile` finds a match, it calls [`rule.MergeRules`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/rule#MergeRules) to combine the rules.
    - If an attribute is present in the new rule but not the existing rule, it's added.
    - If an attribute is present in the existing rule but not the new rule, it's deleted if the attribute is *mergeable* (determined by the `Kinds` map) or preserved if not.
    - If an attribute is present in both the existing and new rules:
        - If the attribute is not mergeable, the existing attribute is preserved. This is appropriate for human-written attributes with a machine generated default.
        - If the attribute is mergeable, the values are merged. The merge process depends on the type of value (string, list, etc.). New values typically replace existing values, but ordering and comments are preseved whenever possible.
        - Extension authors can modify merging behavior with values that implement the [`rule.Merger`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/rule#Merger) interface.
1. If an existing rule is *empty* after merging with a rule from the `Empty` list, `MergeFiles` deletes it. A rule is empty if none of its *non-empty attributes* are set (determined by the `Kinds` map; typically at least `srcs` and `deps` are non-empty attributes).

### Example: file is renamed

To understand how merging works, consider this example, where the user renamed `foo.go` to `bar.go`:

```bzl
### Existing
go_library(
    name = "lib",
    srcs = [
        "foo.go",  # foo comment
        "main.go",  # main comment
    ],
    visibility = ["//:__subpackages__"],
)

### Generated
go_library(
    name = "lib",
    srcs = [
        "bar.go",
        "main.go",
    ],
    visibility = ["//visibility:public"],
)

### Merged
go_library(
    name = "lib",
    srcs = [
        "bar.go",
        "main.go",  # main comment
    ]
    visibility = ["//:__subpackages__"],
)
```

The generated rule matches the existing rule because the kind and name are the same (`go_library` with `name = "lib"`).

The `srcs` attribute is mergeable, and both `srcs` values are lists of strings, which Gazelle knows how to merge. `"main.go"` is preserved with its comment, since it's in the list from both rules. `"foo.go"` is dropped since it's not in the generated rule's list. `"bar.go"` is added. The comment on `"foo.go"` is not preserved, since Gazelle has no way to know it was the same file.

The `visibility` attribute is not mergeable, so Gazelle doesn't change it when merging. The generated rule does have this attribute, since it's important to provide a default, but if the user edits the `BUILD` file to change its value, Gazelle won't overwrite it.

### Example: rule with matchable attribute is renamed

In this example, the user has changed the name of the library from `"foo"` to `"bar"` and imported a new dependency from a source file.

```bzl
### Existing
go_library(
    name = "bar",
    srcs = ["lib.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
)

### Generated
go_library(
    name = "foo",
    srcs = ["lib.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
    deps = ["//dep"],
)

### Merged
go_library(
    name = "bar",
    srcs = ["lib.go"],
    importpath = "example.com/foo",
    visibility = ["//visibility:public"],
    deps = ["//dep"],
)
```

Even though the `name` attribute has changed, Gazelle can still match the generated rule with the existing rule because the `importpath` attribute is listed in the [`MatchAttrs`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/rule#KindInfo.MatchAttrs) list for `go_library`, and the value for that attribute is the same.

### Example: sources are deleted

Consider what happens when all of a library's source files are deleted:

```bzl
### Existing
go_library(
    name = "lib",
    srcs = [
        "a.go",
        "b.go",
    ],
    importpath = "example.com/lib",
    visibility = ["//visibility:public"],
    deps = ["//dep"],
)

### Generated (Empty list)
go_library(
    name = "lib",
    importpath = "example.com/lib",
)

### Merged: rule is deleted
```

Because there are no source files, the language extension returns a `go_library` rule in the `Empty` list instead of the `Gen` list. This is matched and merged with the existing rule, as usual. None of the non-empty attributes are set (for `go_library`, that's `srcs`, `deps`, `embed`), so the rule is deleted.

The `BUILD` file is *not* deleted, even if it is now empty. Deleting a `BUILD` file can `glob` expressions in parent directories to match additional files, which may not be safe.

### `# keep` comments

A user can prevent Gazelle from merging something by adding a `# keep` comment. The comment may be applied to a rule, an attribute, or a value.

```bzl
# keep: don't change this rule
go_library(
    name = "lib",
)

go_library(
    name = "lib",
    # keep: don't change the attribute below
    srcs = ["lib.go"],
    importpath = "example.com/foo",  # keep: or this one
)

go_library(
    name = "lib",
    srcs = [
        # keep: don't change this value
        "a.go",
        "b.go",  # keep: or this one
    ],
)
```

To understand how Gazelle sees `# keep` comments, it may help to know that the `BUILD` file parser divides comments into three lists for each syntax tree node:

- `Before` comments appear on the lines above a syntax tree node (without a blank line in between). They are attached to the top-most tree node.
- A `Suffix` comment appears at the end of the same line. It is attached to the right-most tree node.
- `After` comments appear on the lines below a syntax tree node and aren't important here.

Because a `Suffix` comment is attached to the right-most node, on the line with `importpath` above, the `# keep` comment is attached to the expression node `"example.com/foo"`, not to the attribute node. The comment still works in this case, but this causes subtle behavior for custom mergers.
