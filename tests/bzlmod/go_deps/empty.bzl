"""
An empty module.

Tests what happens when go_deps is loaded, but no tags are declared.
"""

load(
    "//internal/bzlmod:go_deps_test_case.bzl",
    "module",
    "test_case",
)

TEST = test_case(
    name = "empty",
    modules = [module(name = "empty", is_root = True)],
    want = [
        struct(name = "bazel_gazelle_go_repository_config"),
    ],
)
