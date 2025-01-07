# Copyright 2019 The Bazel Authors. All rights reserved.
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

load("//internal:common.bzl", "executable_extension", "watch")

# Change to trigger cache invalidation: 1

def _go_repository_cache_impl(ctx):
    if ctx.attr.go_sdk_name:
        go_sdk_name = ctx.attr.go_sdk_name
    else:
        host_platform = _detect_host_platform(ctx)
        matches = [
            name
            for name, platform in ctx.attr.go_sdk_info.items()
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
    go_path = str(ctx.path("."))
    go_cache = str(ctx.path("gocache"))
    go_mod_cache = ""
    if ctx.os.environ.get("GO_REPOSITORY_USE_HOST_MODCACHE", "") == "1":
        extension = executable_extension(ctx)
        go_tool = go_root + "/bin/go" + extension
        go_mod_cache = read_go_env(ctx, go_tool, "GOMODCACHE")
        if not go_mod_cache:
            fail("GOMODCACHE must be set when GO_REPOSITORY_USE_HOST_MODCACHE is enabled.")
    if ctx.os.environ.get("GO_REPOSITORY_USE_HOST_CACHE", "") == "1":
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
        "GOROOT": go_root,
        "GOCACHE": go_cache,

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
    if go_mod_cache:
        cache_env["GOMODCACHE"] = go_mod_cache

    for key, value in ctx.attr.go_env.items():
        if key in cache_env:
            fail("{} cannot be set in go_env".format(key))
        cache_env[key] = value
    env_content = "\n".join(["{k}='{v}'\n".format(k = k, v = v) for k, v in cache_env.items()])

    ctx.file("go.env", env_content)
    ctx.file("BUILD.bazel", 'exports_files(["go.env"])')

go_repository_cache = repository_rule(
    _go_repository_cache_impl,
    attrs = {
        "go_sdk_name": attr.string(),
        "go_sdk_info": attr.string_dict(),
        "go_env": attr.string_dict(),
    },
    # Don't put anything in environ. If we switch between the host cache
    # and Bazel's cache, it shouldn't actually invalidate Bazel's cache.
)

def read_go_env(ctx, go_tool, var):
    watch(ctx, go_tool)
    res = ctx.execute([go_tool, "env", var])
    if res.return_code:
        fail("failed to read go environment: " + res.stderr)
    return res.stdout.strip()

def read_cache_env(ctx, path):
    contents = ctx.read(path)
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
