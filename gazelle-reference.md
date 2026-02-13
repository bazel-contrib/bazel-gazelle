# Configuration and command line reference

This page describes Gazelle's general-purpose commands, directives, and flags. Language-specific directives and flags are described on separate pages. See [Go](language/go/reference.md) and [proto](language/proto/reference.md) for extensions defined in this repo.

## Usage

```
gazelle <command> [flags...] [directories...]
```

The first argument to Gazelle may be one of the commands below. If no command is specified, `update` is assumed. The remaining arguments are specific to each command and are documented below.

- **[update](#fix-and-update):** Scans sources files, then generates and updates build files.
- **[fix](#fix-and-update):** Same as the `update` command, but it also fixes deprecated usage of rules.
- **[update-repos](language/go/reference.md#update-repos):** Adds and updates repository rules in the WORKSPACE file.

## `fix` and `update`

The `update` command is the most common way of running Gazelle. Gazelle scans sources in directories throughout the repository, then creates and updates build files.

The `fix` command does everything `update` does, but it also fixes deprecated usage of rules, analogous to `go fix`. For example, `cgo_library` will be consolidated with `go_library`. This command may delete or rename rules, so it's not used by default. The transformations are documented with each language extension: see [Go](language/go/reference.md#fix-command-transformations) and [proto](language/proto/reference.md#fix-command-transformations) for details.

Both commands accept a list of directories to process as positional arguments. If no directories are specified, Gazelle will process the current directory. Subdirectories will be processed recursively by default (unless `-r=false`).

### Flags

The following general purpose flags are accepted. See [Go: Flags](language/go/reference.md#flags) and [Proto: Flags](language/proto/reference.md#flags) for flags defined by language extensions in this repo.

Many flags have equivalent [directives](#directives) that may be written in `BUIlD` files rather than passed on the command line. When possible, use directives instead of flags. Directives are more consistent and readable for developers working on a project, and they are more precise, since they can be set in specific subdirectories.

**Flag:** `-build_file_name=file1,file2,...`<br>
**Default:** `BUILD.bazel,BUILD`<br>
Comma-separated list of file names. Gazelle recognizes these files as Bazel build files. New files will use the first name in this list. Use this if your project contains non-Bazel files named `BUILD` (or `build` on case-insensitive file systems).

**Flag:** `-build_tags=tag1,tag2,...`<br>
**Default:** n/a<br>
List of Go build tags Gazelle will defer to Bazel for evaluation. Gazelle applies constraints when generating Go rules. It assumes certain tags are true on certain platforms (for example, `amd64,linux`). It assumes all Go release tags are true (for example, `go1.8`). It considers other tags to be false (for example, `ignore`). This flag allows custom tags to be evaluated by Bazel at build time. Bazel may still filter sources with these tags. Use `bazel build --define gotags=foo,bar` to set tags at build time.

**Flag:** `-exclude=pattern`<br>
**Default:** n/a<br>
Prevents Gazelle from processing a file or directory if the given [`doublestar.Match`](https://github.com/bmatcuk/doublestar#match) pattern matches. If the pattern refers to a source file, Gazelle won't include it in any rules. If the pattern refers to a directory, Gazelle won't recurse into it. This option may be repeated. Patterns must be slash-separated, relative to the repository root. This is equivalent to the `# gazelle:exclude pattern` directive.

**Flag:** `-index=none|lazy|all`<br>
**Default:** `all`<br>
Determines whether Gazelle should index the libraries in the current repository and whether it should use the index to resolve dependencies.

If `none` or `false`, indexing is disabled, and Gazelle relies purely on conventions to translate language-specific import strings into dependency labels.

If `lazy`, Gazelle indexes libraries in directories it visits explicitly. Language extensions may be configured to index additional directories through directives like `# gazelle:go_search`. This mode is very fast when recursion is disabled with `-r=false`.

If `all` or `true`, Gazelle indexes all directories in the repository, even when recursion is disabled. This makes dependency resolution simple but can be slow for large repositories.

**Flag:** `-mode=fix|print|diff`<br>
**Default:** `fix`<br>
Method for emitting merged build files.

- In `fix` mode, Gazelle writes generated and merged files to disk.
- In `print` mode, Gazelle prints updated files to stdout and does not write files to disk.
- In `diff` mode, Gazelle prints a unified diff to stdout and does not write files to disk.

**Flag:** `-r`<br>
**Default:** `true`<br>
Controls whether Gazelle recurses into subdirectories of the directories named on the command line. This is enabled by default, so when Gazelle is run from the repository root directory without arguments, it visits and updates all directories. This can be slow for large repositories.

When recursion is disabled, Gazelle only visits specific named directories. This can be very fast, but you may also want to use lazy indexing (`-index=lazy`) or disable indexing altogether (`-index=none`).

**Flag:** `-repo_root=dir`<br>
**Default:** inferred<br>
The root directory of the repository. Gazelle normally infers this to be the directory containing the WORKSPACE file. Gazelle will not process packages outside this directory.

**Flag:** `-remove_noop_keep_comments`<br>
**Default:** `false`<br>
Whether Gazelle will remove `# keep` comments when the thing being kept would have been kept without the comment. This is always enabled when run with the `fix` command, and for the `update` command must be specified. This will only remove `# keep` comments targeting list items, e.g. not rules, entire lists/dicts, or dict items.

**Flag:** `-lang=lang1,lang2`<br>
**Default:** n/a<br>
Selects languages for which to compose and index rules. By default, all languages that this Gazelle was built with are processed.

**Flag:** `-cpuprofile=filename`<br>
**Default:** n/a<br>
If specified, gazelle uses [runtime/pprof](https://pkg.go.dev/runtime/pprof#StartCPUProfile) to collect CPU profiling information from the command and save it to the given file. By default, this is disabled.

**Flag:** `-memprofile=filename`<br>
**Default:** n/a<br>
If specified, gazelle uses [runtime/pprof](https://pkg.go.dev/runtime/pprof#WriteHeapProfile) to collect memory a profile information from the command and save it to a file. By default, this is disabled.

## `update-repos`

The `update-repos` command updates Go repository rules in Bazel's `WORKSPACE` mode. See [Go: update-repos](language/go/reference.md#update-repos) for details.

## Directives

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

The following general-purpose directives are recognized. See [Go: Directives](language/go/reference.md#directives) and [Proto: Directives](language/proto/reference.md#directives) for directives defined by language extensions in this repo.

**Directive:** `# gazelle:alias_kind macro_name wrapped_kind`<br>
**Default:** n/a<br>
Denotes that a macro aliases / wraps a given rule.

If you have a wrapper macro around a rule that gazelle knows how to update the attrs for, the alias_kind directive will instruct gazelle that it should treat the particular marco like the underlying wrapped kind.

`alias_kind` is different from the `map_kind` directive in that it does not force the rule to be generated as the wrapped kind. Instead, it just instructs gazelle that it should index and update the attrs for rules that match the macro.

For example, if you use `# gazelle:alias_kind my_foo_binary foo_binary`, Gazelle will still generate `foo_binary` targets when generating new targets from new source files. It is up to a person to update the `foo_binary` targets to `my_foo_binary` targets. Once this manual step is done, Gazelle will continue to update the `my_foo_binary` targets as if they were `foo_binary` targets.

Wrapper macros are commonly used to handle common boilerplate or to add deploy/release verbs, as described in the bazel [Verbs Tutorial](https://bazel.build/rules/verbs-tutorial).

**Directive:** `# gazelle:build_file_names name1,name2...`<br>
**Default:** `BUILD.bazel,BUILD`<br>
Comma-separated list of file names. Gazelle recognizes these files as Bazel build files. New files will use the first name in this list. Use this if your project contains non-Bazel files named `BUILD` (or `build` on case-insensitive file systems).

**Directive:** `# gazelle:build_tags foo,bar`<br>
**Default:** n/a<br>
List of Go build tags Gazelle will defer to Bazel for evaluation. Gazelle applies constraints when generating Go rules. It assumes certain tags are true on certain platforms (for example, `amd64,linux`). It assumes all Go release tags are true (for example, `go1.8`). It considers other tags to be false (for example, `ignore`). This flag allows custom tags to be evaluated by Bazel at build time. Bazel may still filter sources with these tags. Use `bazel build --define gotags=foo,bar` to set tags at build time.

**Directive:** `# gazelle:exclude pattern`<br>
**Default:** n/a<br>
Prevents Gazelle from processing a file or directory if the given [`doublestar.Match`](https://pkg.go.dev/github.com/bmatcuk/doublestar/v4#Match) pattern matches. If the pattern refers to a source file, Gazelle won't include it in any rules. If the pattern refers to a directory, Gazelle won't recurse into it. This directive may be repeated to exclude multiple patterns, one per line.

**Directive:** `# gazelle:follow pattern`<br>
**Default:** n/a<br>
Instructs Gazelle to follow a symbolic link to a directory within the repository if the given [`doublestar.Match`](https://pkg.go.dev/github.com/bmatcuk/doublestar/v4#Match) pattern matches. Normally, Gazelle does not follow symbolic links unless they point outside of the repository root. Care must be taken to avoid visiting a directory more than once. The `# gazelle:exclude` directive may be used to prevent Gazelle from recursing into a directory.

**Directive:** `# gazelle:generation_mode create_and_update|update_only`<br>
**Default:** `create_and_update`<br>
Declares if gazelle should create and update `BUILD` files per directory or only update existing `BUILD` files. Valid values are: `create_and_update` and `update_only`.

**Directive:** `# gazelle:ignore`<br>
**Default:** n/a<br>
Prevents Gazelle from modifying the build file. Gazelle will still read rules in the build file and may modify build files in subdirectories.

**Directive:** `# gazelle:map_kind from_kind to_kind to_kind_load`<br>
**Default:** n/a<br>
Customizes the kind of rules generated by Gazelle.

As a separate step after generating rules, any new rules of kind `from_kind` have their kind replaced with `to_kind`. This means that `to_kind` must accept the same parameters and behave similarly.

Most commonly, this would be used to replace the rules provided by `rules_go` with custom macros. For example, `gazelle:map_kind go_binary go_deployable //tools/go:def.bzl` would configure Gazelle to produce rules of kind `go_deployable` as loaded from `//tools/go:def.bzl` instead of `go_binary`, for this directory or within.

Existing rules of the old kind will be ignored. To switch your codebase from a builtin kind to a mapped kind, use [buildozer](https://github.com/bazelbuild/buildtools/tree/master/buildozer).

**Directive:** `# gazelle:resolve source-lang [import-lang] import-string label`<br>
**Default:** n/a<br>
Specifies an explicit mapping from an import string to a label for [Dependency resolution](#dependency-resolution). Accepts the following arguments:

* `source-lang` is the language of the source code being imported.
* `import-lang` (optional) is the language importing the library. This is usually the same as `source-lang` but may differ with generated code. For example, when resolving dependencies for a `go_proto_library`, `source-lang` would be `"proto"` and `import-lang` would be `"go"`. `import-lang` may be omitted if it is the same as `source-lang`.
* `import-string` is the string used in source code to import a library.
* `label` is the Bazel label that Gazelle should write in `deps`.

For example:

```bzl
# gazelle:resolve go example.com/foo //foo:go_default_library
# gazelle:resolve proto go foo/foo.proto //foo:foo_go_proto
```

**Directive:** `# gazelle:resolve_regexp source-lang import-lang import-string-regexp label`<br>
**Default:** n/a<br>
Specifies an explicit mapping from an import regex to a label for [Dependency resolution](#dependency-resolution). Accepts the following arguments:

* `source-lang` is the language of the source code being imported.
* `import-lang` (optional) is the language importing the library. This is usually the same as `source-lang` but may differ with generated code. For example, when resolving dependencies for a `go_proto_library`, `source-lang` would be `"proto"` and `import-lang` would be `"go"`. `import-lang` may be omitted if it is the same as `source-lang`.
* `import-string-regex` is the regex applied to the import in the source code. If it matches, that import will be resolved to the label specified below.
* `label` is the Bazel label that Gazelle should write in `deps`. The label can be constructed using captured strings from the subpattern matching in `import-string-regex`.

For example:

```bzl
# gazelle:resolve_regexp go example.com/.* //foo:go_default_library
# gazelle:resolve_regexp proto go foo/.*\.proto //foo:foo_go_proto
# gazelle:resolve_regexp proto go foo/(.*)\.proto //foo/$1:foo_rule_proto
```

**Directive:** `# gazelle:lang lang1,lang2`<br>
**Default:** n/a<br>
Sets the language selection flag for this and descendent packages, which causes gazelle to index and generate rules for only the languages named in this directive.

**Directive:** `# gazelle:default_visibility visibility`<br>
**Default:** n/a<br>
Comma-separated list of visibility specifications. This directive adds the visibility specifications for this and descendant packages. For example:

```bzl
# gazelle:default_visibility //foo:__subpackages__,//src:__subpackages__
```

You must include the extension `@gazelle//language/bazel/visibility` to use this directive.

### `WORKSPACE` directives

Gazelle also reads directives from the WORKSPACE file. They may be used to discover custom repository names and known prefixes. The `fix` and `update` commands use these directives for dependency resolution. `update-repos` uses them to learn about repository rules defined in alternate locations.

**Directive:** `# gazelle:repository rule_kind attr1_name=attr1_value,...`<br>
**Default:** n/a<br>
Specifies a repository rule that Gazelle should know about. The directive can be repeated multiple times, and can be declared from within a macro definition that Gazelle knows about. At the very least the directive must define a rule kind and a name attribute, but it can define extra attributes after that.

This is useful for teaching Gazelle about repos declared in external macros. The directive can also be used to override an actual repository rule. For example, a `git_repository` rule for `org_golang_x_tools` could be overriden with the directive:

```bzl
# gazelle:repository go_repository name=org_golang_x_tools importpath=golang.org/x/tools
```

Gazelle would then proceed as if `org_golang_x_tools` was declared as a `go_repository` rule.

**Directive:** `# gazelle:repository_macro [+]macroFile%defName`<br>
**Default:** n/a<br>
Tells Gazelle to look for repository rules in a macro in a .bzl file. The directive can be repeated multiple times. The macro can be generated by calling `update-repos` with the `to_macro` flag. The directive can be prepended with a `+`, which will tell Gazelle to also look for repositories within any macros called by the specified macro.

## `keep` comments

In addition to directives, Gazelle supports `# keep` comments that protect parts of build files from being modified. `# keep` may be written before a rule, before an attribute, or after a string within a list.

`# keep` comments might take one of 2 forms; the `# keep` literal or a description prefixed by `# keep:`.

For example, sppose you have a library that includes a generated .go file. Gazelle won't know what imports to resolve, so you may need to add dependencies manually with
`# keep` comments.

```bzl
load("@io_bazel_rules_go//go:def.bzl", "go_library")
load("@com_github_example_gen//:gen.bzl", "gen_go_file")

gen_go_file(
    name = "magic",
    srcs = ["magic.go.in"],
    outs = ["magic.go"],
)

go_library(
    name = "go_default_library",
    srcs = ["magic.go"],
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_example_gen//:go_default_library",  # keep
        "@com_github_example_gen//a/b/c:go_default_library",  # keep: this is also important
    ],
)
```
