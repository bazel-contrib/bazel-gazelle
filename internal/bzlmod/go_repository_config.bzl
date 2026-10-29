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

"""go_repository_config internal repo rule, used by go_deps"""

load("//internal:common.bzl", "repository_config_input_path")
load("//internal:env.bzl", "write_go_env_file")

def _import_path_prefixes(importpath):
    """Returns importpath and its slash-delimited prefixes, shortest first."""
    prefixes = []
    end = -1
    for _ in range(importpath.count("/")):
        end = importpath.find("/", end + 1)
        prefixes.append(importpath[:end])
    prefixes.append(importpath)
    return prefixes

def _module_paths(importpath):
    """Returns a declared Go module path and its possible versionless alias.

    For example, "example.com/lib/v2" also returns "example.com/lib".
    This mirrors repo/remote.go:pathWithoutSemver for repo.NewRemoteCache's
    minimal module compatibility lookup. It does not infer modules from a
    package import. repository_config_inputs indexes every slash-delimited
    prefix of both returned paths via _import_path_prefixes.
    """
    paths = [importpath]
    start = 0
    for _ in range(importpath.count("/v")):
        start = importpath.find("/v", start)
        end = importpath.find("/", start + 2)
        if end == -1:
            end = len(importpath)
        version = importpath[start + 2:end]
        if version.isdigit():
            # Match repo/remote.go:pathWithoutSemver: only the first numeric
            # version component counts, even when it is v0 or v1.
            if version not in ("0", "1"):
                paths.append(importpath[:start] + importpath[end:])
            break
        start = end
    return paths

def repository_config_inputs(importpaths):
    """Finds the repository records that can affect each import prefix.

    Args:
        importpaths: Iterable of declared Go module paths (strings), in the
            same order as the repository records written to WORKSPACE.
            _go_repository_config_impl passes the keys of ctx.attr.repo_names.

    Returns:
        dict[str, list[int]]: each slash-delimited prefix of a declared module
        or its versionless alias maps to the zero-based indices of its matching
        records. The indices are in input order, as required by
        repo.NewRemoteCache's versionless alias precedence. A record matches
        if its module or alias is an ancestor or descendant of the prefix.
        Ancestors may provide an import; descendants may become a more specific
        provider or need access to an internal package.

        For ["example.com/lib", "example.com/lib/sub", "example.com/other"],
        both "example.com/lib" and "example.com/lib/sub" map to [0, 1], while
        "example.com/other" maps to [2]. go_repository watches the nearest
        indexed prefix of each import or visibility prefix reported by Gazelle.
    """
    ancestors = {}
    inputs = {}
    for index, importpath in enumerate(importpaths):
        for module_path in _module_paths(importpath):
            ancestors.setdefault(module_path, {})[index] = None
            for prefix in _import_path_prefixes(module_path):
                inputs.setdefault(prefix, {})[index] = None

    # inputs already includes descendants. Parents must be visited first to
    # inherit ancestors, which can also change the matching Go repository.
    for prefix in sorted(inputs):
        parent, sep, _ = prefix.rpartition("/")
        parent_indices = ancestors.get(parent) if sep else None
        if parent_indices:
            ancestors.setdefault(prefix, {}).update(parent_indices)
            inputs[prefix].update(parent_indices)

    # RemoteCache resolves versionless aliases in WORKSPACE order.
    return {prefix: sorted(indices) for prefix, indices in inputs.items()}

def _go_repository_config_impl(ctx):
    lines = []
    for importpath, repo_name in ctx.attr.repo_names.items():
        # gazelle:repository directives allow duplicate names for Bazel modules
        # that provide multiple Go modules.
        #
        # TODO(#2413): change to a JSON format after we drop support
        # for WORKSPACE mode.
        #
        # BUG: module_name, prefix_dir, and possibly other tokens may
        # contain spaces, and the gazelle:repository directive does not support
        # quoting. We ignore this for now; changing to JSON should fix this.
        words = [
            "# gazelle:repository",
            "go_repository",
            "name=" + repo_name,
            "importpath=" + importpath,
        ]
        module_name = ctx.attr.module_names.get(repo_name)
        if module_name:
            words.append("module_name=" + module_name)
        prefix_dir = ctx.attr.prefix_dirs.get(importpath)
        if prefix_dir:
            words.append("prefix_dir=" + prefix_dir)
        build_naming_convention = ctx.attr.build_naming_conventions.get(repo_name)
        if build_naming_convention:
            words.append("build_naming_convention=" + build_naming_convention)
        lines.append(" ".join(words))

    ctx.file("WORKSPACE", "\n".join(lines))

    # Gazelle reads WORKSPACE, but go_repository must not watch the whole file:
    # an unrelated module change would refetch every repository. Instead, these
    # files contain just the records that can affect each indexed prefix.
    # go_repository watches the files for the imports and visibility prefixes
    # Gazelle consulted. Their contents are not parsed; they only need to change
    # when the matching records change, including renames and naming conventions.
    for prefix, indices in repository_config_inputs(ctx.attr.repo_names).items():
        ctx.file(repository_config_input_path(prefix), "\n".join([lines[i] for i in indices]))
    write_go_env_file(ctx, ctx.attr.go_env)
    ctx.file("BUILD.bazel", "exports_files(['WORKSPACE', 'config.json', 'go.env', 'go_env.bzl', 'go_tools.bzl'])")
    ctx.file("go_env.bzl", content = "GO_ENV = " + repr(ctx.attr.go_env))
    ctx.file("go_tools.bzl", content = "GO_TOOLS = {{k: Label(v) for k, v in {}.items()}}".format(
        repr(ctx.attr.tool_targets),
    ))

    # For use by @rules_go//go.
    ctx.file("config.json", content = json.encode_indent({
        "go_env": ctx.attr.go_env,
        "dep_files": ctx.attr.dep_files,
    }))

go_repository_config = repository_rule(
    implementation = _go_repository_config_impl,
    attrs = {
        "repo_names": attr.string_dict(
            mandatory = True,
            doc = "Map from Go module path to repo name",
        ),
        "module_names": attr.string_dict(
            mandatory = True,
            doc = "Map from repo name (including '@' prefix) to Bazel module name, for when modules are renamed",
        ),
        "prefix_dirs": attr.string_dict(
            mandatory = True,
            doc = "Map from Go module path to module root directory within repo, when Go module root is different than Bazel module root",
        ),
        "tool_targets": attr.string_dict(
            mandatory = True,
            doc = "Map from Go import path to Bazel label for 'tool' directives in go.mod files",
        ),
        "build_naming_conventions": attr.string_dict(
            mandatory = True,
            doc = "Map from repo name to 'build_naming_convention' setting for that repo",
        ),
        "go_env": attr.string_dict(
            mandatory = True,
            doc = "Explicit settings for Go environment variables, from go_deps.config",
        ),
        "dep_files": attr.string_list(
            doc = "List go.mod files in the main Bazel module (just one?). @rules_go//go may run 'bazel mod tidy' when these change.",
        ),
    },
    doc = """
    Generates configuration files used by tools that depend on go_deps

    - WORKSPACE: a configuration file loaded by gazelle within go_repository,
      used to map between Go module paths and repo names.
    - go_env.bzl: contains a GO_ENV dict variable with environment settings to
      be used any time we run Go tools. We use this when running the go command
      via @rules_go//go.
    - go.env: relocatable Go environment settings for go_repository.
    - go_tools.bzl: maps tool names to labels, based on "tool" directives in
      go.mod files declared with go_deps.from_file; used for
      'bazel run @rules_go//go -- tool customtool ...'.
    - config.json: encodes the GO_ENV dict and dep_files, used by @rules_go//go.
    """,
)
