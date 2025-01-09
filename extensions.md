<!-- Generated with Stardoc: http://skydoc.bazel.build -->



<a id="go_deps"></a>

## go_deps

<pre>
go_deps = use_extension("@bazel_gazelle//:extensions.bzl", "go_deps")
go_deps.archive_override(<a href="#go_deps.archive_override-patch_strip">patch_strip</a>, <a href="#go_deps.archive_override-patches">patches</a>, <a href="#go_deps.archive_override-path">path</a>, <a href="#go_deps.archive_override-sha256">sha256</a>, <a href="#go_deps.archive_override-strip_prefix">strip_prefix</a>, <a href="#go_deps.archive_override-urls">urls</a>)
go_deps.config(<a href="#go_deps.config-check_direct_dependencies">check_direct_dependencies</a>, <a href="#go_deps.config-debug_mode">debug_mode</a>, <a href="#go_deps.config-go_env">go_env</a>)
go_deps.from_file(<a href="#go_deps.from_file-fail_on_version_conflict">fail_on_version_conflict</a>, <a href="#go_deps.from_file-go_mod">go_mod</a>, <a href="#go_deps.from_file-go_work">go_work</a>)
go_deps.gazelle_override(<a href="#go_deps.gazelle_override-build_extra_args">build_extra_args</a>, <a href="#go_deps.gazelle_override-build_file_generation">build_file_generation</a>, <a href="#go_deps.gazelle_override-directives">directives</a>, <a href="#go_deps.gazelle_override-path">path</a>)
go_deps.gazelle_default_attributes(<a href="#go_deps.gazelle_default_attributes-build_extra_args">build_extra_args</a>, <a href="#go_deps.gazelle_default_attributes-build_file_generation">build_file_generation</a>, <a href="#go_deps.gazelle_default_attributes-directives">directives</a>)
go_deps.module(<a href="#go_deps.module-build_file_proto_mode">build_file_proto_mode</a>, <a href="#go_deps.module-build_naming_convention">build_naming_convention</a>, <a href="#go_deps.module-indirect">indirect</a>, <a href="#go_deps.module-local_path">local_path</a>, <a href="#go_deps.module-path">path</a>, <a href="#go_deps.module-sum">sum</a>,
               <a href="#go_deps.module-version">version</a>)
go_deps.module_override(<a href="#go_deps.module_override-patch_strip">patch_strip</a>, <a href="#go_deps.module_override-patches">patches</a>, <a href="#go_deps.module_override-path">path</a>)
</pre>


**TAG CLASSES**

<a id="go_deps.archive_override"></a>

### archive_override

Override the default source location on a given Go module in this extension.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="go_deps.archive_override-patch_strip"></a>patch_strip |  The number of leading path segments to be stripped from the file name in the patches.   | Integer | optional |  `0`  |
| <a id="go_deps.archive_override-patches"></a>patches |  A list of patches to apply to the repository *after* gazelle runs.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="go_deps.archive_override-path"></a>path |  The Go module path for the repository to be overridden.<br><br>This module path must be defined by other tags in this extension within this Bazel module.   | String | required |  |
| <a id="go_deps.archive_override-sha256"></a>sha256 |  If the repository is downloaded via HTTP (`urls` is set), this is the SHA-256 sum of the downloaded archive. When set, Bazel will verify the archive against this sum before extracting it.   | String | optional |  `""`  |
| <a id="go_deps.archive_override-strip_prefix"></a>strip_prefix |  If the repository is downloaded via HTTP (`urls` is set), this is a directory prefix to strip. See [`http_archive.strip_prefix`].   | String | optional |  `""`  |
| <a id="go_deps.archive_override-urls"></a>urls |  A list of HTTP(S) URLs where an archive containing the project can be downloaded. Bazel will attempt to download from the first URL; the others are mirrors.   | List of strings | optional |  `[]`  |

<a id="go_deps.config"></a>

### config

Configures the general behavior of the go_deps extension.

Only the root module's config tag is used.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="go_deps.config-check_direct_dependencies"></a>check_direct_dependencies |  The way in which warnings about version mismatches for direct dependencies and Go modules that are also Bazel modules are reported.   | String | optional |  `""`  |
| <a id="go_deps.config-debug_mode"></a>debug_mode |  Whether or not to print stdout and stderr messages from gazelle   | Boolean | optional |  `False`  |
| <a id="go_deps.config-go_env"></a>go_env |  The environment variables to use when fetching Go dependencies or running the `@rules_go//go` tool.   | <a href="https://bazel.build/rules/lib/dict">Dictionary: String -> String</a> | optional |  `{}`  |

<a id="go_deps.from_file"></a>

### from_file

Imports Go module dependencies from either a go.mod file or a go.work file.

All direct and indirect dependencies of the specified module will be imported, but only direct dependencies should
be imported into the scope of the using module via `use_repo` calls. Use `bazel mod tidy` to update these calls
automatically.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="go_deps.from_file-fail_on_version_conflict"></a>fail_on_version_conflict |  Fail if duplicate modules have different versions   | Boolean | optional |  `True`  |
| <a id="go_deps.from_file-go_mod"></a>go_mod |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |
| <a id="go_deps.from_file-go_work"></a>go_work |  -   | <a href="https://bazel.build/concepts/labels">Label</a> | optional |  `None`  |

<a id="go_deps.gazelle_override"></a>

### gazelle_override

Override Gazelle's behavior on a given Go module defined by other tags in this extension.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="go_deps.gazelle_override-build_extra_args"></a>build_extra_args |  A list of additional command line arguments to pass to Gazelle when generating build files.   | List of strings | optional |  `[]`  |
| <a id="go_deps.gazelle_override-build_file_generation"></a>build_file_generation |  One of `"auto"`, `"on"` (default), `"off"`, `"clean"`.<br><br>Whether Gazelle should generate build files for the Go module.<br><br>Although "auto" is the default globally for build_file_generation, if a `"gazelle_override"` or `"gazelle_default_attributes"` tag is present for a Go module, the `"build_file_generation"` attribute will default to "on" since these tags indicate the presence of `"directives"` or `"build_extra_args"`.<br><br>In `"auto"` mode, Gazelle will run if there is no build file in the Go module's root directory.<br><br>In `"clean"` mode, Gazelle will first remove any existing build files.   | String | optional |  `"on"`  |
| <a id="go_deps.gazelle_override-directives"></a>directives |  Gazelle configuration directives to use for this Go module's external repository.<br><br>Each directive uses the same format as those that Gazelle accepts as comments in Bazel source files, with the directive name followed by optional arguments separated by whitespace.   | List of strings | optional |  `[]`  |
| <a id="go_deps.gazelle_override-path"></a>path |  The Go module path for the repository to be overridden.<br><br>This module path must be defined by other tags in this extension within this Bazel module.   | String | required |  |

<a id="go_deps.gazelle_default_attributes"></a>

### gazelle_default_attributes

Override Gazelle's default attribute values for all modules in this extension.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="go_deps.gazelle_default_attributes-build_extra_args"></a>build_extra_args |  A list of additional command line arguments to pass to Gazelle when generating build files.   | List of strings | optional |  `[]`  |
| <a id="go_deps.gazelle_default_attributes-build_file_generation"></a>build_file_generation |  One of `"auto"`, `"on"` (default), `"off"`, `"clean"`.<br><br>Whether Gazelle should generate build files for the Go module.<br><br>Although "auto" is the default globally for build_file_generation, if a `"gazelle_override"` or `"gazelle_default_attributes"` tag is present for a Go module, the `"build_file_generation"` attribute will default to "on" since these tags indicate the presence of `"directives"` or `"build_extra_args"`.<br><br>In `"auto"` mode, Gazelle will run if there is no build file in the Go module's root directory.<br><br>In `"clean"` mode, Gazelle will first remove any existing build files.   | String | optional |  `"on"`  |
| <a id="go_deps.gazelle_default_attributes-directives"></a>directives |  Gazelle configuration directives to use for this Go module's external repository.<br><br>Each directive uses the same format as those that Gazelle accepts as comments in Bazel source files, with the directive name followed by optional arguments separated by whitespace.   | List of strings | optional |  `[]`  |

<a id="go_deps.module"></a>

### module

Declare a single Go module dependency. Prefer using `from_file` instead.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="go_deps.module-build_file_proto_mode"></a>build_file_proto_mode |  Removed, do not use   | String | optional |  `""`  |
| <a id="go_deps.module-build_naming_convention"></a>build_naming_convention |  Removed, do not use   | String | optional |  `""`  |
| <a id="go_deps.module-indirect"></a>indirect |  Whether this Go module is an indirect dependency.   | Boolean | optional |  `False`  |
| <a id="go_deps.module-local_path"></a>local_path |  For when a module is replaced by one residing in a local directory path   | String | optional |  `""`  |
| <a id="go_deps.module-path"></a>path |  The module path.   | String | required |  |
| <a id="go_deps.module-sum"></a>sum |  -   | String | optional |  `""`  |
| <a id="go_deps.module-version"></a>version |  -   | String | required |  |

<a id="go_deps.module_override"></a>

### module_override

Apply patches to a given Go module defined by other tags in this extension.

**Attributes**

| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="go_deps.module_override-patch_strip"></a>patch_strip |  The number of leading path segments to be stripped from the file name in the patches.   | Integer | optional |  `0`  |
| <a id="go_deps.module_override-patches"></a>patches |  A list of patches to apply to the repository *after* gazelle runs.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional |  `[]`  |
| <a id="go_deps.module_override-path"></a>path |  The Go module path for the repository to be overridden.<br><br>This module path must be defined by other tags in this extension within this Bazel module.   | String | required |  |


