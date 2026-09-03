# Copyright 2023 The Bazel Authors. All rights reserved.
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

load("@bazel_skylib//lib:paths.bzl", "paths")
load("//internal:common.bzl", "env_execute", "executable_extension", "path_str", "watch")
load("//internal:env.bzl", "read_go_env_file", "resolve_env")
load("//internal:go_repository.bzl", "go_repository")
load(
    ":default_gazelle_overrides.bzl",
    "DEFAULT_BUILD_EXTRA_ARGS_BY_PATH",
    "DEFAULT_BUILD_FILE_GENERATION_BY_PATH",
    "DEFAULT_DIRECTIVES_BY_PATH",
)
load(":go_repository_config.bzl", "go_repository_config")
load(":semver.bzl", "semver")
load(":utils.bzl", "drop_nones", "extension_metadata", "get_directive_value")

visibility([
    "//",
    "//tests/bzlmod/...",
])

_SHARED_REPOS = [
    "github.com/golang/protobuf",
    "google.golang.org/protobuf",
]

def go_deps_impl(module_ctx):
    module_ctx = _wrap_module_ctx_for_testability(module_ctx)

    # Identify the root module and process config and overrides.
    # If this is an isolated go_deps instance, there will be only one module,
    # and we treat it as the root, even if it's not.
    module_ctx.report_progress("gathering version requirements")
    root_module = None
    config_tag = None
    archive_overrides = {}  # Go module path => archive_override tag
    module_overrides = {}  # Go module path => module_override tag
    gazelle_overrides = {}  # Go module path => gazelle_override tag
    gazelle_default_attributes = None
    for module in module_ctx.modules:
        if _module_acts_as_root(module_ctx, module):
            root_module = module
            if len(module.tags.config) > 1:
                module_ctx.fail("Multiple go_deps.config tags defined in module {}".format(module.name))
                return None
            if len(module.tags.config) == 1:
                config_tag = module.tags.config[0]
            if len(module.tags.gazelle_default_attributes) > 1:
                module_ctx.fail("Multiple go_deps.gazelle_default_attributes tags defined in module {}".format(module.name))
                return None
            if len(module.tags.gazelle_default_attributes) == 1:
                gazelle_default_attributes = module.tags.gazelle_default_attributes[0]

        _process_overrides(module_ctx, module, "archive_override", archive_overrides)
        _process_overrides(module_ctx, module, "module_override", module_overrides, archive_overrides)
        _process_overrides(module_ctx, module, "gazelle_override", gazelle_overrides)
    if module_ctx.failed():
        return None
    if not root_module:
        module_ctx.fail("root module not found")
        return None

    # Compute the environment based on the config tag and available go_sdks.
    # Use this to locate the go tool.
    go_env = read_go_env_file(
        module_ctx,
        env_path = Label("@bazel_gazelle_go_repository_cache//:go.env"),
    )
    if config_tag:
        go_env |= resolve_env(
            module_ctx,
            direct = config_tag.go_env,
            inherit = config_tag.go_env_inherit,
            reserved = [
                "GOCACHE",
                "GOMODCACHE",
                "GOPATH",
                "GOROOT",
                "GOROOT_LABEL",
            ],
        )
    go_tool = go_env["GOROOT"] + "/bin/go" + executable_extension(module_ctx)
    watch(module_ctx, go_tool)

    # Create a scratch Go workspace (with a synthetic go.work and go.mod file)
    # expressing constraints from go_deps tags, linking with go.mod files
    # provided by go_deps.from_file.
    workspace = _create_workspace_from_tags(module_ctx, go_tool, go_env)
    if module_ctx.failed() or workspace == None:
        return None
    bazel_go_modules, root_required_mods = workspace

    # Run 'go list -m' in the scratch workspace to select versions of Go modules.
    module_ctx.report_progress("selecting versions with 'go list -m'")
    go_modules = _select_module_versions(
        module_ctx,
        go_tool,
        go_env,
        bazel_go_modules,
        root_required_mods,
        module_overrides,
    )
    if module_ctx.failed():
        return None

    module_ctx.report_progress("declaring repositories")
    reserved_repo_names = _collect_reserved_repo_names(module_ctx, bazel_go_modules)
    _check_for_version_conflict(
        module_ctx,
        go_modules,
        bazel_go_modules,
        root_module.tags.module,
        root_required_mods,
        _get_checks_reporter(module_ctx, root_module),
        reserved_repo_names,
    )
    if module_ctx.failed():
        return None

    _fail_on_unmatched_overrides(module_ctx, archive_overrides.keys(), go_modules, "archive_override")
    _fail_on_unmatched_overrides(module_ctx, module_overrides.keys(), go_modules, "module_override")
    _fail_on_unmatched_overrides(module_ctx, gazelle_overrides.keys(), go_modules, "gazelle_override")
    if module_ctx.failed():
        return None

    # Build a map of tool targets, and track which modules are direct dependencies
    # of the main Bazel module.
    tool_targets, direct_deps, direct_dev_deps = _index_tool_targets(
        module_ctx,
        bazel_go_modules,
        root_required_mods,
        module_overrides,
    )
    if module_ctx.failed():
        return None

    # Declare a go_repository for each selected Go module that is not provided
    # by a Bazel module. Repo name collisions were checked in
    # _check_for_version_conflict.
    for go_module in go_modules.values():
        if not _should_declare_go_repository(module_ctx, go_module):
            continue
        go_repository_args = {
            "name": go_module.repo_name,
            "importpath": go_module.importpath,
            "debug_mode": config_tag != None and config_tag.debug_mode,
            "internal_only_do_not_use_apparent_name": go_module.repo_name,
        }
        archive_override = archive_overrides.get(go_module.importpath)
        if archive_override:
            go_repository_args.update({
                "urls": archive_override.urls,
                "strip_prefix": archive_override.strip_prefix,
                "sha256": archive_override.sha256,
                "patches": archive_override.patches,
                "patch_args": _get_patch_args(archive_override),
                "patch_cmds": archive_override.patch_cmds,
            })
        else:
            go_repository_args.update({
                "version": go_module.version,
                "sum": go_module.sum,
                "replace": go_module.replace_path,
                "local_path": go_module.local_path,
            })
        module_override = module_overrides.get(go_module.importpath)
        if module_override:
            go_repository_args.update({
                "patches": module_override.patches,
                "patch_args": _get_patch_args(module_override),
                "patch_cmds": module_override.patch_cmds,
            })
        gazelle_attrs = _gazelle_attributes_for_module(
            go_module.importpath,
            gazelle_overrides,
            gazelle_default_attributes,
        )
        go_repository_args.update({
            "build_directives": gazelle_attrs.directives,
            "build_file_generation": gazelle_attrs.build_file_generation,
            "build_extra_args": gazelle_attrs.build_extra_args,
        })
        module_ctx.declare_repo(go_repository, **go_repository_args)

    if "rules_proto" not in reserved_repo_names:
        # The BUILD files of Go modules may still load proto rules from
        # @rules_proto, which Gazelle no longer depends on. Generate a
        # compatibility shim with that name that forwards the loads to
        # @com_google_protobuf. Since this repo is generated by the go_deps
        # extension, it is visible to the Go module repos generated by the same
        # extension under exactly this name. See
        # https://github.com/bazel-contrib/bazel-gazelle/issues/2358.
        module_ctx.declare_repo(_rules_proto_compat, name = "rules_proto")

    # Declare a repo with configuration files for Gazelle and rules_go.
    # Gazelle especially needs instructions for resolving cross-repo imports
    # from within each go_repository.
    # TODO(#2413): refactor go_repository_config. For now, repo names of Go
    # modules provided by Bazel modules must be canonical and start with "@".
    # Also, include entries for modules not in root directories.
    def _repo_name_for_config(info):
        return "@" + info.repo_name if info.go_mod_label else info.repo_name

    module_ctx.declare_repo(
        go_repository_config,
        name = "bazel_gazelle_go_repository_config",
        repo_names = {
            m.importpath: _repo_name_for_config(m)
            for m in go_modules.values()
        },
        module_names = {
            _repo_name_for_config(info): info.bazel_dep_name
            for info in bazel_go_modules.values()
        },
        prefix_dirs = {
            info.importpath: info.go_mod_label.package
            for info in bazel_go_modules.values()
            if info.go_mod_label.package
        },
        tool_targets = tool_targets,
        build_naming_conventions = drop_nones({
            go_modules[path].repo_name: get_directive_value(
                _gazelle_attributes_for_module(
                    path,
                    gazelle_overrides,
                    gazelle_default_attributes,
                ).directives,
                "go_naming_convention",
            )
            for path in go_modules
        }),
        go_env = go_env,
        dep_files = sorted([
            info.go_mod_label.package + "/" + info.go_mod_label.name if info.go_mod_label.package else info.go_mod_label.name
            for info in bazel_go_modules.values()
            if info.is_root
        ]),
    )

    # Return metadata indicating this extension is reproducible so that no
    # MODULE.bazel.lock is needed, and list the direct dependencies so that
    # 'bazel mod tidy' can update use_repo.
    # Only include bazel_gazelle_go_repository_config in direct dependencies for
    # gazelle and rules_go; it shouldn't be generally available.
    # Don't include common repo names if this is an isolated go_deps instance.
    if (root_module.name in ("gazelle", "rules_go", "gazelle_bcr_go_mod_tests", "gazelle_bcr_go_work_tests") and
        not getattr(module_ctx, "is_isolated", False)):
        direct_deps.append("bazel_gazelle_go_repository_config")
    if getattr(module_ctx, "is_isolated", False):
        shared_repo_names = [
            _get_repo_name(path, bazel_go_modules, module_overrides)
            for path in _SHARED_REPOS
        ]
        direct_deps = [dep for dep in direct_deps if dep not in shared_repo_names]
        direct_dev_deps = [dep for dep in direct_dev_deps if dep not in shared_repo_names]

    return extension_metadata(
        module_ctx,
        root_module_direct_deps = direct_deps,
        root_module_direct_dev_deps = direct_dev_deps,
        reproducible = True,
    )

def _go_module_info(
        *,
        importpath,
        repo_name,
        version = None,
        sum = None,
        replace_path = None,
        local_path = None,
        go_mod_label = None):
    """
    Tracks information about a resolved Go module

    Args:
        importpath: the Go module path
        repo_name: name to use when generating go_repository declarations or
            labels that point to a repo containing Go code. If the Go module is
            provided by a Bazel module, this comes from
            _bazel_go_mod_info.repo_name.
        version: the selected version, including the 'v' prefix. For replaced
            modules, this is the replacement version. May be omitted
            for replaced modules with directory replacements or path overrides.
        sum: the cryptographic sum from go.sum. For replaced modules, this is
            the sum of the replacement. May be omitted for replaced modules
            with directory replacements or various overrides. Also omitted
            for modules that are in the build list but are not needed to build
            packages in the root modules (no sum in any go.sum).
        replace_path: Go module path of a versioned replacement. For example,
            in 'replace example.com/a => example.com/b v1.0.0', this is
            'example.com/b'.
        local_path: directory path of a directory replacement. Can come from
            a replace directive or module tag.
        go_mod_label: Label for the go.mod file, if provided by a Bazel module.

    Returns:
        A _go_module_info struct containing the arguments as fields.
    """
    return struct(
        importpath = importpath,
        repo_name = repo_name,
        version = version,
        sum = sum,
        replace_path = replace_path,
        local_path = local_path,
        go_mod_label = go_mod_label,
    )

def _bazel_go_mod_info(
        *,
        importpath,
        go_mod_label,
        repo_name,
        bazel_dep_name,
        bazel_dep_version,
        is_root,
        tool_importpaths):
    """
    Tracks information about a Go module provided by a Bazel module

    Args:
        importpath: the Go module path from the go.mod file
        go_mod_label: label for the module's go.mod file
        repo_name: name used to generate labels pointing to this module's repo.
            For the root module, this is the module name. For other modules,
            this is the canonical name (like "gazelle+").
        bazel_dep_name: the name of the Bazel module (not the apparent name).
        bazel_dep_version: the version of the Bazel module (lacking a 'v'
            prefix, like Go modules have).
        is_root: True for the root module OR for a module loading an isolated
            instance of go_deps.
        tool_importpaths: list of package paths from 'tool' directives in
            a go.mod file in the root Bazel module or isolate. Empty for
            modules where is_root is False.

    Returns:
        a _bazel_go_mod_struct containing the arguments as fields
    """
    return struct(
        importpath = importpath,
        go_mod_label = go_mod_label,
        repo_name = repo_name,
        bazel_dep_name = bazel_dep_name,
        bazel_dep_version = bazel_dep_version,
        is_root = is_root,
        tool_importpaths = tool_importpaths,
    )

def _go_require_info(
        *,
        importpath,
        version,
        sum = None,
        local_path = None,
        indirect = False,
        is_dev_dependency = False):
    """Tracks a version constraint, from a go.mod require directive or go_deps.module tag"""
    return struct(
        importpath = importpath,
        version = version,
        sum = sum,
        local_path = local_path,
        indirect = indirect,
        is_dev_dependency = is_dev_dependency,
    )

def _repo_name(importpath):
    path_segments = importpath.split("/")
    segments = reversed(path_segments[0].split(".")) + path_segments[1:]
    candidate_name = "_".join(segments).replace("-", "_")
    return "".join([c.lower() if c.isalnum() else "_" for c in candidate_name.elems()])

def _bazel_module_repo_name(module, go_mod_label):
    return go_mod_label.repo_name if go_mod_label.repo_name else module.name

def _get_repo_name(importpath, bazel_go_modules, module_overrides):
    """Returns the Bazel repo name for a Go module path.

    If a Go module is provided by a Bazel module (with go_deps.from_file),
    the repo name is taken from the go_mod label on that tag (best effort).
    Gazelle resolves the apparent name from module_name when loading repo config.

    If a module_override for the path specifies a non-empty repo_name, that
    value is used verbatim. Otherwise the name is derived from the import path
    via _repo_name. This allows the root module to break collisions between two
    modules whose import paths mangle to the same default repo name.

    Args:
        importpath: the Go module path
        bazel_go_modules: a dict mapping Go module path to _bazel_go_mod_info
            struct.
        module_overrides: map from Go module paths to go_deps.module_override tag.

    Returns:
        The repo name to use.
    """
    bazel_go_mod = bazel_go_modules.get(importpath)
    if bazel_go_mod:
        return bazel_go_mod.repo_name
    override = module_overrides.get(importpath)
    if override and getattr(override, "repo_name", ""):
        return override.repo_name
    return _repo_name(importpath)

def _module_acts_as_root(module_ctx, module):
    """
    Returns whether this is the Bazel root module or an isolated load of go_deps.

    Some functionality, like go.mod replace directives, is only available in
    a root or isolated module.
    """
    return module.is_root or getattr(module_ctx, "is_isolated", False)

def _should_declare_go_repository(module_ctx, go_module):
    """
    Controls whether we declare go_repository

    In particular, we skip declarations for Go modules provided by Bazel modules.
    Isolated go_deps instances also skip shared repos declared by the non-isolated
    instance.
    """
    if getattr(module_ctx, "is_isolated", False) and go_module.importpath in _SHARED_REPOS:
        return False
    return go_module.go_mod_label == None and (
        go_module.version != None or
        go_module.local_path != None
    )

def _get_checks_reporter(module_ctx, root_module):
    """Returns a function for reporting problems, depending on the error level"""
    OFF, WARNING, ERROR = 0, 1, 2
    LEVEL = {
        "off": OFF,
        "warning": WARNING,
        "error": ERROR,
    }
    check_direct_dependencies_level = OFF
    if len(root_module.tags.config) > 0:
        config_tag = root_module.tags.config[0]
        checks_level = LEVEL[config_tag.checks]
        check_direct_dependencies_level = LEVEL[config_tag.check_direct_dependencies]
    else:
        checks_level = WARNING
        check_direct_dependencies_level = OFF
    from_file_level = OFF
    for tag in root_module.tags.from_file:
        if tag.fail_on_version_conflict:
            from_file_level = ERROR
    level = max(checks_level, check_direct_dependencies_level, from_file_level)
    if level == OFF:
        return lambda *args: None
    elif level == WARNING:
        return module_ctx.print
    else:
        return module_ctx.fail

def _process_overrides(module_ctx, module, override_type, overrides, additional_overrides = None):
    """
    Processes a given override type for a given module

    Checks for duplicates and conflicts, then inserts the override into the given map.

    Args:
        module_ctx: the module context
        module: the module containing overrides
        override_type: the tag name, for error messages
        overrides: a dict mapping Go module paths to tags
        additional_overrides: another dict of override tags that may conflict
    """
    _fail_on_non_root_overrides(module_ctx, module, override_type)
    for override_tag in getattr(module.tags, override_type):
        _fail_on_duplicate_overrides(module_ctx, override_tag.path, module.name, overrides)

        # Some overrides conflict with other overrides. These can be specified in the
        # additional_overrides dict. If the override is in the additional_overrides dict, then fail.
        if additional_overrides:
            _fail_on_duplicate_overrides(module_ctx, override_tag.path, module.name, additional_overrides)

        overrides[override_tag.path] = override_tag

def _gazelle_attributes_for_module(importpath, gazelle_overrides, gazelle_default_attributes):
    """Returns Gazelle settings for a Go module path.

    Priority, lowest to highest:
    1. go_repository defaults (build_file_generation = "auto", empty lists)
    2. default_gazelle_overrides.bzl path entries
    3. go_deps.gazelle_default_attributes tag
    4. go_deps.gazelle_override tag for this path

    A gazelle_override tag always sets build_file_generation, even when left at
    its tag default of "on".
    """
    directives = DEFAULT_DIRECTIVES_BY_PATH.get(importpath, [])
    build_file_generation = DEFAULT_BUILD_FILE_GENERATION_BY_PATH.get(importpath, "auto")
    build_extra_args = DEFAULT_BUILD_EXTRA_ARGS_BY_PATH.get(importpath, [])

    if gazelle_default_attributes:
        if gazelle_default_attributes.directives:
            directives = gazelle_default_attributes.directives
        if gazelle_default_attributes.build_extra_args:
            build_extra_args = gazelle_default_attributes.build_extra_args
        if gazelle_default_attributes.build_file_generation:
            build_file_generation = gazelle_default_attributes.build_file_generation

    per_module = gazelle_overrides.get(importpath)
    if per_module:
        if per_module.directives != _GAZELLE_DEFAULT_ATTRIBUTES.directives:
            directives = per_module.directives
        if per_module.build_extra_args != _GAZELLE_DEFAULT_ATTRIBUTES.build_extra_args:
            build_extra_args = per_module.build_extra_args
        build_file_generation = per_module.build_file_generation

    return struct(
        directives = directives,
        build_file_generation = build_file_generation,
        build_extra_args = build_extra_args,
    )

def _fail_on_non_root_overrides(module_ctx, module, tag_class):
    if _module_acts_as_root(module_ctx, module):
        return

    if getattr(module.tags, tag_class):
        module_ctx.fail("""\
Using the "go_deps.{tag_class}" tag in a non-root Bazel module is forbidden, \
but module "{module_name}" requests it.

If you need this override for a Bazel module that will be available in a public \
registry (such as the Bazel Central Registry), please file an issue at \
https://github.com/bazelbuild/bazel-gazelle/issues/new or submit a PR adding \
the required directives to the "default_gazelle_overrides.bzl" file at \
https://github.com/bazelbuild/bazel-gazelle/tree/master/internal/bzlmod/default_gazelle_overrides.bzl.
""".format(
            tag_class = tag_class,
            module_name = module.name,
        ))

def _fail_on_duplicate_overrides(module_ctx, path, module_name, overrides):
    if path in overrides:
        module_ctx.fail("Multiple overrides defined for Go module path \"{}\" in module \"{}\".".format(path, module_name))

def _fail_on_unmatched_overrides(module_ctx, override_keys, resolutions, override_name):
    unmatched_overrides = [path for path in override_keys if path not in resolutions]
    if unmatched_overrides:
        module_ctx.fail("Some {} did not target a Go module with a matching path: {}".format(
            override_name,
            ", ".join(unmatched_overrides),
        ))

def _get_patch_args(archive_override):
    return ["-p{}".format(archive_override.patch_strip)] if archive_override.patch_strip else []

def _local_replace_path(dir_path):
    """Formats a workspace directory path for a go.mod replace directive."""
    if dir_path.startswith("./") or dir_path.startswith("../") or dir_path.startswith("/"):
        return dir_path
    return "./" + dir_path

def _create_workspace_from_tags(module_ctx, go_tool, go_env):
    """
    Create a scratch go.work workspace based on go_deps tags.

    This expresses constraints so that we can use 'go list -m' for version
    selection. This is for version selection only; we apply archive_override
    and other tags later.

    - For each 'from_file(go_mod = ...)' tag, we reference the module with a
      'use' directive. If the tag is outside the root Bazel module, we make
      a copy of the go.mod file first, dropping 'replace' and 'exclude'
      directives.
    - For each 'from_file(go_work = ...)' tag, we parse the file and copy
      its 'use' directives, normalizing paths as needed. We only copy 'replace'
      directives if the tag is from the root Bazel module.
    - For each 'module' tag, we add a 'require' directive to a dummy go.mod
      file, referenced from our go.work file with 'use .'. If the required
      module is also provided by a Bazel module (via from_file), we add a
      matching 'replace' directive pointing at that module's workspace
      directory so 'go list -m' can resolve the version.

    Args:
        module_ctx: the module context.
        go_tool: path to the go tool, needed for 'go mod edit -json',
            'go work edit -json'.
        go_env: environment to run the go tool with.

    Returns:
        - bazel_go_modules: a dict mapping Go module path to _bazel_go_mod_info
          struct.
        - root_required_mods: a dict mapping Go module path to _go_require_info
          struct for Go modules required by the root Bazel module, either via
          go_deps.module or go_deps.from_file with go.mod.
    """

    result = env_execute(module_ctx, [go_tool, "version"], go_env)
    if result.return_code != 0:
        module_ctx.fail("determining go version: {}".format(result.stderr))
        return None

    # example output: go version go1.27rc3 darwin/arm64
    go_version_str = result.stdout.split(" ")[2][len("go"):]
    go_version = _parse_go_version(go_version_str)

    go_work_lines = [
        "go {}".format(go_version_str),
        "use .",
    ]
    go_mod_lines = [
        "module go_deps_module_tags",
        "go {}".format(go_version_str),
    ]
    go_sum_lines = []
    go_work_sum_lines = []

    bazel_go_modules = {}
    bazel_go_module_dirs = {}  # Go module path => directory in synthetic workspace
    root_required_mods = {}
    module_tag_requires = {}  # Go module path => go_deps.module tag with highest version
    for module in module_ctx.modules:
        module_tag_paths = {}
        for tag in module.tags.module:
            if tag.path in module_tag_paths:
                module_ctx.fail("Multiple go_deps.module tags defined for Go module path \"{}\" in module \"{}\".".format(tag.path, module.name))
                return None
            module_tag_paths[tag.path] = True
            if not _module_acts_as_root(module_ctx, module) and tag.local_path:
                module_ctx.fail("{}: go_deps.module.local_path is only allowed in the root Bazel module".format(tag.path))
                return None
            if not tag.sum and not tag.local_path:
                module_ctx.fail("{}: go_deps.module.sum must be set unless local_path is set".format(tag.path))
                return None
            existing = module_tag_requires.get(tag.path)
            if existing == None or semver.to_comparable(_normalize_version(tag.version)) > semver.to_comparable(_normalize_version(existing.version)):
                module_tag_requires[tag.path] = tag
            if _module_acts_as_root(module_ctx, module):
                root_required_mods[tag.path] = _go_require_info(
                    importpath = tag.path,
                    version = tag.version,
                    sum = tag.sum,
                    local_path = tag.local_path,
                    indirect = tag.indirect,
                    is_dev_dependency = module_ctx.is_dev_dependency(tag),
                )

        def visit_go_mod(go_mod_label):
            go_mod_path = module_ctx.path(go_mod_label)
            watch(module_ctx, go_mod_path)
            go_sum_path = go_mod_path.dirname.get_child("go.sum")
            if go_sum_path.exists:
                watch(module_ctx, go_sum_path)
            go_mod_json = _parse_go_mod_json(module_ctx, go_tool, go_env, go_mod_path)
            if module_ctx.failed():
                return
            go_mod_version = _parse_go_version(go_mod_json["Go"])
            if go_mod_version > go_version:
                module_ctx.fail("""\
In {go_mod_label}, go version is {go_mod_version}, but Bazel is using {go_version}.
To correct this:
    1. Upgrade the Bazel Go version in MODULE.bazel:

        go_sdk = use_extension("@rules_go//go:extensions.bzl", "go_sdk")
        go_sdk.download("{go_mod_version}")

    2. Or downgrade the Go module version to {go_version}.
""".format(
                    go_version = go_version_str,
                    go_mod_label = go_mod_label,
                    go_mod_version = go_mod_json["Go"],
                ))
                return
            if _module_acts_as_root(module_ctx, module):
                tool_importpaths = [tool["Path"] for tool in go_mod_json.get("Tool") or []]
            else:
                tool_importpaths = []
            acts_as_root = _module_acts_as_root(module_ctx, module)
            info = _bazel_go_mod_info(
                importpath = go_mod_json["Module"]["Path"],
                go_mod_label = go_mod_label,
                repo_name = _bazel_module_repo_name(module, go_mod_label),
                bazel_dep_name = module.name,
                bazel_dep_version = module.version,
                is_root = acts_as_root,
                tool_importpaths = tool_importpaths,
            )
            bazel_go_modules[info.importpath] = info

            if acts_as_root:
                # We can use 'replace' and 'exclude' directives in go.mod files from
                # the Bazel root module without modification.
                bazel_go_module_dirs[info.importpath] = path_str(go_mod_path.dirname)
                go_work_lines.append("use {}".format(path_str(go_mod_path.dirname)))
                for r in go_mod_json.get("Require") or []:
                    # A module may be required multiple times from different go.mod
                    # files within a go.work workspace, so update the existing entry
                    # if needed.
                    if r["Path"] in root_required_mods:
                        prev = root_required_mods[r["Path"]]
                        root_required_mods[r["Path"]] = _go_require_info(
                            importpath = r["Path"],
                            version = semver.max(prev.version, r["Version"]),
                            indirect = prev.indirect and bool(r.get("Indirect")),
                        )
                    else:
                        root_required_mods[r["Path"]] = _go_require_info(
                            importpath = r["Path"],
                            version = r["Version"],
                            indirect = bool(r.get("Indirect")),
                        )
            else:
                # We want to ignore 'replace' and 'exclude' directives from go.mod
                # files outside the root module. We need to parse, modify, and copy
                # the file.
                go_mod_json["Replace"] = None
                go_mod_json["Exclude"] = None
                copied_go_mod_path = module_ctx.path(paths.join("mod", module.name, _label_to_rel(go_mod_label)))
                module_ctx.file(copied_go_mod_path, _format_go_mod_json(go_mod_json))
                if go_sum_path.exists:
                    go_sum_content = module_ctx.read(go_sum_path)
                    copied_go_sum_path = copied_go_mod_path.dirname.get_child("go.sum")
                    module_ctx.file(copied_go_sum_path, go_sum_content)
                bazel_go_module_dirs[info.importpath] = path_str(copied_go_mod_path.dirname)
                go_work_lines.append("use {}".format(path_str(copied_go_mod_path.dirname)))

        if len(module.tags.from_file) > 1:
            module_ctx.fail("in {}, multiple go_deps.from_file tags were declared. Use a single go.work file if you need multiple modules.".format(module.name))
            return None
        for tag in module.tags.from_file:
            if bool(tag.go_work) == bool(tag.go_mod):
                module_ctx.fail("in {}, go_deps.from_file tag must have either go_work or go_mod attribute, but not both.".format(module.name))
                return None
            if tag.go_mod:
                visit_go_mod(tag.go_mod)
            else:
                # go.work
                go_work_path = module_ctx.path(tag.go_work)
                go_work_json = _parse_go_work_json(module_ctx, go_tool, go_env, go_work_path)
                if module_ctx.failed():
                    return None
                for u in go_work_json.get("Use"):
                    if u["DiskPath"] == "." or u["DiskPath"].startswith("./") or u["DiskPath"].startswith("../"):
                        go_mod_package = paths.normalize(paths.join(tag.go_work.package, u["DiskPath"]))
                        if go_mod_package == ".":
                            go_mod_package = ""
                        go_mod_label = Label("@@{}//{}:go.mod".format(tag.go_work.repo_name, go_mod_package))
                        visit_go_mod(go_mod_label)
                    else:
                        go_work_lines.append("use {}".format(u["DiskPath"]))

                if _module_acts_as_root(module_ctx, module):
                    _fix_replace_paths(go_work_path, go_work_json)
                    go_work_lines.extend([
                        "replace {}{} => {}{}".format(
                            r["Old"]["Path"],
                            " " + r["Old"]["Version"] if "Version" in r["Old"] else "",
                            r["New"]["Path"],
                            " " + r["New"]["Version"] if "Version" in r["New"] else "",
                        )
                        for r in go_work_json.get("Replace") or []
                    ])

                go_work_stem = go_work_path.basename[:-len(".work")] if go_work_path.basename.endswith(".work") else go_work_path.basename
                orig_go_sum_path = go_work_path.dirname.get_child(go_work_stem + ".sum")
                watch(module_ctx, orig_go_sum_path)
                if orig_go_sum_path.exists:
                    go_work_sum_content = module_ctx.read(orig_go_sum_path)
                    go_work_sum_lines.append(go_work_sum_content.strip())

    for tag in module_tag_requires.values():
        go_mod_lines.append("require {} {}".format(tag.path, tag.version))
        if tag.sum:
            go_sum_lines.append("{} {} {}".format(tag.path, tag.version, tag.sum))
        if tag.path in bazel_go_module_dirs and not tag.local_path:
            go_mod_lines.append("replace {} {} => {}".format(tag.path, tag.version, _local_replace_path(bazel_go_module_dirs[tag.path])))

    module_ctx.file("go.work", "\n".join(go_work_lines))
    module_ctx.file("go.mod", "\n".join(go_mod_lines))
    module_ctx.file("go.sum", "\n".join(go_sum_lines))
    if go_work_sum_lines:
        module_ctx.file("go.work.sum", "\n".join(go_work_sum_lines))

    return bazel_go_modules, root_required_mods

def _parse_go_mod_json(module_ctx, go_tool, go_env, go_mod_path):
    watch(module_ctx, go_mod_path)
    result = env_execute(module_ctx, [go_tool, "mod", "edit", "-json", "--", path_str(go_mod_path)], go_env)
    if result.return_code != 0:
        module_ctx.fail("parsing {}: {}".format(go_mod_path, result.stderr))
        return None
    return json.decode(result.stdout)

def _parse_go_work_json(module_ctx, go_tool, go_env, go_work_path):
    watch(module_ctx, go_work_path)
    result = env_execute(module_ctx, [go_tool, "work", "edit", "-json", "--", path_str(go_work_path)], go_env)
    if result.return_code != 0:
        module_ctx.fail("parsing {}: {}".format(go_work_path, result.stderr))
        return None
    return json.decode(result.stdout)

def _fix_replace_paths(go_mod_path, go_mod_json):
    """
    Edits go_mod_json so that relative replacement paths can be used in another directory

    This function should work with either go.mod or go.work, but we only use it
    with go.work. If a go.mod file is allowed to have replace directives, we
    use it in place. Other go.mod files are copied to the synthetic workspace
    without their replace directives.

    Args:
        go_mod_path: path to the go.mod or go.work file containing the replace.
        go_mod_json: mutable parsed JSON for that file. This is modified.
    """
    for replace in go_mod_json.get("Replace") or []:
        if not replace["New"].get("Version") and (
            replace["New"]["Path"] == "." or
            replace["New"]["Path"].startswith("./") or
            replace["New"]["Path"].startswith("../")
        ):
            replace["New"]["Path"] = paths.join(path_str(go_mod_path.dirname), replace["New"]["Path"])

def _format_go_mod_json(go_mod_json):
    lines = [
        "module {}".format(go_mod_json["Module"]["Path"]),
        "go {}".format(go_mod_json["Go"]),
    ]
    lines.extend([
        "require {} {}{}".format(
            r["Path"],
            r["Version"],
            " //indirect" if r.get("Indirect", False) else "",
        )
        for r in go_mod_json.get("Require") or []
    ])
    lines.extend([
        "replace {}{} => {}{}".format(
            r["Old"]["Path"],
            " " + r["Old"]["Version"] if "Version" in r["Old"] else "",
            r["New"]["Path"],
            " " + r["New"]["Version"] if "Version" in r["New"] else "",
        )
        for r in go_mod_json.get("Replace") or []
    ])
    lines.extend([
        "exclude {} {}".format(e["Path"], e["Version"])
        for e in go_mod_json.get("Exclude") or []
    ])
    return "\n".join(lines)

def _label_to_rel(label):
    return label.package + "/" + label.name if label.package else label.name

def _index_tool_targets(module_ctx, bazel_go_modules, root_required_mods, module_overrides):
    """
    Builds a map of tool targets and lists of directly required modules

    A tool target is declared with a 'tool' directive in a go.mod file
    from the root Bazel module. These targets may be run with
    'bazel run @rules_go//go -- tool <name>' where <name> is the last
    component of the tool's import path, ignoring major version suffixes.
    We build a map to write to @bazel_gazelle_go_repository_config so that
    @rules_go//go knows which target to run.

    A Go module is directly required by the root Bazel module if any of
    these conditions are true:

    - It was declared with go_deps.module in the root Bazel module.
    - Its 'require' directive in a go.mod file in the root Bazel module
      lacks an '// indirect' comment.
    - It provides a package named by a 'tool' directive. (We assume the longest
      matching module path provides this, but we don't know for sure.)

    We track direct requirements so that 'bazel mod tidy' can update
    'use_repo(go_deps, ...)' automatically.

    Args:
        module_ctx: the module context.
        bazel_go_modules: dict mapping Go module paths to _bazel_go_mod_info
            structs, returned by _create_workspace_from_tags.
        root_required_mods: dict mapping Go module paths to _go_require_info
            structs, returned by _create_workspace_from_tags.
        module_overrides: list of module_override tags from the root Bazel
            module, used to infer repo names for modules providing tools.

    Returns:
        - tool_targets: a dict mapping Go package paths to Bazel label strings.
        - direct_deps: a list of go_repository repo names of direct non-dev
             dependencies.
        - direct_dev_deps: a list of go_repository repo names of direct dev
             dependencies (declared when go_deps was loaded with
             'is_dev_dependency = True').
    """
    module_label_prefixes = {
        path: struct(
            repo_name = _get_repo_name(path, bazel_go_modules, module_overrides),
            package = mod.go_mod_label.package,
        )
        for path, mod in bazel_go_modules.items()
    } | {
        path: struct(repo_name = _get_repo_name(path, bazel_go_modules, module_overrides), package = "")
        for path in root_required_mods.keys()
    }

    tool_targets = {}  # tool import path => Bazel label string
    is_direct = {}  # module path => True
    for mod in bazel_go_modules.values():
        if not mod.is_root:
            continue
        for tool in mod.tool_importpaths:
            tool_target = None
            for tool_prefix in _path_prefixes(tool):
                label_prefix = module_label_prefixes.get(tool_prefix)
                if not label_prefix:
                    continue
                if tool == tool_prefix:
                    # package at Go module root
                    tool_target = "@{}//{}:{}".format(
                        label_prefix.repo_name,
                        label_prefix.package,
                        _tool_name(tool),
                    )
                else:
                    # package in subdirectory within Go module
                    tool_suffix = tool[len(tool_prefix) + 1:]
                    if label_prefix.package == "":
                        # Go module in repo root
                        tool_target = "@{}//{}:{}".format(
                            label_prefix.repo_name,
                            tool_suffix,
                            _tool_name(tool),
                        )
                    else:
                        # Go module in repo subdirectory
                        tool_target = "@{}//{}/{}:{}".format(
                            label_prefix.repo_name,
                            label_prefix.package,
                            tool_suffix,
                            _tool_name(tool),
                        )
                if tool_prefix not in bazel_go_modules:
                    is_direct[tool_prefix] = True
                break
            if not tool_target:
                module_ctx.fail("Go tool {} declared in {} is not provided by any known module".format(tool, str(mod.go_mod_label)))
                return None
            tool_targets[_tool_name(tool)] = tool_target

    for mod in root_required_mods.values():
        if mod.importpath not in bazel_go_modules and not mod.indirect:
            is_direct[mod.importpath] = True

    direct_deps = [
        module_label_prefixes[path].repo_name
        for path in is_direct
        if not root_required_mods[path].is_dev_dependency
    ]
    direct_dev_deps = [
        module_label_prefixes[path].repo_name
        for path in is_direct
        if root_required_mods[path].is_dev_dependency
    ]

    return tool_targets, direct_deps, direct_dev_deps

def _path_prefixes(path):
    """
    Returns a list of element prefixes of path, longest first

    For example, if given "example.com/a/b", returns
    ["example.com/a/b", "example.com/a", "example.com"],
    """
    elems = path.split("/")
    prefixes = [path]
    for elem in reversed(elems)[:-1]:
        path = path[:-len(elem) - 1]
        prefixes.append(path)
    return prefixes

def _tool_name(path):
    """
    Returns the name by which a tool can be invoked

    Args:
        path: an import path from a go.mod tool directive, like "example.com/cmd/foo"

    Returns:
        The name to pass to 'bazel run @rules_go//go -- tool <name>', like foo.
    """
    name = paths.basename(path)
    if name != path and name.startswith("v") and name[1:].isdigit():
        # major version suffix
        return paths.basename(paths.dirname(path))
    else:
        return name

def _select_module_versions(
        module_ctx,
        go_tool,
        go_env,
        bazel_go_modules,
        root_required_mods,
        module_overrides):
    """
    Runs 'go list -m' to decide what versions of Go modules to use.

    We run the actual 'go list -m' command because simulating Go's module logic
    in Starlark is surprisingly difficult. This may download .mod files into
    go_repository_cache and verify sums, but it won't download module .zip
    files. Respects configuration set by go_deps.config.

    Runs in the synthetic workspace created by _create_workspace_from_tags.

    Args:
        module_ctx: the module context.
        go_tool: path to the go tool, needed for 'go list -m -json all'.
        go_env: the environment to run the go tool with.
        bazel_go_modules: a dict mapping Go module path to _bazel_go_mod_info
            struct.
        root_required_mods: dict mapping Go module paths to _go_require_info
            structs, returned by _create_workspace_from_tags.
        module_overrides: list of module_override tags from the root Bazel
            module, used to infer repo names for modules providing tools.

    Returns:
        A dict mapping Go module paths to _go_module_info structs.
    """

    list_result = env_execute(module_ctx, [go_tool, "list", "-m", "-json", "all"], go_env)
    if list_result.return_code:
        module_ctx.fail("selecting Go module versions:\n{}".format(list_result.stderr))
        return None

    # json.decode parses a single JSON value, but `go list -m -json all` prints
    # one JSON object per module, separated by newlines. Split stdout on the
    # boundary between objects before decoding each value.
    sep = "\n}\n"
    parsed_list_results = [
        json.decode(part.strip() + sep)
        for part in list_result.stdout.split(sep)
        if part.strip() != ""
    ]
    go_modules = {}
    for m in parsed_list_results:
        importpath = m["Path"]
        if "Replace" in m:
            if "Version" in m["Replace"]:
                replace_path = m["Replace"]["Path"]
                version = m["Replace"]["Version"]
                if "Sum" not in m["Replace"]:
                    module_ctx.fail("""\
{importpath}: sum missing for replacement {repl_importpath}@{repl_version}
Add to go.sum with:
    go mod download {repl_importpath}@{repl_version}""".format(
                        importpath = importpath,
                        repl_importpath = m["Replace"]["Path"],
                        repl_version = m["Replace"]["Version"],
                    ))
                    return None
                sum = m["Replace"]["Sum"]
                local_path = None
            else:
                replace_path = None
                version = None
                sum = None
                local_path = m["Replace"]["Dir"]
        else:
            version = m.get("Version")
            sum = m.get("Sum")
            replace_path = None
            local_path = None
        if (importpath in root_required_mods and
            root_required_mods[importpath].local_path != None):
            if local_path != None:
                module_ctx.fail("{}: Go module has a local path set by both go_deps.module.local_path and a Go replace directive".format(importpath))
                return None
            local_path = root_required_mods[importpath].local_path

        go_modules[importpath] = _go_module_info(
            importpath = importpath,
            version = version,
            sum = sum,
            replace_path = replace_path,
            local_path = local_path,
            repo_name = _get_repo_name(importpath, bazel_go_modules, module_overrides),
            go_mod_label = bazel_go_modules[importpath].go_mod_label if importpath in bazel_go_modules else None,
        )
    return go_modules

def _parse_go_version(v):
    """
    Parses a go version like "go1.23" or "go1.27.8" or "go1.18beta2"

    Drops the "go" prefix if present and any suffix after the version numbers
    like "rc3" or "beta2".

    Returns:
        an array of integers that can be compared to other versions
    """
    if v.startswith("go"):
        v = v[2:]
    for i, c in enumerate(v.elems()):
        if c != "." and c < "0" or "9" < c:
            v = v[:i]
            break
    return [int(part) for part in v.split(".")]

def _normalize_version(version):
    """Strips a leading 'v' from a Go module version for comparison."""
    if version.startswith("v"):
        return version[1:]
    return version

def _collect_reserved_repo_names(module_ctx, bazel_go_modules):
    """Returns repo names already taken by Bazel modules before declaring go_repository rules.

    Maps repo name to a struct describing what owns the name. Used to detect
    collisions between go_repository rules, Bazel module names, and Go modules
    provided via go_deps.from_file.
    """
    reserved = {}
    for module in module_ctx.modules:
        reserved[module.name] = struct(
            kind = "bazel_module",
            bazel_module_name = module.name,
        )
    for info in bazel_go_modules.values():
        if info.repo_name in reserved:
            continue
        reserved[info.repo_name] = struct(
            kind = "bazel_go_module",
            importpath = info.importpath,
            bazel_module_name = info.bazel_dep_name,
        )
    return reserved

def _check_for_version_conflict(
        module_ctx,
        go_modules,
        bazel_go_modules,
        root_module_tags,
        root_required_mods,
        report_error,
        reserved_repo_names):
    """
    Reports problems with module selections and repository names.

    Bazel's module system selects Bazel module versions before go_deps runs,
    and there's no way to override it. We report errors when the user attempts
    to select a different version through Go.

    Args:
        module_ctx: the module context, used for hard failures.
        go_modules: a dict mapping Go module path to
            _go_module_info structs.
        bazel_go_modules: a dict mapping Go module path to _bazel_go_mod_info
            structs.
        root_module_tags: a list of go_deps.module tags from the root Bazel
            mdoule. We don't report problems with other tags because the user
            can't do anything about them.
        root_required_mods: a dict mapping Go module paths to _go_require_info
            structs for modules required from the root Bazel module.
        report_error: module_ctx.print, module_ctx.fail, or a no-op function,
            depending on go_deps.config(checks).
        reserved_repo_names: mutable dict of repo names already in use, updated
            with each go_repository that will be declared. Used to detect name
            collisions and decide whether to declare the rules_proto shim.
    """
    for path, require in root_required_mods.items():
        bazel_dep = bazel_go_modules.get(path)
        if not bazel_dep or bazel_dep.go_mod_label.package != "":
            # Skip check if the module is not in the root directory, for example,
            # Gazelle's v2/go.mod. When a Bazel module provides multiple Go
            # modules, there's not a good correspondence between Bazel module
            # version and Go module version.
            continue
        normalized_require_version = _normalize_version(require.version)
        if (path in bazel_go_modules and
            not bazel_dep.is_root and
            _normalize_version(bazel_dep.bazel_dep_version) != normalized_require_version):
            report_error("""\
Version conflict found for Go module {importpath}:
    provided by Bazel module:       {bazel_dep_version}
    requested by go_deps.from_file: {normalized_require_version}
To correct this:
    1. Update the bazel_dep for {bazel_dep_name} in MODULE.bazel.
    2. Or update go.mod with 'go get {importpath}@v{bazel_dep_version}'.
""".format(
                importpath = path,
                bazel_dep_name = bazel_dep.bazel_dep_name,
                bazel_dep_version = bazel_dep.bazel_dep_version,
                normalized_require_version = normalized_require_version,
            ))

    for tag in root_module_tags:
        if tag.path in bazel_go_modules:
            if tag.local_path != "":
                report_error("""\
Version conflict found for Go module {importpath}:
    provided by Bazel module:    {bazel_dep_version}
    requested by go_deps.module: local_path {local_path}
To replace the content of a Bazel module, use local_path_override.
""".format(
                    importpath = tag.path,
                    bazel_dep_version = bazel_go_modules[tag.path].bazel_dep_version,
                    local_path = tag.local_path,
                ))
                continue
            if _normalize_version(tag.version) != _normalize_version(bazel_go_modules[tag.path].bazel_dep_version):
                report_error("""\
Version conflict found for Go module {importpath}:
    provided by Bazel module:    {bazel_dep_version}
    requested by go_deps.module: {tag_version}
To correct this:
    1. Drop the go_deps.module tag for {importpath} in MODULE.bazel.
    2. Or drop the bazel_dep for {bazel_dep_name} in MODULE.bazel.
""".format(
                    importpath = tag.path,
                    bazel_dep_name = bazel_go_modules[tag.path].bazel_dep_name,
                    bazel_dep_version = bazel_go_modules[tag.path].bazel_dep_version,
                    tag_version = tag.version,
                ))
                continue
            continue

        go_module = go_modules[tag.path]
        if tag.version != go_module.version:
            report_error("""\
Version conflict found for Go module {importpath}:
    requested with go_deps.module: {tag_version}
    selected by Go:                {go_version}
To correct this:
    1. Set the go_deps.module version to {go_version}.
    2. Ensure that no higher version is requested in any MODULE.bazel, go.mod,
       or go.work file. When using go_deps.from_file, you can use 'go get'
       to downgrade indirect dependencies if needed.
""".format(
                importpath = go_module.importpath,
                tag_version = tag.version,
                go_version = go_module.version,
            ))

    root_module_tag_paths = {tag.path: True for tag in root_module_tags}
    for path, require in root_required_mods.items():
        if (require.indirect or
            path in bazel_go_modules or
            path in root_module_tag_paths):
            continue
        go_module = go_modules[path]
        if go_module.version != None and require.version != go_module.version:
            report_error("""\
Version conflict found for Go module {importpath}:
    requested in root module: {require_version}
    selected by Go:           {go_version}
To correct this:
    1. Set the requested version to {go_version}.
    2. Ensure that no higher version is requested in any MODULE.bazel, go.mod,
       or go.work file. When using go_deps.from_file, you can use 'go get'
       to downgrade indirect dependencies if needed.
""".format(
                importpath = path,
                require_version = require.version,
                go_version = go_module.version,
            ))

    for path, go_module in go_modules.items():
        if path not in root_required_mods or go_module.version == None:
            continue
        if (go_module.go_mod_label == None and
            go_module.sum == None and
            go_module.local_path == None):
            report_error("""\
Missing go.sum entry for Go module {importpath}:
    selected by Go: {go_version}
To correct this:
    Run 'go get {importpath}@{go_version}' to update go.mod and go.sum.
""".format(
                importpath = path,
                go_version = go_module.version,
            ))

    for go_module in go_modules.values():
        if not _should_declare_go_repository(module_ctx, go_module):
            continue
        existing = reserved_repo_names.get(go_module.repo_name)
        if existing:
            if existing.kind == "go_repository":
                module_ctx.fail("Go module {prev_path} and {path} will resolve to the same Bazel repo name: {name}. While Go allows modules to only differ in case, this isn't supported in Gazelle. Please ensure you only use one of these modules in your go.mod(s), or assign a distinct repo name to one of them via the \"repo_name\" attribute of a \"module_override\" tag.".format(
                    prev_path = existing.importpath,
                    path = go_module.importpath,
                    name = go_module.repo_name,
                ))
            elif existing.kind == "bazel_module":
                module_ctx.fail("Go module {path} will resolve to Bazel repo name \"{name}\", which is already used by Bazel module \"{bazel_module_name}\".".format(
                    path = go_module.importpath,
                    name = go_module.repo_name,
                    bazel_module_name = existing.bazel_module_name,
                ))
            else:
                module_ctx.fail("Go module {path} will resolve to Bazel repo name \"{name}\", which is already used by Go module \"{importpath}\" from Bazel module \"{bazel_module_name}\".".format(
                    path = go_module.importpath,
                    name = go_module.repo_name,
                    importpath = existing.importpath,
                    bazel_module_name = existing.bazel_module_name,
                ))
            return
        reserved_repo_names[go_module.repo_name] = struct(
            kind = "go_repository",
            importpath = go_module.importpath,
        )

_GAZELLE_DEFAULT_ATTRIBUTES = struct(
    build_file_generation = "on",
    build_extra_args = [],
    directives = [],
)

_GAZELLE_ATTRS = {
    "build_file_generation": attr.string(
        default = _GAZELLE_DEFAULT_ATTRIBUTES.build_file_generation,
        doc = """One of `"auto"`, `"on"` (default), `"off"`, `"clean"`.

        Whether Gazelle should generate build files for the Go module.

        Although "auto" is the default globally for build_file_generation,
        if a `"gazelle_override"` or `"gazelle_default_attributes"` tag is present
        for a Go module, the `"build_file_generation"` attribute will default to "on"
        since these tags indicate the presence of `"directives"` or `"build_extra_args"`.

        In `"auto"` mode, Gazelle will run if there is no build file in the Go
        module's root directory.

        In `"clean"` mode, Gazelle will first remove any existing build files.

        """,
        values = [
            "auto",
            "off",
            "on",
            "clean",
        ],
    ),
    "build_extra_args": attr.string_list(
        default = _GAZELLE_DEFAULT_ATTRIBUTES.build_extra_args,
        doc = """
        A list of additional command line arguments to pass to Gazelle when generating build files.
        """,
    ),
    "directives": attr.string_list(
        default = _GAZELLE_DEFAULT_ATTRIBUTES.directives,
        doc = """Gazelle configuration directives to use for this Go module's external repository.

        Each directive uses the same format as those that Gazelle
        accepts as comments in Bazel source files, with the
        directive name followed by optional arguments separated by
        whitespace.""",
    ),
}

def _wrap_module_ctx_for_testability(module_ctx):
    """Wraps a real module_ctx for use in functions that can be unit tested.

    Unit tests can mock module_ctx to simulate reading files, executing commands,
    and so on. Unfortunately, they cannot mock repository rule registration,
    which is the main thing tests need to verify. So we add a "declare_repo"
    method to be used instead of calling a repo rule directly. In tests, the mock
    module_ctx has this already.
    """
    if hasattr(module_ctx, "declare_repo"):
        return module_ctx
    return struct(
        declare_repo = lambda rule, **kwargs: rule(**kwargs),
        print = print,
        fail = fail,
        failed = lambda: False,
        **{k: getattr(module_ctx, k) for k in dir(module_ctx)}
    )

_config_tag = tag_class(
    doc = "Configures the general behavior of the go_deps extension.",
    attrs = {
        "checks": attr.string(
            doc = """\
            How to handle problems with inconsistent versions, like a Go module being
            requested at different versions with go_deps.module and go.mod.
            "error" fails the build when an inconsistency is detected.
            "warning" prints a message. "off" suppresses these messages.
            """,
            values = ["off", "warning", "error"],
            default = "warning",
        ),
        "check_direct_dependencies": attr.string(
            doc = "DEPRECATED: Use `checks` instead.",
            values = ["off", "warning", "error"],
            default = "off",
        ),
        "go_env": attr.string_dict(
            doc = "The environment variables to use when fetching Go dependencies or running the `@rules_go//go` tool.",
        ),
        "go_env_inherit": attr.string_list(
            doc = "Host environment variable names to inherit when fetching Go dependencies or running the `@rules_go//go` tool.",
        ),
        "debug_mode": attr.bool(doc = "Whether or not to print stdout and stderr messages from gazelle", default = False),
    },
)

_from_file_tag = tag_class(
    doc = """
    Imports Go module dependencies from either a go.mod file or a go.work file.

    All direct and indirect dependencies of the specified module will be imported, but only direct dependencies should
    be imported into the scope of the using module via `use_repo` calls. Use `bazel mod tidy` to update these calls
    automatically.
    """,
    attrs = {
        "go_mod": attr.label(mandatory = False),
        "go_work": attr.label(mandatory = False),
        "fail_on_version_conflict": attr.bool(
            default = False,
            doc = 'DEPRECATED: Use `go_deps.config(checks = "error")` instead.',
        ),
    },
)

_module_tag = tag_class(
    doc = """Declare a single Go module dependency. Prefer using `from_file` instead.""",
    attrs = {
        "path": attr.string(
            doc = """The module path.""",
            mandatory = True,
        ),
        "version": attr.string(mandatory = True),
        "sum": attr.string(),
        "indirect": attr.bool(
            doc = """Whether this Go module is an indirect dependency.""",
            default = False,
        ),
        "local_path": attr.string(
            doc = """For when a module is replaced by one residing in a local directory path """,
            mandatory = False,
        ),
    },
)

_archive_override_tag = tag_class(
    attrs = {
        "path": attr.string(
            doc = """The Go module path for the repository to be overridden.

            This module path must be defined by other tags in this
            extension within this Bazel module.""",
            mandatory = True,
        ),
        "urls": attr.string_list(
            doc = """A list of HTTP(S) URLs where an archive containing the project can be
            downloaded. Bazel will attempt to download from the first URL; the others
            are mirrors.""",
        ),
        "strip_prefix": attr.string(
            doc = """If the repository is downloaded via HTTP (`urls` is set), this is a
            directory prefix to strip. See [`http_archive.strip_prefix`].""",
        ),
        "sha256": attr.string(
            doc = """If the repository is downloaded via HTTP (`urls` is set), this is the
            SHA-256 sum of the downloaded archive. When set, Bazel will verify the archive
            against this sum before extracting it.""",
        ),
        "patches": attr.label_list(
            doc = "A list of patches to apply to the repository *after* gazelle runs.",
        ),
        "patch_strip": attr.int(
            default = 0,
            doc = "The number of leading path segments to be stripped from the file name in the patches.",
        ),
        "patch_cmds": attr.string_list(
            doc = "Commands to run in the repository after patches are applied.",
        ),
    },
    doc = "Override the default source location on a given Go module in this extension.",
)

_gazelle_override_tag = tag_class(
    attrs = {
        "path": attr.string(
            doc = """The Go module path for the repository to be overridden.

            This module path must be defined by other tags in this
            extension within this Bazel module.""",
            mandatory = True,
        ),
    } | _GAZELLE_ATTRS,
    doc = "Override Gazelle's behavior on a given Go module defined by other tags in this extension.",
)

_gazelle_default_attributes_tag = tag_class(
    attrs = _GAZELLE_ATTRS,
    doc = "Override Gazelle's default attribute values for all modules in this extension.",
)

_module_override_tag = tag_class(
    attrs = {
        "path": attr.string(
            doc = """The Go module path for the repository to be overridden.

            This module path must be defined by other tags in this
            extension within this Bazel module.""",
            mandatory = True,
        ),
        "patches": attr.label_list(
            doc = "A list of patches to apply to the repository *after* gazelle runs.",
        ),
        "patch_strip": attr.int(
            default = 0,
            doc = "The number of leading path segments to be stripped from the file name in the patches.",
        ),
        "patch_cmds": attr.string_list(
            doc = "Commands to run in the repository after patches are applied.",
        ),
        "repo_name": attr.string(
            doc = """The Bazel repository name to use for this Go module.

            By default, Gazelle derives the repository name from the module's
            import path. Two distinct modules whose import paths differ only by
            "/" vs "_" (or only by case) derive the same default name, which
            Gazelle rejects with an error. When both are pulled in transitively,
            dropping one from the go.mod is not always possible. Setting this
            attribute lets the root module assign a distinct repository name to
            one of the colliding modules so they can coexist. The collision
            check still runs on the resulting names.""",
        ),
    },
    doc = "Override the definition of a Go module defined by other tags in this extension, e.g. to apply patches or change its Bazel repository name.",
)

# Older Gazelle versions generated BUILD files that load proto_library from
# @rules_proto//proto:defs.bzl.
_RULES_PROTO_COMPAT_DEFS_BZL = '''\
"""Forwards proto_library from its new home in Protobuf, as previously provided by @rules_proto//proto:defs.bzl.

This repository is generated by the go_deps module extension of Gazelle, which no longer depends
on rules_proto. It keeps the BUILD files of Go modules working that were generated by older
versions of Gazelle and thus still load proto_library from @rules_proto
(https://github.com/bazel-contrib/bazel-gazelle/issues/2358).

BUILD files that load other symbols from @rules_proto can be regenerated by adding a
go_deps.gazelle_override tag with build_file_generation = "clean" for the Go module to the
relevant MODULE.bazel file.
"""

load("@com_google_protobuf//bazel:proto_library.bzl", _proto_library = "proto_library")

proto_library = _proto_library
'''

def _rules_proto_compat_impl(ctx):
    ctx.file("WORKSPACE")
    ctx.file("proto/BUILD.bazel")
    ctx.file("proto/defs.bzl", _RULES_PROTO_COMPAT_DEFS_BZL)

_rules_proto_compat = repository_rule(
    implementation = _rules_proto_compat_impl,
    doc = """Compatibility shim for @rules_proto, which Gazelle no longer depends on, that keeps
    the BUILD files of Go modules working that still load proto rules from it.""",
)

go_deps = module_extension(
    implementation = go_deps_impl,
    tag_classes = {
        "archive_override": _archive_override_tag,
        "config": _config_tag,
        "from_file": _from_file_tag,
        "gazelle_override": _gazelle_override_tag,
        "gazelle_default_attributes": _gazelle_default_attributes_tag,
        "module": _module_tag,
        "module_override": _module_override_tag,
    },
)
