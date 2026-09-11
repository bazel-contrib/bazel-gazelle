load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//internal:env.bzl", "host_env")

def _host_env_test_impl(ctx):
    env = unittest.begin(ctx)

    # Proxy, sumdb, and VCS settings are passed through; other variables that
    # could change how the go command behaves are not.
    asserts.equals(
        env,
        {
            "GOPROXY": "https://proxy.example.com",
            "GOPRIVATE": "example.com/*",
            "PATH": "/usr/bin",
            "HOME": "/home/user",
            "HTTPS_PROXY": "http://proxy.example.com:3128",
            "SSL_CERT_FILE": "/etc/ssl/ca.pem",
        },
        host_env({
            "GOFLAGS": "-mod=vendor",
            "GOPROXY": "https://proxy.example.com",
            "GOPRIVATE": "example.com/*",
            "GOWORK": "off",
            "HOME": "/home/user",
            "HTTPS_PROXY": "http://proxy.example.com:3128",
            "PATH": "/usr/bin",
            "SSL_CERT_FILE": "/etc/ssl/ca.pem",
        }),
    )

    # Git configuration is passed through GIT_CONFIG_COUNT.
    asserts.equals(
        env,
        {
            "GIT_CONFIG_COUNT": "2",
            "GIT_CONFIG_KEY_0": "url.ssh://git@example.com/.insteadOf",
            "GIT_CONFIG_VALUE_0": "https://example.com/",
            "GIT_CONFIG_KEY_1": "safe.directory",
            "GIT_CONFIG_VALUE_1": "*",
        },
        host_env({
            "GIT_CONFIG_COUNT": "2",
            "GIT_CONFIG_KEY_0": "url.ssh://git@example.com/.insteadOf",
            "GIT_CONFIG_VALUE_0": "https://example.com/",
            "GIT_CONFIG_KEY_1": "safe.directory",
            "GIT_CONFIG_VALUE_1": "*",
            "GIT_CONFIG_KEY_2": "unused",
        }),
    )

    asserts.equals(env, {}, host_env({}))

    return unittest.end(env)

host_env_test = unittest.make(_host_env_test_impl)

def env_test_suite(name):
    unittest.suite(
        name,
        host_env_test,
    )
