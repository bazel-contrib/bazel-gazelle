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

load("//internal:env.bzl", "write_go_env_file")
load(":utils.bzl", "format_rule_call")

def _go_repository_config_impl(ctx):
    repos = []
    for name, importpath in sorted(ctx.attr.importpaths.items()):
        repos.append(format_rule_call(
            "go_repository",
            name = name,
            importpath = importpath,
            module_name = ctx.attr.module_names.get(name),
            build_naming_convention = ctx.attr.build_naming_conventions.get(name),
        ))

    ctx.file("WORKSPACE", "\n".join(repos))
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
        "importpaths": attr.string_dict(
            mandatory = True,
            doc = "Map from repo name to Go module path",
        ),
        "module_names": attr.string_dict(
            mandatory = True,
            doc = "Map from repo name (including '@' prefix) to Bazel module name, for when modules are renamed",
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
