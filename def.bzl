# Copyright 2017 The Bazel Authors. All rights reserved.
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

load(
    "@bazel_skylib//lib:shell.bzl",
    "shell",
)
load(
    "//internal:go_repository.bzl",
    _go_repository = "go_repository",
)
load(
    "//internal:overlay_repository.bzl",
    _git_repository = "git_repository",
    _http_archive = "http_archive",
)
load(
    "//internal:gazelle_binary.bzl",
    _gazelle_binary = "gazelle_binary_wrapper",
)
load(
    "//internal/generationtest:generationtest.bzl",
    _gazelle_generation_test = "gazelle_generation_test",
)

go_repository = _go_repository
git_repository = _git_repository
http_archive = _http_archive
gazelle_binary = _gazelle_binary
gazelle_generation_test = _gazelle_generation_test

DEFAULT_LANGUAGES = [
    Label("//language/proto:go_default_library"),
    Label("//language/go:go_default_library"),
]

def _gazelle_runner_impl(ctx):
    args = [ctx.attr.command]
    if ctx.attr.mode:
        args.extend(["-mode", ctx.attr.mode])
    if ctx.attr.external:
        args.extend(["-external", ctx.attr.external])
    if ctx.attr.prefix:
        args.extend(["-go_prefix", ctx.attr.prefix])
    if ctx.attr.build_tags:
        args.extend(["-build_tags", ",".join(ctx.attr.build_tags)])
    args.extend([ctx.expand_location(arg, ctx.attr.data) for arg in ctx.attr.extra_args])

    out_file = ctx.actions.declare_file(ctx.label.name + ".bash")
    go_tool = ctx.toolchains["@io_bazel_rules_go//go:toolchain"].sdk.go
    substitutions = {
        "@@ARGS@@": shell.array_literal(args),
        "@@GAZELLE_LABEL@@": shell.quote(str(ctx.attr.gazelle.label)),
        "@@GAZELLE_SHORT_PATH@@": shell.quote(ctx.executable.gazelle.short_path),
        "@@GENERATED_MESSAGE@@": """
# Generated by {label}
# DO NOT EDIT
""".format(label = str(ctx.label)),
        "@@RUNNER_LABEL@@": shell.quote(str(ctx.label)),
        "@@GOTOOL@@": shell.quote(go_tool.path),
    }
    ctx.actions.expand_template(
        template = ctx.file._template,
        output = out_file,
        substitutions = substitutions,
        is_executable = True,
    )
    runfiles = ctx.runfiles(files = [
        ctx.executable.gazelle,
        go_tool,
    ]).merge(
        ctx.attr.gazelle[DefaultInfo].default_runfiles,
    )
    for d in ctx.attr.data:
        runfiles = runfiles.merge(d[DefaultInfo].default_runfiles)
    return [DefaultInfo(
        files = depset([out_file]),
        runfiles = runfiles,
        executable = out_file,
    )]

_gazelle_runner = rule(
    implementation = _gazelle_runner_impl,
    attrs = {
        "gazelle": attr.label(
            default = "//cmd/gazelle",
            executable = True,
            cfg = "exec",
        ),
        "command": attr.string(
            values = [
                "fix",
                "update",
                "update-repos",
            ],
            default = "update",
        ),
        "mode": attr.string(
            values = ["", "print", "fix", "diff"],
            default = "",
        ),
        "external": attr.string(
            values = ["", "external", "static", "vendored"],
            default = "",
        ),
        "build_tags": attr.string_list(),
        "prefix": attr.string(),
        "extra_args": attr.string_list(),
        "data": attr.label_list(allow_files = True),
        "_template": attr.label(
            default = "//internal:gazelle.bash.in",
            allow_single_file = True,
        ),
    },
    executable = True,
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)

def gazelle(name, **kwargs):
    if "args" in kwargs:
        # The args attribute has special meaning for executable rules, but we
        # always want extra_args here instead.
        if "extra_args" in kwargs:
            fail("{}: both args and extra_args were provided".format(name))
        kwargs["extra_args"] = kwargs["args"]
        kwargs.pop("args")

    visibility = kwargs.pop("visibility", default = None)

    tags_set = {t: "" for t in kwargs.pop("tags", [])}
    tags_set["manual"] = ""
    tags = [k for k in tags_set.keys()]
    runner_name = name + "-runner"
    _gazelle_runner(
        name = runner_name,
        tags = tags,
        **kwargs
    )
    native.sh_binary(
        name = name,
        srcs = [runner_name],
        tags = tags,
        visibility = visibility,
    )
