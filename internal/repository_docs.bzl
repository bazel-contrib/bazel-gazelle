# Module used by stardoc to generate API documentation.
# Not meant for use by bazel-gazelle users.
"""
Repository rules
================

Repository rules are Bazel rules that can be used in WORKSPACE files to import
projects in external repositories. Repository rules may download projects
and transform them by applying patches or generating build files.

The Gazelle repository provides the following repository rule:

* [`go_repository`](#go_repository) downloads a Go project using either `go mod download`, a
  version control tool like `git`, or a direct HTTP download. It understands
  Go import path redirection. If build files are not already present, it can
  generate them with Gazelle.

Repository rules can be loaded and used in WORKSPACE like this:

```starlark
load("@bazel_gazelle//:deps.bzl", "go_repository")

go_repository(
    name = "com_github_pkg_errors",
    commit = "816c9085562cd7ee03e7f8188a1cfd942858cded",
    importpath = "github.com/pkg/errors",
)
```

Gazelle can add and update some of these rules automatically using the
`update-repos` command. For example, the rule above can be added with:

```shell
$ gazelle update-repos github.com/pkg/errors
```
"""

load("go_repository.bzl", _go_repository = "go_repository")

go_repository = _go_repository
