# Proto Reference

This page describes the directives and flags defined by Gazelle's Protobuf extension. See [Configuration and command line reference](../../gazelle-reference.md) for a general command-line reference.

## Directives

The Protobuf extension defines the following directives.

**Directive:** `# gazelle:proto default|file|package|legacy|disable|disable_global`<br>
**Default:** `default`<br>
Tells Gazelle how to generate rules for .proto files. Valid values are:

* `default`: `proto_library`, `go_proto_library`, and `go_library` rules are generated using `@io_bazel_rules_go//proto:def.bzl`. Only one of each rule may be generated per directory. This is the default mode.
* `file`: a `proto_library` rule is generated for every .proto file.
* `package`: multiple `proto_library` and `go_proto_library` rules may be generated in the same directory. .proto files are grouped into rules based on their package name or another option (see `proto_group`).
* `legacy`: `filegroup` rules are generated for use by `@io_bazel_rules_go//proto:go_proto_library.bzl`. `go_proto_library` rules must be written by hand. Gazelle will run in this mode automatically if `go_proto_library.bzl` is loaded to avoid disrupting existing projects, but this can be overridden with a directive.
* `disable`: .proto files are ignored. Gazelle will run in this mode automatically if `go_proto_library` is loaded from any other source, but this can be overridden with a directive.
* `disable_global`: like `disable` mode, but also prevents Gazelle from using any special cases in dependency resolution for Well Known Types and Google APIs. Useful for avoiding build-time dependencies on protoc.

This directive applies to the current directory and subdirectories. As a special case, when Gazelle enters a directory named `vendor`, if the proto mode isn't set explicitly in a parent directory or on the command line, Gazelle will run in `disable` mode. Additionally, if the file `@io_bazel_rules_go//proto:go_proto_library.bzl` is loaded, Gazelle will run in `legacy` mode.

**Directive:** `# gazelle:proto_group option`<br>
**Default:** n/a<br>
*This directive is only effective in* `package` *mode (see above).*

Specifies an option that Gazelle can use to group .proto files into rules. For example, when set to `go_package`, .proto files with the same `option go_package` will be grouped together.

When this directive is set to the empty string, Gazelle will group packages by their proto package statement.

Rule names are generated based on the last run of identifier characters in the package name. For example, if the package is `"foo/bar/baz"`, the `proto_library` rule will be named `baz_proto`.

**Directive:** `# gazelle:proto_import_prefix path`<br>
**Default:** n/a<br>
Sets the [`import_prefix`](https://docs.bazel.build/versions/master/be/protocol-buffer.html#proto_library.import_prefix) attribute of generated `proto_library` rules. This adds a prefix to the string used to import `.proto` files listed in the `srcs` attribute of generated rules.

For example, if the target `//a:b_proto` has `srcs = ["b.proto"]` and `import_prefix = "github.com/x/y"`, then `b.proto` should be imported with the string `"github.com/x/y/a/b.proto"`.

**Directive:** `# gazelle:proto_search strip prefix`<br>
**Default:** n/a<br>
When lazy indexing is enabled (`-index=lazy`), this directive tells Gazelle how to transform a proto import string into a repo-root-relative directory path where the proto might be found.

Like `go_search`, this directive configures lazy indexing. However, the arguments are more similar to [`cc_search`](https://github.com/EngFlow/gazelle_cc?tab=readme-ov-file#-gazellecc_search-strip_include_prefix-include_prefix) because protobuf rules handle import strings similarly to how C++ handles include strings.

As an example, suppose you have a library in `third_party/foo/` with the label `//third_party/foo`. It has a proto file `third_party/foo/proto/api.proto` that you include as `foo/api.proto`. The library's `proto_library` target might be written as:

```bzl
proto_library(
    name = "foo",
    srcs = ["api.proto"],
    strip_import_prefix = "third_party/foo/proto",
    import_prefix = "foo",
    visibility = ["//visibility:public"],
)
```

You can tell Gazelle how to find this library when lazy indexing is enabled with the
directive:

```bzl
# gazelle:proto_search foo third_party/foo/proto
```

The first argument is a prefix to remove from an import string. The second is a prefix
to add. So when Gazelle sees the import string `foo/api.proto` in a file, it's transformed
to `third_party/foo/proto/api.proto`. Gazelle then indexes the directory
`third_party/foo/proto` after removing the base name.

You can specify the `proto_search` directive multiple times. It applies in the directory
where it's written and to subdirectories. An empty `proto_search` directory resets the
list of translation rules for the current directory.

**Directive:** `# gazelle:proto_strip_import_prefix path`
**Default:** n/a
Sets the [`strip_import_prefix`](https://docs.bazel.build/versions/master/be/protocol-buffer.html#proto_library.strip_import_prefix) attribute of generated `proto_library` rules. This is a prefix to strip from the strings used to import `.proto` files.

If the prefix starts with a slash, it's intepreted relative to the repository root. Otherwise, it's relative to the directory containing the build file. The package-relative form is only useful when a single build file covers `.proto` files in subdirectories. Gazelle doesn't generate build files like this, so only paths with a  leading slash should be used. Gazelle will print a warning when the package-relative form is used.

For example, if the target `//proto/a:b_proto` has `srcs = ["b.proto"]` and `strip_import_prefix = "/proto"`, then `b.proto` should be imported with the string `"a/b.proto"`.

## Flags

**Flag:** `-proto=default|file|package|legacy|disable|disable_global`<br>
**Default:** `default`<br>
Determines how Gazelle should generate rules for .proto files. See details in [Directives](#directives) below.

**Flag:** `-proto_group=group`<br>
**Default:** n/a<br>
Determines the proto option Gazelle uses to group .proto files into rules when in `package` mode. See details in [Directives](#directives) below.

**Flag:** `-proto_import_prefix=path`<br>
**Default:** n/a<br>
Sets the [`import_prefix`](https://docs.bazel.build/versions/master/be/protocol-buffer.html#proto_library.import_prefix) attribute of generated `proto_library` rules. This adds a prefix to the string used to import `.proto` files listed in the `srcs` attribute of generated rules. Equivalent to the `# gazelle:proto_import_prefix` directive. See details in [Directives](#directives) below.

## `fix` command transformations

The Protobuf extension does not apply any additional transformations when the `fix` command is used.
