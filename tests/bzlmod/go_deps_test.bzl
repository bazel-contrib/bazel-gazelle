load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//internal/bzlmod:go_deps.bzl", "get_repo_name")

def _get_repo_name_default_test_impl(ctx):
    env = unittest.begin(ctx)

    # Without any override, the repo name is derived from the import path.
    asserts.equals(
        env,
        "com_example_foo_bar_baz",
        get_repo_name("example.com/foo/bar/baz", {}),
    )
    asserts.equals(
        env,
        "com_example_foo_bar_baz",
        get_repo_name("example.com/foo_bar_baz", {}),
    )

    # An override without a repo_name falls back to the derived name.
    asserts.equals(
        env,
        "com_example_foo_bar_baz",
        get_repo_name("example.com/foo/bar/baz", {
            "example.com/foo/bar/baz": struct(repo_name = ""),
        }),
    )

    # An override for a different path does not affect this one.
    asserts.equals(
        env,
        "com_example_foo_bar_baz",
        get_repo_name("example.com/foo/bar/baz", {
            "example.com/other": struct(repo_name = "custom_name"),
        }),
    )

    return unittest.end(env)

get_repo_name_default_test = unittest.make(_get_repo_name_default_test_impl)

def _get_repo_name_override_test_impl(ctx):
    env = unittest.begin(ctx)

    # A non-empty repo_name override is used verbatim, breaking the collision
    # with another module that mangles to the same default name.
    asserts.equals(
        env,
        "com_example_foo_bar_baz_alt",
        get_repo_name("example.com/foo_bar_baz", {
            "example.com/foo_bar_baz": struct(repo_name = "com_example_foo_bar_baz_alt"),
        }),
    )

    return unittest.end(env)

get_repo_name_override_test = unittest.make(_get_repo_name_override_test_impl)

def go_deps_test_suite(name):
    unittest.suite(
        name,
        get_repo_name_default_test,
        get_repo_name_override_test,
    )
