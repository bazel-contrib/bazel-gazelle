"""Unit tests for //internal:env.bzl"""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load("//internal:env.bzl", "compute_env", "host_network_env", "resolve_go_env")

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

def _mock_ctx(environ, go_env, os_name = "linux"):
    """Mocks the parts of repository_ctx that compute_env and resolve_go_env use.

    Args:
        environ: the host environment.
        go_env: the settings reported by 'go env'.
        os_name: the value of repository_ctx.os.name.

    Returns:
        a struct that can be passed to compute_env or resolve_go_env.
    """

    def execute(arguments, **_kwargs):
        if arguments[1:3] != ["env", "-json"]:
            fail("unexpected command: {}".format(arguments))
        return struct(
            return_code = 0,
            stdout = json.encode({name: go_env.get(name, "") for name in arguments[3:]}),
            stderr = "",
        )

    def path(label):
        if str(label).endswith("go.env"):
            return struct(_name = "/cache/go.env", dirname = struct(_name = "/cache"))
        return struct(_name = "/sdk/ROOT", dirname = struct(_name = "/sdk"))

    return struct(
        os = struct(name = os_name, environ = environ),
        getenv = lambda name, default = None: environ.get(name, default),
        execute = execute,
        path = path,
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

def _resolve_go_env_test_impl(ctx):
    env = unittest.begin(ctx)

    cache_env = {"GOPROXY": "https://proxy.example.com,direct"}
    resolved = {
        "GOCACHE": "/cache/gocache",
        "GOPATH": "/cache",
        "GOPROXY": "https://proxy.example.com,direct",
    }

    # On other platforms, Go falls back to /tmp, so no temporary directory is
    # needed.
    asserts.equals(
        env,
        resolved,
        resolve_go_env(_mock_ctx({}, _GO_ENV), cache_env, "/cache/go.env"),
    )

    # On Windows, the command gets a temporary directory in the cache repo,
    # like GOCACHE. The host environment must not be consulted.
    for environ in [{}, {"TMP": "C:/Temp", "TEMP": "C:/Temp"}]:
        asserts.equals(
            env,
            resolved | {"TEMP": "/cache/tmp", "TMP": "/cache/tmp"},
            resolve_go_env(
                _mock_ctx(environ, _GO_ENV, os_name = "windows 11"),
                cache_env,
                "/cache/go.env",
            ),
        )

    # Like GOPATH and GOCACHE, a setting from go.env wins, for both variables.
    for temp_var in ["TMP", "TEMP"]:
        asserts.equals(
            env,
            resolved | {"TEMP": "D:/tmp", "TMP": "D:/tmp"},
            resolve_go_env(
                _mock_ctx({}, _GO_ENV, os_name = "windows 11"),
                cache_env | {temp_var: "D:/tmp"},
                "/cache/go.env",
            ),
        )

    return unittest.end(env)

resolve_go_env_test = unittest.make(_resolve_go_env_test_impl)

def env_test_suite(name):
    unittest.suite(
        name,
        compute_env_test,
        host_network_env_test,
        resolve_go_env_test,
    )
