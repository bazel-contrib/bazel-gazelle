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

Gazelle updates `BUILD` files in the following steps. Each is described in detail below.

1. [Configure](#configure): Parse `BUILD` files and load directory metadata.
1. [Generate](#generate): Generate new rules.
1. [Index](#index): Build an index of library rules (both generated and pre-existing).
1. [Resolve](#resolve): Map imports in source files to Bazel labels in `deps` attributes.
1. [Write](#write): Format and save modified `BUILD` files.

All of Gazelle's language-specific functionality is implemented in plugins called [*extensions*](#extensions).

### Configure

When Gazelle starts, it configures each directory it expects to visit for the [generate](#generate) and [index](#index) steps. It also configures parent directories, up to the repository root. Parent directories are always configured before child directories so that `# gazelle:exclude` and similar directives can be applied.

To configure a directory, Gazelle parses the `BUILD` or `BUILD.bazel` if present, then makes a list of files and subdirectories, excluding those matched by a `# gazelle:exclude` directive or `.bazelignore` file. This metadata is cached in memory so that later steps may access it quickly without requiring additional I/O.

Directives and command line flags control whether Gazelle visits a directory.

- Gazelle always visits directories named with positional arguments on the command line. If no arguments are specified, Gazelle visits the repository root directory.
- The `-r` flag controls whether Gazelle recursively visits subdirectories. By default (`-r=auto`), recursion is disabled when command line arguments are specified and enabled when there are none.
- The `-index` flag controls which directories Gazelle visits for [indexing](#index).
    - If lazy indexing is enabled (with `-index=lazy`, enabled by default), Gazelle visits directories requested by language extensions in [`GenerateResult.RelsToIndex`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#GenerateResult.RelsToIndex). These directories are configured lazily during the [generate](#generate) step.
    - If eager indexing is enabled (with `-index=all`), Gazelle configures all directories.
    - If indexing is disabled (with `-index=none`), Gazelle does not visit additional directories.

Gazelle calls the [`Configure`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/config#Configurer.Configure) method on each extension in each directory it visits. Each extension can read directives from the `BUILD` file (if there is one) to decide what to do. Most directives apply to the directory they appear in and to subdirectories.

### Generate

As Gazelle visits each directory, it calls extension methods to fix deprecated usage, generate rules, and combine generated rules with existing rules. Most of Gazelle's work happens during this step. Gazelle only calls these methods in directories where it expects to update build files (controlled by command line arguments and the `-r` flag).

The following extension methods are called in this step:

1. [`Fix`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Fixer.Fix) is called in each directory that has an existing `BUILD` file. The purpose of this method is to fix deprecated rule usage, so extensions can make any necessary transformations here.
1. [`Generate`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Generator.Generate) is called in each directory. This method returns a *generated* list of rules that should be present in the `BUILD` file, and an *empty* list of rules that should be removed. Unlike `Fix`, this method must not actually modify the rules parsed from the `BUILD` file.
1. [`Kinds`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Generator.Kinds) is called once on each extension before Gazelle visits any directories. It returns metadata about kinds of generated rules.

Gazelle uses the `Kinds` metadata to merge the generated and empty lists into the existing `BUILD` file using [`merger.MergeFile`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/merger#MergeFile). Extensions don't directly participate in the merging process, other than returning `Kinds` metadata.

1. `MergeFile` attempts to match each generated rule with an existing rule. If a rule is not matched, it's added to the end of the `BUILD` file. Usually the matching is based on the rule kind and name (a `go_library` named `client`), but matching can be influenced by other heuristics. 
1. Each attribute is merged separately. The `Kinds` metadata determines whether an attribute is *mergeable* or not. If an attribute is mergeable, it's expected to be managed by Gazelle, so the merger can overwrite existing values (except for values marked with `# keep`). If an attribute is not mergeable, Gazelle may set an initial value, but won't overwrite it later.
1. `MergeFile` merges both the generated and empty lists returned by `Generate`. If a rule in the empty list matches an existing rule, and the merged rule is empty according to the `Kinds` metadata, `MergeFile` deletes it from the `BUILD` file unless it's protected with a `# keep` comment. For example, this allows Gazelle to delete a `go_test` rule after all the `_test.go` files are removed.

### Index

Gazelle extensions need to be able to resolve import strings parsed from source files like `example.com/foo/bar` into Bazel labels like `//foo/bar` that can be written as `deps` attributes. Sometimes this can be done with a simple transformation, assuming a strong set of conventions are followed. Conventions are not followed strictly in all repos though, so most extensions make use of an index of importable libraries Gazelle builds in memory each time it runs.

To build this index, Gazelle calls an extension's [`Imports`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Indexer.Imports) method on each rule that might be relevant. For an importable rule, `Imports` returns a list of import strings and languages, which are added to the main [`RuleIndex`](https://github.com/bazel-contrib/bazel-gazelle/v2/resolve#RuleIndex). For a non-importable rule like `go_binary`, `Imports` returns nothing.

To call `Imports`, Gazelle visits directories controlled by the `-index` flag.

- If `-index=lazy` (default), `Imports` is called in:
    - Each directory where `Generate` was called.
    - Additional search directories requested by extensions through `GenerateResult.RelsToIndex`.
    - Parent directories, up to the repository root.
- If `-index=all`, `Imports` is called in all directories in the repository. This can take a long time in large repositories.
- If `-index=none`, `Imports` is not called at all.

Search directories are configured by the user with directives like `# gazelle:go_search` or `# gazelle:cc_search`. Extensions need to interpret these directives though, since each language is different.

No guarantee is made about the order in which `Imports` is called. Gazelle usually calls it in child directories before parent directories, but this can be changed by `RelsToIndex`.

Within a directory, `Imports` is called on each rule after `MergeFile`, so it may be called on both updated and existing rules. For a rule that was updated, Gazelle calls `Imports` on the extension that generated it; for an existing rule, Gazelle calls `Imports` on the first extension that returned `Kinds` metadata for that kind of rule.

### Resolve

After the index is complete, Gazelle resolves dependencies for each generated rule.

1. [`Resolve`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Resolver.Resolve) is called for each generated rule on the extension that generated it.
    - The extension implements whatever logic is necessary to set `deps` and related attributes on each generated rule. The `RuleIndex` created during the [index](#index) step is available, but is not required.
    - To avoid redundant I/O, an extension may return information about import strings found in source files through [`GenerateResult.Imports`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#GenerateResult.Imports) when returning from [`Generate`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Generator.Generate). This value is opaque to Gazelle. It's passed back to `Resolve`. Alternatively, an extension may set a private attribute on each generated rule, or it can keep its own internal cache.
1. When an extension calls [`RuleIndex.Find`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#RuleIndex.Find), and the import string is not present in the index, Gazelle calls the [`Find`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Finder.Find) method on each extension that supports it. This lets extensions support cross-language imports or features like offline indexing.
1. [`merger.MergeFile`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/merger#MergeFile) is called again to merge changes made by `Resolve` with existing rules in `BUILD` files.

### Write

At this point, Gazelle has made all necessary changes to `BUILD` files in memory. It then formats these files with [`build.Format`](https://pkg.go.dev/github.com/bazelbuild/buildtools/build#Format) and writes them back to disk or prints them, depending on the `-mode` flag (see [Flags](gazelle-reference.md#flags)).

## Extensions

This section explains how Gazelle uses extensions. See [Extending Gazelle](https://github.com/bazel-contrib/bazel-gazelle/blob/master/extend.md) for a guide to writing a new extension.

Gazelle provides a language-agnostic framework for generating rules and updating `BUILD` files. All the language-specific functionality is implemented in *extensions*. An extension implements a subset of the following interfaces:

- [`language.Language`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Language) (required): provides the `Name` method.
- [`config.Configurer`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/config#Configurer): custom configuration through directive comments in `BUILD` files.
- [`language.Fixer`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Fixer): repairs deprecated usage of rule sets
- [`language.Generator`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Generator): generates rules for each directory.
- [`resolve.Indexer`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Indexer): returns language-specific import strings for each rule.
- [`resolve.Finder`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Finder): supports cross-language dependency resolution outside of the main index.
- [`resolve.Resolver`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/resolve#Resolver): resolves dependencies, modifying generated rules after the main index is built.

Additionally, an extension may implement interfaces to receive calls between steps when Gazelle runs. This is useful for managing external resources like server connections.

- [`language.OnStarter`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#OnStarter): called when Gazelle starts.
- [`language.OnResolve`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#OnResolver): called after `Generate` and `Imports`, before `Resolve`.
- [`language.OnFinisher`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#OnFinisher): called after Gazelle writes files, before it exits.

Gazelle can't dynamically load extensions at run-time: the Gazelle binary must be built with all extensions the user might need. The [`gazelle_binary`](https://github.com/bazel-contrib/bazel-gazelle/blob/master/reference.md#gazelle_binary) rule makes this easy: the user lists packages built with `go_library` that contain necessary extensions. Each package must contain a `New()` or `NewV2()` function that returns a value implementing the extension interfaces. 

`gazelle_binary` generates a source file that builds a list of extensions by calling either `New()` or `NewV2()` on each library. This source file is compiled together with the rest of Gazelle.

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

Gazelle uses the [`build` package](https://pkg.go.dev/github.com/bazelbuild/buildtools/build) (from buildifier and buildozer) to parse, edit, and format `BUILD` files. This package provides a low-level interface to the syntax tree. For more convenient editing and merging, Gazelle provides its own [`merger`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/merger) and [`rule`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/rule) packages.

The `rule` package lets extensions create, update, and delete "rules" (which become call expressions in the underlying syntax tree) and read or write their attributes using simple values rather than syntax tree nodes. For example, an extension can create a new rule as follows:

```go
r := rule.NewRule("go_binary", "server")
r.SetAttr("importpath", "example.com/hello/server")
r.SetAttr("srcs", []string{"main.go", "server.go"})
```

To add a new rule to a `BUILD` file, Gazelle must add it to [`File.Rules`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/rule#File), then call [`File.Save`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/rule#File.Save), which syncs changes to the syntax tree, formats the syntax tree to bytes, then writes the file. Extensions do not call `File.Save` directly: Gazelle does this once for each file, after all extensions have run.

### Merging changes to the syntax tree

`BUILD` files often contain a mix of human-written and machine-generated rules and attributes. Updating the machine-generated parts while preserving the human-written portion is a delicate process, so extensions do not directly modify the syntax tree. 

Instead, each extension's [`Generate`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Generator.Generate) method creates and returns two lists: a `Gen` list of rules the `BUILD` file should contain, and an `Empty` list of rules that should be deleted from the `BUILD` file, if they're present. This is often enough information to regenerate the `BUILD` file from scratch.

After calling `Generate`, Gazelle calls [`merger.MergeFile`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/merger#MergeFile) to merge the `Gen` and `Empty` lists with the rules that are already present in the `BUILD` file. `MergeFile` is language-neutral, but the way it handles rule attributes is controlled by the map returned by each extension's [`Kinds`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/language#Generator.Kinds) method. 

`MergeFile` processes each rule as follows:

1. `MergeFile` attempts to match the rule with an existing rule. An existing rule matches if:
    - It has the same kind and name (a `go_binary` with `name = "server"`).
    - One of its *matchable attributes* (determined by the `Kinds` map) has the same value (a `go_library` with `importpath = "example.com/hello/server"`).
    - If the rule kind's `MatchAny` flag is set in the `Kinds` map, then any rule of that kind can match. This is useful when only one rule is expected per directory.
1. If `MergeFile` doesn't find a match, then it either adds the rule if it was from the `Gen` list or ignores the rule if it was from the `Empty` list.
1. If `MergeFile` finds a match, it calls [`rule.MergeRules`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/rule#MergeRules) to combine the rules.
    - If an attribute is present in the new rule but not the existing rule, it's added.
    - If an attribute is present in the existing rule but not the new rule, it's deleted if the attribute is *mergeable* (determined by the `Kinds` map) or preserved if not.
    - If an attribute is present in both the existing and new rules:
        - If the attribute is not mergeable, the existing attribute is preserved. This is appropriate for human-written attributes with a machine generated default.
        - If the attribute is mergeable, the values are merged. The merge process depends on the type of value (string, list, etc.). New values typically replace existing values, but ordering and comments are preseved whenever possible.
        - Extension authors can modify merging behavior with values that implement the [`rule.Merger`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/rule#Merger) interface.
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

Even though the `name` attribute has changed, Gazelle can still match the generated rule with the existing rule because the `importpath` attribute is listed in the [`MatchAttrs`](https://pkg.go.dev/github.com/bazel-contrib/bazel-gazelle/v2/rule#KindInfo.MatchAttrs) list for `go_library`, and the value for that attribute is the same.

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
