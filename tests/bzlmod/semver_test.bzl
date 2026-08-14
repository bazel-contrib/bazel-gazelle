load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//internal/bzlmod:semver.bzl", "semver")

_SORTED_TEST_VERSIONS = [
    "0.1-a",
    "0.1",
    "1.0.0-0.3.7",
    "1.0.0-alpha",
    "1.0.0-alpha+001",
    "1.0.0-alpha.1",
    "1.0.0-alpha.beta",
    "1.0.0-beta+exp.sha.5114f85",
    "1.0.0-beta",
    "1.0.0-beta.2",
    "1.0.0-beta.11",
    "1.0.0-rc.1",
    "1.0.0-x.7.z.92",
    "1.0.0-x-y-z.--",
    "1.0.0+21AF26D3----117B344092BD",
    "1.0.0+20130313144700",
    "1.0.0",
    "2.0.0",
    "2.1.0",
    "2.1.1-0",
    "2.1.1",
    "2.1.1.0",
    "2.1.1.1-a",
    "2.1.1.1",
    "2.1.1.a",
    "2.1.2",
    "3",
    "a",
]

_SCRAMBLED_TEST_VERSIONS = {
    "a": True,
    "3": True,
    "2.1.2": False,
    "2.1.1.a": True,
    "2.1.1.1": True,
    "2.1.1.1-a": True,
    "2.1.1.0": True,
    "2.1.1": False,
    "2.1.1-0": False,
    "2.1.0": False,
    "2.0.0": False,
    "1.0.0+21AF26D3----117B344092BD": False,
    "1.0.0+20130313144700": False,
    "1.0.0": False,
    "1.0.0-x.7.z.92": False,
    "1.0.0-x-y-z.--": False,
    "1.0.0-rc.1": False,
    "1.0.0-beta.11": False,
    "1.0.0-beta.2": False,
    "1.0.0-beta+exp.sha.5114f85": False,
    "1.0.0-beta": False,
    "1.0.0-alpha.beta": False,
    "1.0.0-alpha.1": False,
    "1.0.0-alpha": False,
    "1.0.0-alpha+001": False,
    "1.0.0-0.3.7": False,
    "0.1-a": True,
    "0.1": True,
}

def _semver_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(
        env,
        _SORTED_TEST_VERSIONS,
        sorted(
            _SCRAMBLED_TEST_VERSIONS.keys(),
            key = lambda x: semver.to_comparable(x, relaxed = _SCRAMBLED_TEST_VERSIONS[x]),
        ),
    )
    return unittest.end(env)

semver_test = unittest.make(_semver_test_impl)

def _semver_max_test_impl(ctx):
    env = unittest.begin(ctx)
    asserts.equals(env, "v1.2.3", semver.max("v1.2.3", "v1.1.0"))
    asserts.equals(env, "v1.2.3", semver.max("v1.0.0", "v1.2.3"))
    return unittest.end(env)

semver_max_test = unittest.make(_semver_max_test_impl)

def semver_test_suite(name):
    unittest.suite(
        name,
        semver_test,
        semver_max_test,
    )
