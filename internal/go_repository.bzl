# Copyright 2014 The Bazel Authors. All rights reserved.
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

load("@bazel_tools//tools/build_defs/repo:utils.bzl", "patch", "read_user_netrc", "use_netrc")
load("//internal:common.bzl", "env_execute", "executable_extension", "watch")
load("//internal:go_repository_cache.bzl", "read_cache_env")

_DOC = """
`go_repository` downloads a Go project and generates build files with Gazelle
if they are not already present. This is the simplest way to depend on
external Go projects.

When `go_repository` is in module mode, it saves downloaded modules in a shared,
internal cache within Bazel's cache. It may be cleared with `bazel clean --expunge`.
By setting the environment variable `GO_REPOSITORY_USE_HOST_CACHE=1`, you can
force `go_repository` to use the module cache on the host system in the location
returned by `go env GOPATH`. Alternatively, by setting the environment variable
`GO_REPOSITORY_USE_HOST_MODCACHE=1`, you can force `go_repository` to use only
the module cache on the host system in the location returned by `go env GOMODCACHE`.

**Example**

```starlark
load("@bazel_gazelle//:deps.bzl", "go_repository")

# Download using "go mod download"
go_repository(
    name = "com_github_pkg_errors",
    importpath = "github.com/pkg/errors",
    sum = "h1:iURUrRGxPUNPdy5/HRSm+Yj6okJ6UtLINN0Q9M4+h3I=",
    version = "v0.8.1",
)

# Download automatically via git
go_repository(
    name = "com_github_pkg_errors",
    commit = "816c9085562cd7ee03e7f8188a1cfd942858cded",
    importpath = "github.com/pkg/errors",
)

# Download from git fork
go_repository(
    name = "com_github_pkg_errors",
    commit = "816c9085562cd7ee03e7f8188a1cfd942858cded",
    importpath = "github.com/pkg/errors",
    remote = "https://example.com/fork/github.com/pkg/errors",
    vcs = "git",
)

# Download via HTTP
go_repository(
    name = "com_github_pkg_errors",
    importpath = "github.com/pkg/errors",
    urls = ["https://codeload.github.com/pkg/errors/zip/816c9085562cd7ee03e7f8188a1cfd942858cded"],
    strip_prefix = "errors-816c9085562cd7ee03e7f8188a1cfd942858cded",
    type = "zip",
)

# Download major version suffixed via git
go_repository(
    name = "com_github_thediveo_enumflag_v2",
    commit = "0217df583bf3d37b92798602e5061b36556bcd38",
    importpath = "github.com/thediveo/enumflag/v2",
    remote = "https://github.com/thediveo/enumflag",
    vcs = "git",
)
```

"""

# copied from
# https://github.com/bazelbuild/bazel/blob/d273cb62f43ef8169415cf60fc96e503ea2ad823/tools/build_defs/repo/http.bzl#L76
_AUTH_PATTERN_DOC = """An optional dict mapping host names to custom authorization patterns.

If a URL's host name is present in this dict the value will be used as a pattern when
generating the authorization header for the http request. This enables the use of custom
authorization schemes used in a lot of common cloud storage providers.

The pattern currently supports 2 tokens: <code>&lt;login&gt;</code> and
<code>&lt;password&gt;</code>, which are replaced with their equivalent value
in the netrc file for the same host name. After formatting, the result is set
as the value for the <code>Authorization</code> field of the HTTP request.

Example attribute and netrc for a http download to an oauth2 enabled API using a bearer token:

<pre>
auth_patterns = {
    "storage.cloudprovider.com": "Bearer &lt;password&gt;"
}
</pre>

netrc:

<pre>
machine storage.cloudprovider.com
        password RANDOM-TOKEN
</pre>

The final HTTP request would have the following header:

<pre>
Authorization: Bearer RANDOM-TOKEN
</pre>
"""

# We can't disable timeouts on Bazel, but we can set them to large values.
_GO_REPOSITORY_TIMEOUT = 86400

def _go_repository_impl(ctx):
    # TODO(#549): vcs repositories are not cached and still need to be fetched.
    # Download the repository or module.
    fetch_repo_args = None
    gazelle_path = None

    is_module_extension_repo = bool(ctx.attr.internal_only_do_not_use_apparent_name)

    # Explicitly watch label dependencies as they are only used as execute arguments.
    # https://bazel.build/extending/repo#when_is_the_implementation_function_executed
    go_env_cache = str(ctx.path(Label("@bazel_gazelle_go_repository_cache//:go.env")))
    watch(ctx, go_env_cache)
    fetch_repo = str(ctx.path(Label("@bazel_gazelle_go_repository_tools//:bin/fetch_repo{}".format(executable_extension(ctx)))))
    watch(ctx, fetch_repo)
    generate = ctx.attr.build_file_generation in ["on", "clean"]
    _gazelle = "@bazel_gazelle_go_repository_tools//:bin/gazelle{}".format(executable_extension(ctx))
    if generate:
        gazelle_path = ctx.path(Label(_gazelle))
        watch(ctx, gazelle_path)

    if ctx.attr.local_path:
        if hasattr(ctx, "watch_tree"):
            # https://github.com/bazelbuild/bazel/commit/fffa0affebbacf1961a97ef7cd248be64487d480
            ctx.watch_tree(ctx.attr.local_path)
        else:
            print("""
  WARNING: go.mod replace directives to module paths is only supported in bazel 7.1.0-rc1 or later,
          Because of this changes to %s will not be detected by your version of Bazel.""" % ctx.attr.local_path)

        fetch_repo_args = ["--path", ctx.attr.local_path, "--dest", ctx.path("")]
    elif ctx.attr.urls:
        # HTTP mode
        for key in ("commit", "tag", "vcs", "remote", "version", "sum", "replace"):
            if getattr(ctx.attr, key):
                fail("cannot specifiy both urls and %s" % key, key)
        result = ctx.download_and_extract(
            url = ctx.attr.urls,
            sha256 = ctx.attr.sha256,
            canonical_id = ctx.attr.canonical_id,
            stripPrefix = ctx.attr.strip_prefix,
            type = ctx.attr.type,
            auth = use_netrc(read_user_netrc(ctx), ctx.attr.urls, ctx.attr.auth_patterns),
        )
        if not ctx.attr.sha256:
            print("For Go module \"{path}\", integrity not specified, calculated sha256 = \"{sha256}\"".format(
                path = ctx.attr.importpath,
                sha256 = result.sha256,
            ))
        fetch_repo_args = ["-dest", ctx.path(""), "-no-fetch"]
    elif ctx.attr.commit or ctx.attr.tag:
        # repository mode
        if ctx.attr.commit:
            rev = ctx.attr.commit
            rev_key = "commit"
        elif ctx.attr.tag:
            rev = ctx.attr.tag
            rev_key = "tag"
        for key in ("urls", "strip_prefix", "type", "sha256", "version", "sum", "replace", "canonical_id"):
            if getattr(ctx.attr, key):
                fail("cannot specify both %s and %s" % (rev_key, key), key)

        if ctx.attr.vcs and not ctx.attr.remote:
            fail("if vcs is specified, remote must also be")

        fetch_repo_args = ["-dest", ctx.path(""), "-importpath", ctx.attr.importpath]
        if ctx.attr.remote:
            fetch_repo_args.extend(["--remote", ctx.attr.remote])
        if rev:
            fetch_repo_args.extend(["--rev", rev])
        if ctx.attr.vcs:
            fetch_repo_args.extend(["--vcs", ctx.attr.vcs])
    elif ctx.attr.version:
        # module mode
        for key in ("urls", "strip_prefix", "type", "sha256", "commit", "tag", "vcs", "remote"):
            if getattr(ctx.attr, key):
                fail("cannot specify both version and %s" % key)
        if not ctx.attr.sum:
            if is_module_extension_repo:
                fail("No sum for {}@{} found, update go.sum with:\n  bazel run".format(ctx.attr.importpath, ctx.attr.version), Label("@io_bazel_rules_go//go"), "-- mod tidy")
            else:
                fail("if version is specified, sum must also be")

        fetch_path = ctx.attr.replace if ctx.attr.replace else ctx.attr.importpath
        fetch_repo_args = [
            "-dest=" + str(ctx.path("")),
            "-importpath=" + fetch_path,
            "-version=" + ctx.attr.version,
            "-sum=" + ctx.attr.sum,
        ]
    else:
        fail("one of urls, commit, tag, or version must be specified")

    env = read_cache_env(ctx, go_env_cache)
    env_keys = [
        # keep sorted

        # Respect user proxy and sumdb settings for privacy.
        # TODO(jayconrod): gazelle in go_repository mode should probably
        # not go out to the network at all. This means *the build*
        # goes out to the network. We tolerate this for downloading
        # archives, but finding module roots is a bit much.
        "GONOPROXY",
        "GONOSUMDB",
        "GOPRIVATE",
        "GOPROXY",
        "GOSUMDB",

        # PATH is needed to locate git and other vcs tools.
        "PATH",

        # HOME is needed to locate vcs configuration files (.gitconfig).
        "HOME",

        # Settings below are used by vcs tools.
        "GIT_CONFIG",
        "GIT_CONFIG_COUNT",
        "GIT_CONFIG_GLOBAL",
        "GIT_CONFIG_NOSYSTEM",
        "GIT_CONFIG_SYSTEM",
        "GIT_SSH",
        "GIT_SSH_COMMAND",
        "GIT_SSL_CAINFO",
        "HTTPS_PROXY",
        "HTTP_PROXY",
        "NO_PROXY",
        "SSH_AUTH_SOCK",
        "SSL_CERT_DIR",
        "SSL_CERT_FILE",
        "http_proxy",
        "https_proxy",
        "no_proxy",
    ]

    # Git allows passing configuration through environmental variables, this will be picked
    # by go get properly: https://www.git-scm.com/docs/git-config/#Documentation/git-config.txt-GITCONFIGCOUNT
    if "GIT_CONFIG_COUNT" in ctx.os.environ:
        count = ctx.os.environ["GIT_CONFIG_COUNT"]
        if count:
            if not count.isdigit or int(count) < 1:
                fail("GIT_CONFIG_COUNT has to be a positive integer")
            count = int(count)
            for i in range(count):
                key = "GIT_CONFIG_KEY_%d" % i
                value = "GIT_CONFIG_VALUE_%d" % i
                for j in [key, value]:
                    if j not in ctx.os.environ:
                        fail("%s is not defined as an environment variable, but you asked for GIT_COUNT_COUNT=%d" % (j, count))
                env_keys = env_keys + [key, value]

    env.update({k: ctx.os.environ[k] for k in env_keys if k in ctx.os.environ})

    # Clean existing build files if requested
    if ctx.attr.build_file_generation == "clean":
        fetch_repo_args += ["-clean"]

    # Disable sumdb in fetch_repo. In module mode, the sum is a mandatory
    # attribute of go_repository, so we don't need to look it up.
    fetch_repo_env = dict(env)
    fetch_repo_env["GOSUMDB"] = "off"

    # Override external GO111MODULE, because it is needed by module mode, no-op in repository mode
    fetch_repo_env["GO111MODULE"] = "on"

    result = env_execute(
        ctx,
        [fetch_repo] + fetch_repo_args,
        environment = fetch_repo_env,
        timeout = _GO_REPOSITORY_TIMEOUT,
    )
    if result.return_code:
        fail("%s: %s" % (ctx.name, result.stderr))

    # Repositories are fetched. Determine if build file generation is needed.
    build_file_names = ctx.attr.build_file_name.split(",")
    existing_build_file = ""
    for name in build_file_names:
        path = ctx.path(name)
        if path.exists and not env_execute(ctx, ["test", "-f", path]).return_code:
            existing_build_file = name
            break

    generate = generate or (not existing_build_file and ctx.attr.build_file_generation == "auto")

    if generate:
        # Build file generation is needed. Populate Gazelle directive at root build file
        build_file_name = existing_build_file or build_file_names[0]
        if len(ctx.attr.build_directives) > 0:
            ctx.file(
                build_file_name,
                "\n".join(["# " + d for d in ctx.attr.build_directives]),
            )

        # Run Gazelle
        if gazelle_path == None:
            gazelle_path = ctx.path(Label(_gazelle))

        if is_module_extension_repo:
            # This repository is generated by the 'go_deps' extension. Since as of Bazel 7.2.0 there
            # is no API that constructs a label referencing a sibling repo from within a repo rule,
            # we rely on the assumption that the apparent name of the extension-generated repos is
            # the last component of their canonical names.
            extension_repo_prefix = ctx.attr.name[:-len(ctx.attr.internal_only_do_not_use_apparent_name)]
            repo_config = ctx.path(Label("@@" + extension_repo_prefix + "bazel_gazelle_go_repository_config//:WORKSPACE"))
        else:
            repo_config = ctx.path(ctx.attr.build_config)

        watch(ctx, gazelle_path)
        watch(ctx, repo_config)

        cmd = [
            gazelle_path,
            "-go_repository_mode",
            "-go_prefix",
            ctx.attr.importpath,
            "-mode",
            "fix",
            "-repo_root",
            ctx.path(""),
            "-repo_config",
            repo_config,
        ]
        if ctx.attr.version or ctx.attr.local_path:
            cmd.append("-go_repository_module_mode")
        if ctx.attr.build_file_name:
            cmd.extend(["-build_file_name", ctx.attr.build_file_name])
        if ctx.attr.build_tags:
            cmd.extend(["-build_tags", ",".join(ctx.attr.build_tags)])
        if ctx.attr.build_external:
            cmd.extend(["-external", ctx.attr.build_external])
        if ctx.attr.build_file_proto_mode:
            cmd.extend(["-proto", ctx.attr.build_file_proto_mode])
        if ctx.attr.build_naming_convention:
            cmd.extend(["-go_naming_convention", ctx.attr.build_naming_convention])
        if is_module_extension_repo:
            cmd.append("-bzlmod")
        cmd.extend(ctx.attr.build_extra_args)
        cmd.append(ctx.path(""))
        ctx.report_progress("running Gazelle")
        result = env_execute(ctx, cmd, environment = env, timeout = _GO_REPOSITORY_TIMEOUT)
        if result.return_code:
            fail("failed to generate BUILD files for %s: %s" % (
                ctx.attr.importpath,
                result.stderr,
            ))
        if ctx.attr.debug_mode and result.stderr:
            print("%s gazelle.stdout: %s" % (ctx.name, result.stdout))
            print("%s gazelle.stderr: %s" % (ctx.name, result.stderr))

    # Apply patches if necessary.
    patch(ctx)

    if generate:
        # Do not override a REPO.bazel patched in by users. This also provides a
        # way for users to opt out of Gazelle-generated package_info.
        repo_file = ctx.path("REPO.bazel")
        if not repo_file.exists:
            ctx.file("REPO.bazel", """\
repo(
    default_package_metadata = [
        "//:gazelle_generated_package_info",
        "//:package_metadata",
    ],
)
""")

            # Modify the top-level build file after patches have been applied as the
            # patches may otherwise conflict with our generated content.
            build_file = ctx.path(build_file_name)
            if build_file.exists:
                build_file_content = ctx.read(build_file)
            else:
                build_file_content = ""
            build_file_content += _generate_package_info(
                importpath = ctx.attr.importpath,
                version = ctx.attr.version,
            )
            ctx.file(build_file_name, build_file_content)

def _generate_package_info(*, importpath, version):
    package_name = importpath

    # TODO: Consider adding support for custom remotes.
    package_url = "https://{}".format(importpath) if version else None
    package_version = version.removeprefix("v") if version else None

    # See specification:
    # https://github.com/package-url/purl-spec/blob/master/PURL-TYPES.rst#golang
    # scheme:type/namespace/name@version?qualifiers#subpath
    purl = "pkg:golang/{namespace_and_name}@{version}".format(
        namespace_and_name = importpath,
        version = version if version else "unknown",
    )

    return """
load("@package_metadata//rules:package_metadata.bzl", "package_metadata")
load("@rules_license//rules:package_info.bzl", "package_info")

package_metadata(
    name = "package_metadata",
    purl = {purl},
    visibility = [
        "//:__subpackages__",
    ],
)

package_info(
    name = "gazelle_generated_package_info",
    package_name = {package_name},
    package_url = {package_url},
    package_version = {package_version},
    purl = {purl},
    visibility = ["//:__subpackages__"],
)
""".format(
        package_name = repr(package_name),
        package_url = repr(package_url),
        package_version = repr(package_version),
        purl = repr(purl),
    )

go_repository = repository_rule(
    implementation = _go_repository_impl,
    doc = _DOC,
    attrs = {
        # Fundamental attributes of a go repository
        "importpath": attr.string(
            doc = """The Go import path that matches the root directory of this repository.

            In module mode (when `version` is set), this must be the module path. If
            neither `urls` nor `remote` is specified, `go_repository` will
            automatically find the true path of the module, applying import path
            redirection.

            If build files are generated for this repository, libraries will have their
            `importpath` attributes prefixed with this `importpath` string.  """,
            mandatory = True,
        ),

        # Attributes for a repository that should be checked out from VCS
        "commit": attr.string(
            doc = """If the repository is downloaded using a version control tool, this is the
            commit or revision to check out. With git, this would be a sha1 commit id.
            `commit` and `tag` may not both be set.""",
        ),
        "tag": attr.string(
            doc = """If the repository is downloaded using a version control tool, this is the
            named revision to check out. `commit` and `tag` may not both be set.""",
        ),
        "vcs": attr.string(
            default = "",
            doc = """One of `"git"`, `"hg"`, `"svn"`, `"bzr"`.

            The version control system to use. This is usually determined automatically,
            but it may be necessary to set this when `remote` is set and the VCS cannot
            be inferred. You must have the corresponding tool installed on your host.""",
            values = [
                "",
                "git",
                "hg",
                "svn",
                "bzr",
            ],
        ),
        "remote": attr.string(
            doc = """The VCS location where the repository should be downloaded from. This is
            usually inferred from `importpath`, but you can set `remote` to download
            from a private repository or a fork.""",
        ),

        # Attributes for a repository that should be downloaded via HTTP.
        "urls": attr.string_list(
            doc = """A list of HTTP(S) URLs where an archive containing the project can be
            downloaded. Bazel will attempt to download from the first URL; the others
            are mirrors.""",
        ),
        "strip_prefix": attr.string(
            doc = """If the repository is downloaded via HTTP (`urls` is set), this is a
            directory prefix to strip. See [`http_archive.strip_prefix`].""",
        ),
        "type": attr.string(
            doc = """One of `"zip"`, `"tar.gz"`, `"tgz"`, `"tar.bz2"`, `"tar.xz"`.

            If the repository is downloaded via HTTP (`urls` is set), this is the
            file format of the repository archive. This is normally inferred from the
            downloaded file name.""",
        ),
        "sha256": attr.string(
            doc = """If the repository is downloaded via HTTP (`urls` is set), this is the
            SHA-256 sum of the downloaded archive. When set, Bazel will verify the archive
            against this sum before extracting it.

            **CAUTION:** Do not use this with services that prepare source archives on
            demand, such as codeload.github.com. Any minor change in the server software
            can cause differences in file order, alignment, and compression that break
            SHA-256 sums.""",
        ),
        "canonical_id": attr.string(
            doc = """If the repository is downloaded via HTTP (`urls` is set) and this is set, restrict cache hits to those cases where the
            repository was added to the cache with the same canonical id.""",
        ),
        "auth_patterns": attr.string_dict(
            doc = _AUTH_PATTERN_DOC,
        ),

        # Attributes for a module that should be loaded from the local file system.
        "local_path": attr.string(
            doc = """ If specified, `go_repository` will load the module from this local directory""",
        ),

        # Attributes for a module that should be downloaded with the Go toolchain.
        "version": attr.string(
            doc = """If specified, `go_repository` will download the module at this version
            using `go mod download`. `sum` must also be set. `commit`, `tag`,
            and `urls` may not be set. """,
        ),
        "sum": attr.string(
            doc = """A hash of the module contents. In module mode, `go_repository` will verify
            the downloaded module matches this sum. May only be set when `version`
            is also set.

            A value for `sum` may be found in the `go.sum` file or by running
            `go mod download -json <module>@<version>`.""",
        ),
        "replace": attr.string(
            doc = """A replacement for the module named by `importpath`. The module named by
            `replace` will be downloaded at `version` and verified with `sum`.

            NOTE: There is no `go_repository` equivalent to file path `replace`
            directives. Use `local_repository` instead.""",
        ),

        # Attributes for a repository that needs automatic build file generation
        "build_external": attr.string(
            default = "static",
            doc = """One of `"external"`, `"static"` or `"vendored"`.

            This sets Gazelle's `-external` command line flag. In `"static"` mode,
            Gazelle will not call out to the network to resolve imports.

            **NOTE:** This cannot be used to ignore the `vendor` directory in a
            repository. The `-external` flag only controls how Gazelle resolves
            imports which are not present in the repository. Use
            `build_extra_args = ["-exclude=vendor"]` instead.""",
            values = [
                "",
                "external",
                "static",
                "vendored",
            ],
        ),
        "build_file_name": attr.string(
            default = "BUILD.bazel,BUILD",
            doc = """Comma-separated list of names Gazelle will consider to be build files.
            If a repository contains files named `build` that aren't related to Bazel,
            it may help to set this to `"BUILD.bazel"`, especially on case-insensitive
            file systems.""",
        ),
        "build_file_generation": attr.string(
            default = "auto",
            doc = """One of `"auto"`, `"on"`, `"off"`, `"clean"`.

            Whether Gazelle should generate build files in the repository. In `"auto"`
            mode, Gazelle will run if there is no build file in the repository root
            directory. In `"clean"` mode, Gazelle will first remove any existing build
            files.""",
            values = [
                "on",
                "auto",
                "off",
                "clean",
            ],
        ),
        "build_naming_convention": attr.string(
            values = [
                "go_default_library",
                "import",
                "import_alias",
            ],
            default = "import_alias",
            doc = """Sets the library naming convention to use when resolving dependencies against this external
            repository. If unset, the convention from the external workspace is used.
            Legal values are `go_default_library`, `import`, and `import_alias`.

            See the `gazelle:go_naming_convention` directive in [Directives] for more information.""",
        ),
        "build_tags": attr.string_list(
            doc = "This sets Gazelle's `-build_tags` command line flag.",
        ),
        "build_file_proto_mode": attr.string(
            doc = """One of `"default"`, `"legacy"`, `"disable"`, `"disable_global"` or `"package"`.

            This sets Gazelle's `-proto` command line flag. See [Directives] for more
            information on each mode.""",
            values = [
                "",
                "default",
                "package",
                "disable",
                "disable_global",
                "legacy",
            ],
        ),
        "build_extra_args": attr.string_list(
            doc = "A list of additional command line arguments to pass to Gazelle when generating build files.",
        ),
        "build_config": attr.label(
            default = "@bazel_gazelle_go_repository_config//:WORKSPACE",
            doc = """A file that Gazelle should read to learn about external repositories before
            generating build files. This is useful for dependency resolution. For example,
            a `go_repository` rule in this file establishes a mapping between a
            repository name like `golang.org/x/tools` and a workspace name like
            `org_golang_x_tools`. Workspace directives like
            `# gazelle:repository_macro` are recognized.

            `go_repository` rules will be re-evaluated when parts of WORKSPACE related
            to Gazelle's configuration are changed, including Gazelle directives and
            `go_repository` `name` and `importpath` attributes.
            Their content should still be fetched from a local cache, but build files
            will be regenerated. If this is not desirable, `build_config` may be set
            to a less frequently updated file or `None` to disable this functionality.""",
        ),
        "build_directives": attr.string_list(
            default = [],
            doc = """A list of directives to be written to the root level build file before
            Calling Gazelle to generate build files. Each string in the list will be
            prefixed with `#` automatically. A common use case is to pass a list of
            Gazelle directives.""",
        ),

        # Patches to apply after running gazelle.
        "patches": attr.label_list(
            doc = "A list of patches to apply to the repository after gazelle runs.",
        ),
        "patch_tool": attr.string(
            default = "",
            doc = """The patch tool used to apply `patches`. If this is specified, Bazel will
            use the specifed patch tool instead of the Bazel-native patch implementation.""",
        ),
        "patch_args": attr.string_list(
            default = ["-p0"],
            doc = "Arguments passed to the patch tool when applying patches.",
        ),
        "patch_cmds": attr.string_list(
            default = [],
            doc = "Commands to run in the repository after patches are applied.",
        ),

        # Attributes that affect the verbosity of logging:
        "debug_mode": attr.bool(
            default = False,
            doc = """Enables logging of fetch_repo and Gazelle output during succcesful runs. Gazelle can be noisy
            so this defaults to `False`. However, setting to `True` can be useful for debugging build failures and
            unexpected behavior for the given rule.
            """,
        ),
        "internal_only_do_not_use_apparent_name": attr.string(doc = "Internal usage only"),
    },
)
"""See repository.md#go-repository for full documentation."""
