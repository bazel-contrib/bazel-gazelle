"""Unit tests for //internal:env.bzl"""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//internal:env.bzl", "compute_env", "host_network_env")

# Settings reported by the mocked 'go env', as the go command computes them
# from the host environment and 'go env -w'.
_GO_ENV = {
    "GOAUTH": "netrc",
    "GOCACHE": "/home/user/.cache/go-build",
    "GOMODCACHE": "/home/user/go/pkg/mod",
    "GONOPROXY": "corp.example.com/*",
    "GONOSUMDB": "corp.example.com/*",
    "GOPATH": "/home/user/go",
    "GOPRIVATE": "corp.example.com/*",
    "GOPROXY": "https://proxy.example.com,direct",
    "GOSUMDB": "off",
}

_ENVIRON = {
    "GOFLAGS": "-mod=vendor",
    "HOME": "/home/user",
    "HTTPS_PROXY": "http://proxy.example.com:3128",
    "PATH": "/usr/bin",
    "SSL_CERT_FILE": "/etc/ssl/ca.pem",
}

def _mock_ctx(environ, go_env):
    """Mocks the parts of repository_ctx that compute_env uses.

    Args:
        environ: the host environment.
        go_env: the settings reported by 'go env'.

    Returns:
        a struct that can be passed to compute_env.
    """

    def execute(arguments, **_kwargs):
        if arguments[1:3] != ["env", "-json"]:
            fail("unexpected command: {}".format(arguments))
        return struct(
            return_code = 0,
            stdout = json.encode({name: go_env.get(name, "") for name in arguments[3:]}),
            stderr = "",
        )

    return struct(
        os = struct(name = "linux", environ = environ),
        getenv = lambda name, default = None: environ.get(name, default),
        execute = execute,
        path = lambda _label: struct(_name = "/sdk/ROOT", dirname = struct(_name = "/sdk")),
        watch = lambda _path: None,
    )

def _compute_env_test_impl(ctx):
    env = unittest.begin(ctx)

    # Download settings come from 'go env'. Nothing is taken from the host
    # environment directly: HTTP proxy and TLS settings are not persisted, and
    # neither is PATH.
    asserts.equals(
        env,
        {
            "GOAUTH": "netrc",
            "GONOPROXY": "corp.example.com/*",
            "GONOSUMDB": "corp.example.com/*",
            "GOPRIVATE": "corp.example.com/*",
            "GOPROXY": "https://proxy.example.com,direct",
            "GOROOT": "/sdk",
            "GOROOT_LABEL": str(Label("@go_sdk//:ROOT")),
            "GOSUMDB": "off",
            "GOTOOLCHAIN": "local",
        },
        compute_env(_mock_ctx(_ENVIRON, _GO_ENV), go_sdk_name = "@go_sdk"),
    )

    # Explicit settings take precedence, GONOPROXY and GONOSUMDB follow an
    # explicit GOPRIVATE, and other variables can still be inherited.
    asserts.equals(
        env,
        {
            "GOAUTH": "netrc",
            "GONOPROXY": "example.com/*",
            "GONOSUMDB": "example.com/*",
            "GOPRIVATE": "example.com/*",
            "GOPROXY": "https://proxy.example.com,direct",
            "GOROOT": "/sdk",
            "GOROOT_LABEL": str(Label("@go_sdk//:ROOT")),
            "GOSUMDB": "off",
            "GOTOOLCHAIN": "local",
            "PATH": "/usr/bin",
        },
        compute_env(
            _mock_ctx(_ENVIRON, _GO_ENV),
            go_sdk_name = "@go_sdk",
            go_env = {"GOPRIVATE": "example.com/*"},
            go_env_inherit = ["PATH"],
        ),
    )

    return unittest.end(env)

compute_env_test = unittest.make(_compute_env_test_impl)

def _host_network_env_test_impl(ctx):
    env = unittest.begin(ctx)

    # Only HTTP proxy and TLS settings are passed through to the go command.
    asserts.equals(
        env,
        {
            "HTTPS_PROXY": "http://proxy.example.com:3128",
            "SSL_CERT_FILE": "/etc/ssl/ca.pem",
        },
        host_network_env(_ENVIRON),
    )
    asserts.equals(env, {}, host_network_env({}))

    return unittest.end(env)

host_network_env_test = unittest.make(_host_network_env_test_impl)

def env_test_suite(name):
    unittest.suite(
        name,
        compute_env_test,
        host_network_env_test,
    )
