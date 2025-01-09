<!-- Generated with Stardoc: http://skydoc.bazel.build -->

Repository rules
================

Repository rules are Bazel rules that can be used in WORKSPACE files to import
projects in external repositories. Repository rules may download projects
and transform them by applying patches or generating build files.

The Gazelle repository provides the following repository rule:

* [`go_repository`](#go_repository) downloads a Go project using either `go mod download`, a
  version control tool like `git`, or a direct HTTP download. It understands
  Go import path redirection. If build files are not already present, it can
  generate them with Gazelle.

Repository rules can be loaded and used in WORKSPACE like this:

```starlark
load("@bazel_gazelle//:deps.bzl", "go_repository")

go_repository(
    name = "com_github_pkg_errors",
    commit = "816c9085562cd7ee03e7f8188a1cfd942858cded",
    importpath = "github.com/pkg/errors",
)
```

Gazelle can add and update some of these rules automatically using the
`update-repos` command. For example, the rule above can be added with:

```shell
$ gazelle update-repos github.com/pkg/errors
```

<a id="go_repository"></a>

## go_repository

<pre>
go_repository(<a href="#go_repository-name">name</a>, <a href="#go_repository-auth_patterns">auth_patterns</a>, <a href="#go_repository-build_config">build_config</a>, <a href="#go_repository-build_directives">build_directives</a>, <a href="#go_repository-build_external">build_external</a>, <a href="#go_repository-build_extra_args">build_extra_args</a>,
              <a href="#go_repository-build_file_generation">build_file_generation</a>, <a href="#go_repository-build_file_name">build_file_name</a>, <a href="#go_repository-build_file_proto_mode">build_file_proto_mode</a>, <a href="#go_repository-build_naming_convention">build_naming_convention</a>,
              <a href="#go_repository-build_tags">build_tags</a>, <a href="#go_repository-canonical_id">canonical_id</a>, <a href="#go_repository-commit">commit</a>, <a href="#go_repository-debug_mode">debug_mode</a>, <a href="#go_repository-importpath">importpath</a>,
              <a href="#go_repository-internal_only_do_not_use_apparent_name">internal_only_do_not_use_apparent_name</a>, <a href="#go_repository-local_path">local_path</a>, <a href="#go_repository-patch_args">patch_args</a>, <a href="#go_repository-patch_cmds">patch_cmds</a>, <a href="#go_repository-patch_tool">patch_tool</a>,
              <a href="#go_repository-patches">patches</a>, <a href="#go_repository-remote">remote</a>, <a href="#go_repository-replace">replace</a>, <a href="#go_repository-repo_mapping">repo_mapping</a>, <a href="#go_repository-sha256">sha256</a>, <a href="#go_repository-strip_prefix">strip_prefix</a>, <a href="#go_repository-sum">sum</a>, <a href="#go_repository-tag">tag</a>, <a href="#go_repository-type">type</a>, <a href="#go_repository-urls">urls</a>, <a href="#go_repository-vcs">vcs</a>,
              <a href="#go_repository-version">version</a>)
</pre>

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

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="go_repository-name"></a>name |  A unique name for this repository.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="go_repository-auth_patterns"></a>auth_patterns |  An optional dict mapping host names to custom authorization patterns.<br><br>If a URL's host name is present in this dict the value will be used as a pattern when generating the authorization header for the http request. This enables the use of custom authorization schemes used in a lot of common cloud storage providers.<br><br>The pattern currently supports 2 tokens: <code>&lt;login&gt;</code> and <code>&lt;password&gt;</code>, which are replaced with their equivalent value in the netrc file for the same host name. After formatting, the result is set as the value for the <code>Authorization</code> field of the HTTP request.<br><br>Example attribute and netrc for a http download to an oauth2 enabled API using a bearer token:<br><br><pre> auth_patterns = {     "storage.cloudprovider.com": "Bearer &lt;password&gt;" } </pre><br><br>netrc:<br><br><pre> machine storage.cloudprovider.com         password RANDOM-TOKEN </pre><br><br>The final HTTP request would have the following header:<br><br><pre> Authorization: Bearer RANDOM-TOKEN </pre>   | <a href="https://bazel.build/rules/lib/dict">Dictionary: String -> String</a> | optional |  `{}`  |
| <a id="go_repository-build_config"></a>build_config |  A file that Gazelle should read to learn about external repositories before generating build files. This is useful for dependency resolution. For example, a `go_repository` rule in this file establishes a mapping between a repository name like `golang.org/x/tools` and a workspace name like `org_golang_x_tools`. Workspace directives like `# gazelle:repository_macro` are recognized.<br><br>`go_repository` rules will be re-evaluated when parts of WORKSPACE related to Gazelle's configuration are changed, including Gazelle directives and `go_repository` `name` and `importpath` attributes. Their content should still be fetched from a local cache, but build files will be regenerated. If this is not desirable, `build_config` may be set to a less frequently updated file or `None` to disable this functionality.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `"@bazel_gazelle_go_repository_config//:WORKSPACE"`  |
| <a id="go_repository-build_directives"></a>build_directives |  A list of directives to be written to the root level build file before Calling Gazelle to generate build files. Each string in the list will be prefixed with `#` automatically. A common use case is to pass a list of Gazelle directives.   | List of strings | optional |  `[]`  |
| <a id="go_repository-build_external"></a>build_external |  One of `"external"`, `"static"` or `"vendored"`.<br><br>This sets Gazelle's `-external` command line flag. In `"static"` mode, Gazelle will not call out to the network to resolve imports.<br><br>**NOTE:** This cannot be used to ignore the `vendor` directory in a repository. The `-external` flag only controls how Gazelle resolves imports which are not present in the repository. Use `build_extra_args = ["-exclude=vendor"]` instead.   | String | optional |  `"static"`  |
| <a id="go_repository-build_extra_args"></a>build_extra_args |  A list of additional command line arguments to pass to Gazelle when generating build files.   | List of strings | optional |  `[]`  |
| <a id="go_repository-build_file_generation"></a>build_file_generation |  One of `"auto"`, `"on"`, `"off"`, `"clean"`.<br><br>Whether Gazelle should generate build files in the repository. In `"auto"` mode, Gazelle will run if there is no build file in the repository root directory. In `"clean"` mode, Gazelle will first remove any existing build files.   | String | optional |  `"auto"`  |
| <a id="go_repository-build_file_name"></a>build_file_name |  Comma-separated list of names Gazelle will consider to be build files. If a repository contains files named `build` that aren't related to Bazel, it may help to set this to `"BUILD.bazel"`, especially on case-insensitive file systems.   | String | optional |  `"BUILD.bazel,BUILD"`  |
| <a id="go_repository-build_file_proto_mode"></a>build_file_proto_mode |  One of `"default"`, `"legacy"`, `"disable"`, `"disable_global"` or `"package"`.<br><br>This sets Gazelle's `-proto` command line flag. See [Directives] for more information on each mode.   | String | optional |  `""`  |
| <a id="go_repository-build_naming_convention"></a>build_naming_convention |  Sets the library naming convention to use when resolving dependencies against this external repository. If unset, the convention from the external workspace is used. Legal values are `go_default_library`, `import`, and `import_alias`.<br><br>See the `gazelle:go_naming_convention` directive in [Directives] for more information.   | String | optional |  `"import_alias"`  |
| <a id="go_repository-build_tags"></a>build_tags |  This sets Gazelle's `-build_tags` command line flag.   | List of strings | optional |  `[]`  |
| <a id="go_repository-canonical_id"></a>canonical_id |  If the repository is downloaded via HTTP (`urls` is set) and this is set, restrict cache hits to those cases where the repository was added to the cache with the same canonical id.   | String | optional |  `""`  |
| <a id="go_repository-commit"></a>commit |  If the repository is downloaded using a version control tool, this is the commit or revision to check out. With git, this would be a sha1 commit id. `commit` and `tag` may not both be set.   | String | optional |  `""`  |
| <a id="go_repository-debug_mode"></a>debug_mode |  Enables logging of fetch_repo and Gazelle output during succcesful runs. Gazelle can be noisy so this defaults to `False`. However, setting to `True` can be useful for debugging build failures and unexpected behavior for the given rule.   | Boolean | optional |  `False`  |
| <a id="go_repository-importpath"></a>importpath |  The Go import path that matches the root directory of this repository.<br><br>In module mode (when `version` is set), this must be the module path. If neither `urls` nor `remote` is specified, `go_repository` will automatically find the true path of the module, applying import path redirection.<br><br>If build files are generated for this repository, libraries will have their `importpath` attributes prefixed with this `importpath` string.   | String | required |  |
| <a id="go_repository-internal_only_do_not_use_apparent_name"></a>internal_only_do_not_use_apparent_name |  Internal usage only   | String | optional |  `""`  |
| <a id="go_repository-local_path"></a>local_path |  If specified, `go_repository` will load the module from this local directory   | String | optional |  `""`  |
| <a id="go_repository-patch_args"></a>patch_args |  Arguments passed to the patch tool when applying patches.   | List of strings | optional |  `["-p0"]`  |
| <a id="go_repository-patch_cmds"></a>patch_cmds |  Commands to run in the repository after patches are applied.   | List of strings | optional |  `[]`  |
| <a id="go_repository-patch_tool"></a>patch_tool |  The patch tool used to apply `patches`. If this is specified, Bazel will use the specifed patch tool instead of the Bazel-native patch implementation.   | String | optional |  `""`  |
| <a id="go_repository-patches"></a>patches |  A list of patches to apply to the repository after gazelle runs.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="go_repository-remote"></a>remote |  The VCS location where the repository should be downloaded from. This is usually inferred from `importpath`, but you can set `remote` to download from a private repository or a fork.   | String | optional |  `""`  |
| <a id="go_repository-replace"></a>replace |  A replacement for the module named by `importpath`. The module named by `replace` will be downloaded at `version` and verified with `sum`.<br><br>NOTE: There is no `go_repository` equivalent to file path `replace` directives. Use `local_repository` instead.   | String | optional |  `""`  |
| <a id="go_repository-repo_mapping"></a>repo_mapping |  In `WORKSPACE` context only: a dictionary from local repository name to global repository name. This allows controls over workspace dependency resolution for dependencies of this repository.<br><br>For example, an entry `"@foo": "@bar"` declares that, for any time this repository depends on `@foo` (such as a dependency on `@foo//some:target`, it should actually resolve that dependency within globally-declared `@bar` (`@bar//some:target`).<br><br>This attribute is _not_ supported in `MODULE.bazel` context (when invoking a repository rule inside a module extension's implementation function).   | <a href="https://bazel.build/rules/lib/dict">Dictionary: String -> String</a> | optional |  |
| <a id="go_repository-sha256"></a>sha256 |  If the repository is downloaded via HTTP (`urls` is set), this is the SHA-256 sum of the downloaded archive. When set, Bazel will verify the archive against this sum before extracting it.<br><br>**CAUTION:** Do not use this with services that prepare source archives on demand, such as codeload.github.com. Any minor change in the server software can cause differences in file order, alignment, and compression that break SHA-256 sums.   | String | optional |  `""`  |
| <a id="go_repository-strip_prefix"></a>strip_prefix |  If the repository is downloaded via HTTP (`urls` is set), this is a directory prefix to strip. See [`http_archive.strip_prefix`].   | String | optional |  `""`  |
| <a id="go_repository-sum"></a>sum |  A hash of the module contents. In module mode, `go_repository` will verify the downloaded module matches this sum. May only be set when `version` is also set.<br><br>A value for `sum` may be found in the `go.sum` file or by running `go mod download -json <module>@<version>`.   | String | optional |  `""`  |
| <a id="go_repository-tag"></a>tag |  If the repository is downloaded using a version control tool, this is the named revision to check out. `commit` and `tag` may not both be set.   | String | optional |  `""`  |
| <a id="go_repository-type"></a>type |  One of `"zip"`, `"tar.gz"`, `"tgz"`, `"tar.bz2"`, `"tar.xz"`.<br><br>If the repository is downloaded via HTTP (`urls` is set), this is the file format of the repository archive. This is normally inferred from the downloaded file name.   | String | optional |  `""`  |
| <a id="go_repository-urls"></a>urls |  A list of HTTP(S) URLs where an archive containing the project can be downloaded. Bazel will attempt to download from the first URL; the others are mirrors.   | List of strings | optional |  `[]`  |
| <a id="go_repository-vcs"></a>vcs |  One of `"git"`, `"hg"`, `"svn"`, `"bzr"`.<br><br>The version control system to use. This is usually determined automatically, but it may be necessary to set this when `remote` is set and the VCS cannot be inferred. You must have the corresponding tool installed on your host.   | String | optional |  `""`  |
| <a id="go_repository-version"></a>version |  If specified, `go_repository` will download the module at this version using `go mod download`. `sum` must also be set. `commit`, `tag`, and `urls` may not be set.   | String | optional |  `""`  |


