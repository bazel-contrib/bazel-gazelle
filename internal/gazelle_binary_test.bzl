load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(
    ":gazelle_binary.bzl",
    "language_package",
    "languages_for_version",
)

def _language_package_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(env, "language/go", language_package("//language/go:go"))
    asserts.equals(env, "language/go", language_package(Label("//language/go:go")))
    asserts.equals(
        env,
        "language/bazel/visibility",
        language_package("@myrepo//language/bazel/visibility:visibility"),
    )
    asserts.equals(env, "language/defaults", language_package("//language/defaults"))
    return unittest.end(env)

_language_package_test = unittest.make(_language_package_test_impl)

def _languages_for_version_test_impl(ctx):
    env = unittest.begin(ctx)
    go = "//language/go:go"
    visibility = "//language/bazel/visibility:visibility"
    defaults = "//language/defaults"

    asserts.equals(env, [go], languages_for_version([go], 1))

    asserts.equals(
        env,
        [defaults, go],
        languages_for_version([go], 2),
    )
    asserts.equals(
        env,
        [defaults, go],
        languages_for_version([visibility, go], 2),
    )
    asserts.equals(
        env,
        [defaults, go],
        languages_for_version([defaults, go], 2),
    )
    return unittest.end(env)

_languages_for_version_test = unittest.make(_languages_for_version_test_impl)

def gazelle_binary_test_suite(name):
    unittest.suite(
        name,
        _language_package_test,
        _languages_for_version_test,
    )
