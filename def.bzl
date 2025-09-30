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
    "@bazel_gazelle_go_repository_config//:go_env.bzl",
    "GO_ENV",
)
load(
    "@bazel_gazelle_is_bazel_module//:defs.bzl",
    "GAZELLE_IS_BAZEL_MODULE",
)
load(
    "@bazel_skylib//lib:shell.bzl",
    "shell",
)
load(
    "@rules_shell//shell:sh_binary.bzl",
    "sh_binary",
)
load(
    "@rules_shell//shell:sh_test.bzl",
    "sh_test",
)
load(
    "//internal:gazelle_binary.bzl",
    _gazelle_binary = "gazelle_binary_wrapper",
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
    "//internal/generationtest:generationtest.bzl",
    _gazelle_generation_test = "gazelle_generation_test",
)

go_repository = _go_repository
git_repository = _git_repository
http_archive = _http_archive
gazelle_binary = _gazelle_binary
gazelle_generation_test = _gazelle_generation_test

DEFAULT_LANGUAGES = [
    Label("//language/bazel/visibility"),
    Label("//language/proto"),
    Label("//language/go"),
]

def _valid_env_variable_name(name):
    """ Returns if a string is in the regex [a-zA-Z_][a-zA-Z0-9_]*

    Given that bazel lacks support of regex, we need to implement
    a poor man validation
    """
    if not name:
        return False
    for i, c in enumerate(name.elems()):
        if c.isalpha() or c == "_" or (i > 0 and c.isdigit()):
            continue
        return False
    return True

def _rlocation_path(ctx, file):
    if file.short_path.startswith("../"):
        return file.short_path[3:]
    else:
        return ctx.workspace_name + "/" + file.short_path

def _gazelle_runner_attr_factory(test_runner = False):
    attrs = {
        "gazelle": attr.label(
            allow_single_file = True,
            default = "//cmd/gazelle",
            executable = True,
            cfg = "target",
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
        "env": attr.string_dict(),
        "_repo_config": attr.label(
            default = "@bazel_gazelle_go_repository_config//:WORKSPACE" if GAZELLE_IS_BAZEL_MODULE else None,
            allow_single_file = True,
        ),
        "_template": attr.label(
            default = "//internal:gazelle.bash.in",
            allow_single_file = True,
        ),
    }
    if test_runner:
        attrs.update({
            "workspace": attr.label(
                allow_single_file = True,
                doc = "Label of the WORKSPACE file",
                mandatory = True,
            ),
            "mode": attr.string(
                values = ["diff"],
                default = "diff",
            ),
        })
    return attrs

def _gazelle_runner_impl_factory(ctx, test_runner = False):
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

    combined_env = {}
    combined_env.update(GO_ENV)
    combined_env.update(ctx.attr.env)
    for key in combined_env:
        if not _valid_env_variable_name(key):
            fail("Invalid environmental variable name: '%s'" % key)

    env = "\n".join(["export %s=%s" % (x, shell.quote(y)) for (x, y) in combined_env.items()])

    out_file = ctx.actions.declare_file(ctx.label.name + ".bash")
    go_tool = ctx.toolchains["@io_bazel_rules_go//go:toolchain"].sdk.go
    repo_config = ctx.file._repo_config
    substitutions = {
        "@@ARGS@@": shell.array_literal(args),
        "@@GAZELLE_PATH@@": shell.quote(_rlocation_path(ctx, ctx.executable.gazelle)),
        "@@GENERATED_MESSAGE@@": """
# Generated by {label}
# DO NOT EDIT
""".format(label = str(ctx.label)),
        "@@GOTOOL@@": shell.quote(_rlocation_path(ctx, go_tool)),
        "@@ENV@@": env,
        "@@REPO_CONFIG_PATH@@": shell.quote(_rlocation_path(ctx, repo_config)) if repo_config else "",
        "@@WORKSPACE@@": ctx.file.workspace.path if test_runner else "",
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
    ] + ([repo_config] if repo_config else []) + ([ctx.file.workspace] if test_runner else [])).merge(
        ctx.attr.gazelle[DefaultInfo].default_runfiles,
    )
    for d in ctx.attr.data:
        runfiles = runfiles.merge(d[DefaultInfo].default_runfiles)
    return [DefaultInfo(
        files = depset([out_file]),
        runfiles = runfiles,
        executable = out_file,
    )]

def _gazelle_runner_impl(ctx):
    return _gazelle_runner_impl_factory(ctx)

def _gazelle_test_runner_impl(ctx):
    return _gazelle_runner_impl_factory(ctx, test_runner = True)

_gazelle_runner = rule(
    implementation = _gazelle_runner_impl,
    attrs = _gazelle_runner_attr_factory(),
    executable = True,
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)

_gazelle_test_runner = rule(
    implementation = _gazelle_test_runner_impl,
    attrs = _gazelle_runner_attr_factory(test_runner = True),
    executable = True,
    toolchains = ["@io_bazel_rules_go//go:toolchain"],
)

def _gazelle_kwargs_prepare(name, kwargs):
    if "args" in kwargs:
        # The args attribute has special meaning for executable rules, but we
        # always want extra_args here instead.
        if "extra_args" in kwargs:
            fail("{}: both args and extra_args were provided".format(name))
        kwargs["extra_args"] = kwargs["args"]
        kwargs.pop("args")

    visibility = kwargs.pop("visibility", None)
    return name + "-runner", visibility

def gazelle(name, testonly = False, **kwargs):
    runner_name, visibility = _gazelle_kwargs_prepare(name, kwargs)

    _gazelle_runner(
        name = runner_name,
        testonly = testonly,
        **kwargs
    )
    tags_set = {t: "" for t in kwargs.pop("tags", [])}
    tags_set["manual"] = ""
    tags = tags_set.keys()
    sh_binary(
        name = name,
        tags = tags,
        srcs = [runner_name],
        visibility = visibility,
        testonly = testonly,
        deps = ["@bazel_tools//tools/bash/runfiles"],
        data = kwargs["data"] if "data" in kwargs else [],
    )

def gazelle_test(name, size = None, timeout = None, **kwargs):
    runner_name, visibility = _gazelle_kwargs_prepare(name, kwargs)

    _gazelle_test_runner(
        name = runner_name,
        **kwargs
    )

    # Note: the "external" tag is a workaround for
    # https://github.com/bazelbuild/bazel/issues/15516.
    tags_set = {t: "" for t in kwargs.pop("tags", [])}
    tags_set["no-sandbox"] = ""
    tags_set["no-cache"] = ""
    tags_set["external"] = ""
    tags = tags_set.keys()
    sh_test(
        name = name,
        srcs = [runner_name],
        visibility = visibility,
        tags = tags,
        size = size,
        timeout = timeout,
        deps = ["@bazel_tools//tools/bash/runfiles"],
        data = kwargs["data"] if "data" in kwargs else [],
    )
