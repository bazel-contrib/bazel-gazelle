<!-- Generated with Stardoc: http://skydoc.bazel.build -->



<a id="gazelle"></a>

## gazelle

<pre>
gazelle(<a href="#gazelle-name">name</a>, <a href="#gazelle-testonly">testonly</a>, <a href="#gazelle-kwargs">kwargs</a>)
</pre>



**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="gazelle-name"></a>name |  <p align="center"> - </p>   |  none |
| <a id="gazelle-testonly"></a>testonly |  <p align="center"> - </p>   |  `False` |
| <a id="gazelle-kwargs"></a>kwargs |  <p align="center"> - </p>   |  none |


<a id="gazelle_binary"></a>

## gazelle_binary

<pre>
gazelle_binary(<a href="#gazelle_binary-kwargs">kwargs</a>)
</pre>



**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="gazelle_binary-kwargs"></a>kwargs |  <p align="center"> - </p>   |  none |


<a id="gazelle_generation_test"></a>

## gazelle_generation_test

<pre>
gazelle_generation_test(<a href="#gazelle_generation_test-name">name</a>, <a href="#gazelle_generation_test-gazelle_binary">gazelle_binary</a>, <a href="#gazelle_generation_test-test_data">test_data</a>, <a href="#gazelle_generation_test-build_in_suffix">build_in_suffix</a>, <a href="#gazelle_generation_test-build_out_suffix">build_out_suffix</a>,
                        <a href="#gazelle_generation_test-gazelle_timeout_seconds">gazelle_timeout_seconds</a>, <a href="#gazelle_generation_test-size">size</a>, <a href="#gazelle_generation_test-kwargs">kwargs</a>)
</pre>

gazelle_generation_test is a macro for testing gazelle against workspaces.

The generation test expects a file structure like the following:

```
|-- <testDataPath>
    |-- some_test
        |-- WORKSPACE and/or MODULE.bazel -> Indicates the directory is a test case.
        |-- README.md --> README describing what the test does.
        |-- arguments.txt --> newline delimited list of arguments to pass in (ignored if empty).
        |-- expectedStdout.txt --> Expected stdout for this test.
        |-- expectedStderr.txt --> Expected stderr for this test.
        |-- expectedExitCode.txt --> Expected exit code for this test.
        |-- app
            |-- sourceFile.foo
            |-- BUILD.in --> BUILD file prior to running gazelle.
            |-- BUILD.out --> BUILD file expected after running gazelle.
```

To update the expected files, run `UPDATE_SNAPSHOTS=true bazel run //path/to:the_test_target`.


**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="gazelle_generation_test-name"></a>name |  The name of the test.   |  none |
| <a id="gazelle_generation_test-gazelle_binary"></a>gazelle_binary |  The name of the gazelle binary target. For example, //path/to:my_gazelle.   |  none |
| <a id="gazelle_generation_test-test_data"></a>test_data |  A list of target of the test data files you will pass to the test. This can be a https://bazel.build/reference/be/general#filegroup.   |  none |
| <a id="gazelle_generation_test-build_in_suffix"></a>build_in_suffix |  The suffix for the input BUILD.bazel files. Defaults to .in. By default, will use files named BUILD.in as the BUILD files before running gazelle.   |  `".in"` |
| <a id="gazelle_generation_test-build_out_suffix"></a>build_out_suffix |  The suffix for the expected BUILD.bazel files after running gazelle. Defaults to .out. By default, will use files named check the results of the gazelle run against files named BUILD.out.   |  `".out"` |
| <a id="gazelle_generation_test-gazelle_timeout_seconds"></a>gazelle_timeout_seconds |  Number of seconds to allow the gazelle process to run before killing.   |  `2` |
| <a id="gazelle_generation_test-size"></a>size |  Specifies a test target's "heaviness": how much time/resources it needs to run.   |  `None` |
| <a id="gazelle_generation_test-kwargs"></a>kwargs |  Attributes that are passed directly to the test declaration.   |  none |


<a id="gazelle_test"></a>

## gazelle_test

<pre>
gazelle_test(<a href="#gazelle_test-name">name</a>, <a href="#gazelle_test-size">size</a>, <a href="#gazelle_test-timeout">timeout</a>, <a href="#gazelle_test-kwargs">kwargs</a>)
</pre>



**PARAMETERS**


| Name  | Description | Default Value |
| :------------- | :------------- | :------------- |
| <a id="gazelle_test-name"></a>name |  <p align="center"> - </p>   |  none |
| <a id="gazelle_test-size"></a>size |  <p align="center"> - </p>   |  `None` |
| <a id="gazelle_test-timeout"></a>timeout |  <p align="center"> - </p>   |  `None` |
| <a id="gazelle_test-kwargs"></a>kwargs |  <p align="center"> - </p>   |  none |


<a id="git_repository"></a>

## git_repository

<pre>
git_repository(<a href="#git_repository-name">name</a>, <a href="#git_repository-commit">commit</a>, <a href="#git_repository-overlay">overlay</a>, <a href="#git_repository-remote">remote</a>, <a href="#git_repository-repo_mapping">repo_mapping</a>, <a href="#git_repository-tag">tag</a>)
</pre>

**NOTE:** `git_repository` is deprecated in favor of the rule of the same name
in [@bazel_tools//tools/build_defs/repo:git.bzl].

`git_repository` downloads a project with git. It has the same features as the
[native git_repository rule], but it also allows you to copy a set of files
into the repository after download. This is particularly useful for placing
pre-generated build files.

**Example**

```starlark
load("@bazel_gazelle//:deps.bzl", "git_repository")

git_repository(
    name = "com_github_pkg_errors",
    remote = "https://github.com/pkg/errors",
    commit = "816c9085562cd7ee03e7f8188a1cfd942858cded",
    overlay = {
        "@my_repo//third_party:com_github_pkg_errors/BUILD.bazel.in" : "BUILD.bazel",
    },
)
```

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="git_repository-name"></a>name |  A unique name for this repository.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="git_repository-commit"></a>commit |  The git commit to check out. Either `commit` or `tag` may be specified.   | String | optional |  `""`  |
| <a id="git_repository-overlay"></a>overlay |  A set of files to copy into the downloaded repository. The keys in this dictionary are Bazel labels that point to the files to copy. These must be fully qualified labels (i.e., `@repo//pkg:name`) because relative labels are interpreted in the checked out repository, not the repository containing the WORKSPACE file. The values in this dictionary are root-relative paths where the overlay files should be written.<br><br>It's convenient to store the overlay dictionaries for all repositories in a separate .bzl file. See Gazelle's `manifest.bzl`_ for an example.   | <a href="https://bazel.build/rules/lib/dict">Dictionary: Label -> String</a> | optional |  `{}`  |
| <a id="git_repository-remote"></a>remote |  The remote repository to download.   | String | required |  |
| <a id="git_repository-repo_mapping"></a>repo_mapping |  In `WORKSPACE` context only: a dictionary from local repository name to global repository name. This allows controls over workspace dependency resolution for dependencies of this repository.<br><br>For example, an entry `"@foo": "@bar"` declares that, for any time this repository depends on `@foo` (such as a dependency on `@foo//some:target`, it should actually resolve that dependency within globally-declared `@bar` (`@bar//some:target`).<br><br>This attribute is _not_ supported in `MODULE.bazel` context (when invoking a repository rule inside a module extension's implementation function).   | <a href="https://bazel.build/rules/lib/dict">Dictionary: String -> String</a> | optional |  |
| <a id="git_repository-tag"></a>tag |  The git tag to check out. Either `commit` or `tag` may be specified.   | String | optional |  `""`  |


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


<a id="http_archive"></a>

## http_archive

<pre>
http_archive(<a href="#http_archive-name">name</a>, <a href="#http_archive-overlay">overlay</a>, <a href="#http_archive-repo_mapping">repo_mapping</a>, <a href="#http_archive-sha256">sha256</a>, <a href="#http_archive-strip_prefix">strip_prefix</a>, <a href="#http_archive-type">type</a>, <a href="#http_archive-urls">urls</a>)
</pre>

**NOTE:** `http_archive` is deprecated in favor of the rule of the same name
in [@bazel_tools//tools/build_defs/repo:http.bzl].

`http_archive` downloads a project over HTTP(S). It has the same features as
the [native http_archive rule], but it also allows you to copy a set of files
into the repository after download. This is particularly useful for placing
pre-generated build files.

**Example**

```starlark
load("@bazel_gazelle//:deps.bzl", "http_archive")

http_archive(
    name = "com_github_pkg_errors",
    urls = ["https://codeload.github.com/pkg/errors/zip/816c9085562cd7ee03e7f8188a1cfd942858cded"],
    strip_prefix = "errors-816c9085562cd7ee03e7f8188a1cfd942858cded",
    type = "zip",
    overlay = {
        "@my_repo//third_party:com_github_pkg_errors/BUILD.bazel.in" : "BUILD.bazel",
    },
)
```

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="http_archive-name"></a>name |  A unique name for this repository.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="http_archive-overlay"></a>overlay |  A set of files to copy into the downloaded repository. The keys in this dictionary are Bazel labels that point to the files to copy. These must be fully qualified labels (i.e., `@repo//pkg:name`) because relative labels are interpreted in the checked out repository, not the repository containing the WORKSPACE file. The values in this dictionary are root-relative paths where the overlay files should be written.<br><br>It's convenient to store the overlay dictionaries for all repositories in a separate .bzl file. See Gazelle's `manifest.bzl`_ for an example.   | <a href="https://bazel.build/rules/lib/dict">Dictionary: Label -> String</a> | optional |  `{}`  |
| <a id="http_archive-repo_mapping"></a>repo_mapping |  In `WORKSPACE` context only: a dictionary from local repository name to global repository name. This allows controls over workspace dependency resolution for dependencies of this repository.<br><br>For example, an entry `"@foo": "@bar"` declares that, for any time this repository depends on `@foo` (such as a dependency on `@foo//some:target`, it should actually resolve that dependency within globally-declared `@bar` (`@bar//some:target`).<br><br>This attribute is _not_ supported in `MODULE.bazel` context (when invoking a repository rule inside a module extension's implementation function).   | <a href="https://bazel.build/rules/lib/dict">Dictionary: String -> String</a> | optional |  |
| <a id="http_archive-sha256"></a>sha256 |  The SHA-256 sum of the downloaded archive. When set, Bazel will verify the archive against this sum before extracting it.<br><br>**CAUTION:** Do not use this with services that prepare source archives on demand, such as codeload.github.com. Any minor change in the server software can cause differences in file order, alignment, and compression that break SHA-256 sums.   | String | optional |  `""`  |
| <a id="http_archive-strip_prefix"></a>strip_prefix |  A directory prefix to strip. See [http_archive.strip_prefix].   | String | optional |  `""`  |
| <a id="http_archive-type"></a>type |  One of `"zip"`, `"tar.gz"`, `"tgz"`, `"tar.bz2"`, `"tar.xz"`.<br><br>The file format of the repository archive. This is normally inferred from the downloaded file name.   | String | optional |  `""`  |
| <a id="http_archive-urls"></a>urls |  A list of HTTP(S) URLs where the project can be downloaded. Bazel will attempt to download the first URL; the others are mirrors.   | List of strings | optional |  `[]`  |


