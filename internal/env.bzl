# Copyright 2026 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Functions for computing environment varibles when invoking Go tools."""

load(
    ":common.bzl",
    "executable_extension",
    "watch",
)

def compute_env(
        ctx,
        *,
        go_sdk_name = None,
        go_sdk_info = {},
        go_env = {},
        go_env_inherit = []):
    """
    Computes the environment to use for Go toolchain invocations

    Args:
        ctx: a repository_ctx or module_ctx, giving access to the host environment.
        go_sdk_name: a string or None, the name of the go_sdk repo to use, used in Bzlmod mode.
        go_sdk_info: map from repo name to "goos_goarch" platform or "host", used in WORKSPACE mode.
        go_env: map from environment variable names to values, explicit settings.
        go_env_inherit: list of environment variable names to inherit from the host environment.

    Returns:
        a dict of environment variable settings.
    """

    if go_sdk_name:
        go_sdk_name = go_sdk_name
    else:
        host_platform = _detect_host_platform(ctx)
        matches = [
            name
            for name, platform in go_sdk_info.items()
            if host_platform == platform or platform == "host"
        ]
        if len(matches) > 1:
            fail('gazelle found more than one suitable Go SDK ({}). Specify which one to use with gazelle_dependencies(go_sdk = "go_sdk").'.format(", ".join(matches)))
        if len(matches) == 0:
            fail('gazelle could not find a Go SDK. Specify which one to use with gazelle_dependencies(go_sdk = "go_sdk").')
        if len(matches) == 1:
            go_sdk_name = matches[0]

    go_sdk_label = Label("@" + go_sdk_name + "//:ROOT")

    go_root = str(ctx.path(go_sdk_label).dirname)
    go_path = ""  # default: the cache repo itself; recomputed by read_go_env_file()
    go_cache = ""  # default: <cache repo>/gocache; recomputed by read_go_env_file()
    go_mod_cache = ""
    if getenv(ctx, "GO_REPOSITORY_USE_HOST_MODCACHE") == "1":
        extension = executable_extension(ctx)
        go_tool = go_root + "/bin/go" + extension
        go_mod_cache = read_go_env(ctx, go_tool, "GOMODCACHE")
        if not go_mod_cache:
            fail("GOMODCACHE must be set when GO_REPOSITORY_USE_HOST_MODCACHE is enabled.")
    if getenv(ctx, "GO_REPOSITORY_USE_HOST_CACHE") == "1":
        extension = executable_extension(ctx)
        go_tool = go_root + "/bin/go" + extension
        go_mod_cache = read_go_env(ctx, go_tool, "GOMODCACHE")
        go_path = read_go_env(ctx, go_tool, "GOPATH")
        if not go_mod_cache and not go_path:
            fail("GOPATH or GOMODCACHE must be set when GO_REPOSITORY_USE_HOST_CACHE is enabled.")
        go_cache = read_go_env(ctx, go_tool, "GOCACHE")
        if not go_cache:
            fail("GOCACHE must be set when GO_REPOSITORY_USE_HOST_CACHE is enabled.")

    cache_env = {
        # Record the SDK repo's ROOT file as a label rather than an absolute
        # path so that consumers are forced to add a dependency on the Go SDK.
        # This avoids a class of staleness issues, both with and without repo
        # content caches.
        "GOROOT": str(go_sdk_label),

        # Since Go v1.21.0, set GOTOOLCHAIN to "local" to use the current toolchain
        # of the Go SDK. This is required to avoid `go mod download` commands
        # download and use another Go toolchain according to the user’s environment
        # default file (GOENV, managed by `go env -w` and `go env -u`).
        #
        # See https://go.dev/doc/toolchain for more info.
        #
        # TODO(#1858): Find a way to retrieve this value when the host's Go SDK is used and avoid to override it.
        "GOTOOLCHAIN": "local",
    }
    if go_path:
        cache_env["GOPATH"] = go_path
    if go_cache:
        cache_env["GOCACHE"] = go_cache
    if go_mod_cache:
        cache_env["GOMODCACHE"] = go_mod_cache

    cache_env.update(resolve_env(
        ctx,
        direct = go_env,
        inherit = go_env_inherit,
        reserved = cache_env.keys() + ["GOCACHE", "GOPATH"],
    ))

    return cache_env

def write_go_env_file(ctx, env_dict):
    """Writes a go.env file that can be read by Go or read_go_env_file"""
    env_content = "\n".join(["{k}='{v}'\n".format(k = k, v = v) for k, v in env_dict.items()])
    ctx.file("go.env", env_content)

def resolve_env(ctx, direct = {}, inherit = [], reserved = []):
    """
    Builds an environment map from explicit values and host env passthroughs.

    Args:
        ctx: a module_ctx or repository_ctx.
        direct: dict of explicit settings for environment variables.
        inherit: list of environment variable names whose values should be taken
            from the host environment.
        reserved: list of environment variable names that must not be set by
            either direct or inherit.

    Returns:
        A dict of environment variable settings.
    """
    env = {}
    reserved_keys = {key: True for key in reserved}
    inherited_keys = {}
    for key, value in direct.items():
        if key in reserved_keys:
            fail("{} cannot be set in go_env".format(key))
        env[key] = value

    for key in inherit:
        if key in inherited_keys:
            continue
        inherited_keys[key] = True
        if key in reserved_keys:
            fail("{} cannot be set in go_env_inherit".format(key))
        if key in env:
            fail("{} cannot be set in both go_env and go_env_inherit".format(key))

        # Use getenv when available so repository rules invalidate if the
        # underlying host environment changes.
        value = getenv(ctx, key, None)
        if value != None:
            env[key] = value

    return env

def getenv(ctx, name, default = ""):
    """
    Gets an environment variable.

    Uses ctx.getenv if it exists (after Bazel 7.1.0) to also invalidate
    the repository rule when the env var changes. We can remove this wrapper
    after the minimal supported Bazel version is >= 7.1.0.
    Args:
        ctx: a module_ctx or repository_ctx.
        name: the name of the environment variable to get.
        default: the value to return, if the variable is not set.

    Returns:
        The value of the environment variable or default.
    """
    if hasattr(ctx, "getenv"):
        return ctx.getenv(name, default)
    return ctx.os.environ.get(name, default)

def read_go_env(ctx, go_tool, var):
    """
    Runs 'go env' to find Go's opinion on what an environment variable should be

    Args:
        ctx: a repository_ctx or module_ctx, giving access to the host environment.
        go_tool: path to the go binary
        var: the environment variable to check, like GOROOT

    Returns:
        Go's value for that environment variable
    """
    watch(ctx, go_tool)

    # watch var too if possible.
    getenv(ctx, var)
    res = ctx.execute([go_tool, "env", var])
    if res.return_code:
        fail("failed to read go environment: " + res.stderr)
    return res.stdout.strip()

def read_go_env_file(ctx, env_path, cache_dir_file = None):
    """
    Reads a go.env file, resolving labels to absolute paths as needed.

    Two repos have go.env files. Make sure to read the correct one!
    - @bazel_gazelle_go_repository_cache: used by go_repository_tools and by
      go_repository in WORKSPACE mode. Can only be configured in WORKSPACE mode
      via gazelle_dependencies.
    - @bazel_gazelle_go_repository_config: used by go_repository repos created
      by go_deps. Can be configured with go_deps.config.go_env and
      go_env_inherit.

    Args:
        ctx: a repository_ctx or module_ctx.
        env_path: path, label, or string for the go.env file to read.
        cache_dir_file: optional path, label, or string for a file within
            @bazel_gazelle_go_repository_cache. GOPATH and GOCACHE default to
            this repo's directory when unset in env_path. Defaults to env_path.

    Returns:
        A dict of environment variables, ready for execution. Do not write to
        a file, since it contains absolute paths.
    """
    contents = ctx.read(env_path)
    env = {}
    lines = contents.split("\n")
    for line in lines:
        line = line.strip()
        if line == "" or line.startswith("#"):
            continue
        k, sep, v = line.partition("=")
        if sep == "":
            fail("failed to parse cache environment")
        env[k] = v.strip("'")

    # Resolve the GOROOT label (see _go_repository_cache_impl) to an absolute
    # path and register a dependency by doing so.
    if env.get("GOROOT"):
        env["GOROOT"] = str(ctx.path(Label(env["GOROOT"])).dirname)
    if cache_dir_file == None:
        cache_dir_file = env_path
    cache_dir = str(ctx.path(cache_dir_file).dirname)
    env.setdefault("GOPATH", cache_dir)
    env.setdefault("GOCACHE", cache_dir + "/gocache")
    return env

# copied from rules_go. Keep in sync.
def _detect_host_platform(ctx):
    if ctx.os.name == "linux":
        host = "linux_amd64"
        res = ctx.execute(["uname", "-p"])
        if res.return_code == 0:
            uname = res.stdout.strip()
            if uname == "s390x":
                host = "linux_s390x"
            elif uname == "i686":
                host = "linux_386"

        # uname -p is not working on Aarch64 boards
        # or for ppc64le on some distros
        res = ctx.execute(["uname", "-m"])
        if res.return_code == 0:
            uname = res.stdout.strip()
            if uname == "aarch64":
                host = "linux_arm64"
            elif uname == "armv6l":
                host = "linux_arm"
            elif uname == "armv7l":
                host = "linux_arm"
            elif uname == "ppc64le":
                host = "linux_ppc64le"

        # Default to amd64 when uname doesn't return a known value.

    elif ctx.os.name == "mac os x":
        host = "darwin_amd64"
        res = ctx.execute(["uname", "-m"])
        if res.return_code == 0:
            uname = res.stdout.strip()
            if uname == "arm64":
                host = "darwin_arm64"

    elif ctx.os.name.startswith("windows"):
        host = "windows_amd64"
    elif ctx.os.name == "freebsd":
        host = "freebsd_amd64"
    else:
        fail("Unsupported operating system: " + ctx.os.name)

    return host
