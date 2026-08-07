"""
A very basic use of the go_deps.module tag.

Only one module is declared. No go.mod or go.work file is referenced.
We should get a go_repository just for that module.
"""

load(
    "//internal/bzlmod:go_deps_test_case.bzl",
    "module",
    "module_tag",
    "tags",
    "test_case",
)

TEST = test_case(
    name = "module",
    modules = [module(
        name = "root",
        is_root = True,
        tags = tags(
            module = [module_tag(
                path = "golang.org/x/mod",
                version = "v0.38.0",
                sum = "h1:MECBjubtXD7yj4HrhIUcywNaGeNVUdfVnxmPajOk4yk=",
            )],
        ),
    )],
    want = [
        struct(
            name = "org_golang_x_mod",
            importpath = "golang.org/x/mod",
            version = "v0.38.0",
            sum = "h1:MECBjubtXD7yj4HrhIUcywNaGeNVUdfVnxmPajOk4yk=",
        ),
    ],
)
