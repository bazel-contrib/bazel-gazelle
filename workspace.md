# `WORKSPACE` setup

**NOTE:** Gazelle is dropping support for `WORKSPACE` mode in v2. This documentation is only for older versions of Gazelle.

To use Gazelle in a new project, add the `bazel_gazelle` repository and its dependencies to your WORKSPACE file and call `gazelle_dependencies`. It should look like this:

```bzl
load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

http_archive(
    name = "io_bazel_rules_go",
    integrity = "sha256-C4BclPs3MNwj3zKSXtR3s/TtN7VgddysbyGMPqe0q0I=",
    urls = [
        "https://github.com/bazel-contrib/rules_go/releases/download/v0.62.0/rules_go-v0.62.0.zip",
    ],
)

http_archive(
    name = "bazel_gazelle",
    integrity = "sha256-ZUm9N88bgrrEBhGa7xsmv+xdHALQ1aVRgnXkUT9Hs7I=",
    urls = [
        "https://github.com/bazel-contrib/bazel-gazelle/releases/download/v0.52.2/bazel-gazelle-v0.52.2.tar.gz",
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

go_register_toolchains(version = "1.26.5")

# Create the host platform repository transitively required by rules_go.
load("@bazel_tools//tools/build_defs/repo:utils.bzl", "maybe")
load("@platforms//host:extension.bzl", "host_platform_repo")

maybe(
    host_platform_repo,
    name = "host_platform",
)

gazelle_dependencies()
```

`gazelle_dependencies` supports optional arguments `go_env` (dict-mapping)
to set project specific go environment variables and `go_env_inherit`
(list of names) to copy selected variables from the host environment.
This is useful when dependency fetching relies on runtime-provided
authentication, proxy settings, or repository configuration that should
not be checked into source control. If you are using a
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
