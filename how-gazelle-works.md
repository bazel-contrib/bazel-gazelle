# How Gazelle Works

This page explains how Gazelle generates and updates `BUILD` files. It's intended to help Gazelle developers, extension authors, and users wishing to understand how (and why) Gazelle makes its changes.

## Overview

Gazelle updates build files in the following stages. Each is described in detail below.

1. [Load](#load): Parse `BUILD` files and load directory metadata.
1. [Generate](#generate): Generate new rules and build an index of existing library rules.
1. [Resolve](#resolve): Map imports in source files to Bazel labels in `deps` attributes.
1. [Write](#write): Format and save modified `BUILD` files.

All of Gazelle's language-specific functionality is implemented in plugins called [*extensions*](#extensions).

### Load

When Gazelle starts, it begins traversing the directory tree. This process runs in parallel with later stages as an optimization.

In each directory it visits, Gazelle parses the `BUILD` or `BUILD.bazel` file if present and makes a list of files and subdirectories, excluding those matched by `# gazelle:exclude` directives or `.bazelignore` files. This metadata is cached in memory so that later stages may access it quickly without requiring additional I/O.

Gazelle may or may not visit a directory based on directives and command line flags.

- Gazelle always visits directories named as command line arguments. If no arguments are specified, Gazelle visits the repository root directory.
- If recursion is enabled (with `-r=true`, enabled by default), Gazelle recursively visits subdirectories.
- If eager indexing is enabled (with `-index=all`, enabled by default), Gazelle visits *all* directories.
- If lazy indexing is enabled (with `-index=lazy`), Gazelle visits directories requested by language extensions in [`GenerateResult.RelsToIndex`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#GenerateResult.RelsToIndex). These directories are loaded lazily during the *Generate* stage.
- If indexing is disabled (with `-index=none`), Gazelle does not visit additional directories.
- Gazelle visits parent directories within the repository in addition to other directories it visits. This is necessary to apply `# gazelle:exclude` directives, which may tell Gazelle to act as if a subdirectory does not exist.

The Load stage is implemented in [`walk.walker.populateCache`](https://github.com/bazel-contrib/bazel-gazelle/blob/028c500e9f911a73683b6ec390f3e59e8f31fccc/walk/dirinfo.go#L104), which is called from [`walk.Walk2`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/walk#Walk2). No extension methods are called during this stage.

### Generate

As Gazelle visits each directory, it calls extension methods to apply configuration, fix deprecated usage, generate rules, and index libraries. Most of Gazelle's work happens during this stage.

1. [`Configure`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/config#Configurer.Configure) is called in each directory Gazelle visits. Each extension can read directives from the `BUILD` file (if there is one) to decide what to do. Most directives apply to the directory the appear in and to subdirectories.
    - `Configure` is called in each directory in pre-order. All other methods in this stage are called in post-order. Sibling directories are visited sequentially (not concurrently) in lexicographic order.
    - `Configure` is called in each directory Gazelle visits, even if Gazelle won't update the `BUILD` file.
1. [`Fix`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.Fix) is called in each directory that has an existing `BUILD` file. The purpose of this method is to fix deprecated rule usage, so extensions can make arbitrary transformations here.
1. [`GenerateRules`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.GenerateRules) is called in each directory. This method returns rules that should be present in the `BUILD` file, and rules that should be removed. Unlike `Fix`, this method must not actually modify the rules parse from the `BUILD` file.
1. [`merger.MergeFile`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/merger#MergeFile) is called to combine the generated rules with existing rules.
    - `MergeFile` attempts to match each rule with an existing rule. If a rule is not matched, it's added to the end of the file. Usually the matching is based on the rule kind (a `go_library` named `client`), but it can be influenced by other heuristics. 
    - Each attribute is merged separately. An attribute can be *mergeable* or not. If an attribute is mergeable, it's expected to be managed by Gazelle, so the merger can overwrite values that are already there (except for values marked with `# keep`). If an attribute is not mergeable, Gazelle may set an initial value, but won't overwrite it later.
    - `MergeFile` also merges empty rules returned by `GenerateRules` with existing rules. This can delete existing rules if they're not marked with `# keep`. For example, this allows Gazelle to delete a `go_test` rule after all the `_test.go` files were removed.
    - Extensions don't directly participate in the merging process, though they can influence matching and merging by returning [`rule.KindInfo`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/rule#KindInfo) for each generate rule kind from the [`Kinds`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.Kinds) method.
1. [`Imports`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.47.0/resolve#Resolver.Imports) is called on each rule after the merge to build an in-memory index for dependency resolution. This table maps import strings and language names to Bazel labels. `Imports` is not called if indexing is disabled with `-index=none`.

The extension methods `Fix`, `GenerateRules`, and `MergeFile` are only called in directories where Gazelle was asked to update the `BUILD` file. `Configure` and `Imports` are called in all directories Gazelle visits.

The Generate stage is implemented in the [callback function](https://github.com/bazel-contrib/bazel-gazelle/blob/028c500e9f911a73683b6ec390f3e59e8f31fccc/cmd/gazelle/fix-update.go#L322) passed to [`walk.Walk2`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/walk#Walk2) as part of the [`runFixUpdate`](https://github.com/bazel-contrib/bazel-gazelle/blob/028c500e9f911a73683b6ec390f3e59e8f31fccc/cmd/gazelle/fix-update.go#L263) function.

### Resolve

Gazelle resolves dependencies as a separate stage so it can use the index built by calling `Imports` during the previous stage.

1. [`Resolve`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/resolve#Resolver.Resolve) is called on each generated rule.
    - The extension that generated the rule should resolve dependencies, often using the index, then set `deps` and any related attributes.
    - To avoid redundant I/O, an extension may return information about import strings found in source files through [`GenerateResult.Imports`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#GenerateResult.Imports) when returning from [`GenerateRules`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language.GenerateRules). This value is opaque to Gazelle. It's passed back to `Resolve`.
1. [`merger.MergeFile`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/merger#MergeFile) is called again to merge changes with existing rules in `BUILD` files.

### Write

At this point, Gazelle has made all necessary changes to `BUILD` files in memory. It then formats files with [`build.Format`](https://pkg.go.dev/github.com/bazelbuild/buildtools/build#Format) and writes files back to disk.

The `-mode` flag modifies this slightly. `-mode=print` causes Gazelle to print the modified files. `-mode=diff` causes Gazelle to print the differences as a patch.

## Extensions

This section explains how Gazelle uses extensions. See [Extending Gazelle](https://github.com/bazel-contrib/bazel-gazelle/blob/master/extend.md) for a guide to writing a new extension.

Gazelle provides a language-agnostic framework for generating rules and updating `BUILD` files. All the language-specific functionality is implemented in *extensions*. An extension is an implementation of the [`language.Language`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/language#Language) interface, which requires `GenerateRules` and a few other methods. `language.Language` also embeds [`config.Configurer`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle@v0.47.0/config#Configurer) and [`resolve.Resolver`](https://pkg.go.dev/github.com/bazelbuild/bazel-gazelle/resolve#Resolver).

Gazelle can't dynamically load extensions at run-time: the Gazelle binary must be built with all extensions the user might need. The [`gazelle_binary`](https://github.com/bazel-contrib/bazel-gazelle/blob/master/reference.md#gazelle_binary) rule makes this easy: the user lists packages built with `go_library` that contain extensions. Each package must contain a `New()` function that returns a value implementing the extension interface. `gazelle_binary` generates a source file that calls each of those `New()` functions, then compiles that together with the rest of Gazelle.

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
