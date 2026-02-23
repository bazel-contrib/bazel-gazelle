# Copyright 2018 The Bazel Authors. All rights reserved.
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
    "@bazel_gazelle_is_bazel_module//:defs.bzl",
    "GAZELLE_IS_BAZEL_MODULE",
    "GAZELLE_MODULE_VERSION",
)
load(
    "@io_bazel_rules_go//go:def.bzl",
    "GoArchive",
    "go_context",
    "new_go_info",
)

def _gazelle_binary_impl(ctx):
    go = go_context(ctx)

    version = ctx.attr.version
    if version == 0:
        version = _get_gazelle_major_version()
    srcs = ctx.attr._srcs_v2 if version == 2 else ctx.attr._srcs_v1

    # Generate a source file with a list of languages. This will get compiled
    # with the rest of the sources in the main package.
    langs_file = go.declare_file(go, "langs.go")
    langs_content_tpl = """
package main

import (
	"github.com/bazelbuild/bazel-gazelle/language"

	{lang_imports}
)

var languages = []language.Language{{
	{lang_calls},
}}
"""
    lang_imports = [format_import(d[GoArchive].data.importpath) for d in ctx.attr.languages]
    lang_calls = [format_call(d[GoArchive].data.importpath) for d in ctx.attr.languages]
    langs_content = langs_content_tpl.format(
        lang_imports = "\n\t".join(lang_imports),
        lang_calls = ",\n\t".join(lang_calls),
    )
    go.actions.write(langs_file, langs_content)

    # Build the gazelle binary.
    attr = struct(
        srcs = [struct(files = [langs_file])],
        deps = ctx.attr.languages,
        embed = [srcs],
    )
    go_info = new_go_info(go, attr, is_main = True)

    archive, executable, runfiles = go.binary(
        go,
        name = ctx.label.name,
        source = go_info,
        version_file = ctx.version_file,
        info_file = ctx.info_file,
    )

    return [
        go_info,
        archive,
        OutputGroupInfo(compilation_outputs = [archive.data.file]),
        DefaultInfo(
            files = depset([executable]),
            runfiles = runfiles,
            executable = executable,
        ),
    ]

_gazelle_binary_kwargs = {
    "implementation": _gazelle_binary_impl,
    "doc": """The `gazelle_binary` rule builds a Go binary that incorporates a list of
language extensions. This requires generating a small amount of code that
must be compiled into Gazelle's main package, so the normal [go_binary]
rule is not used.

When the binary runs, each language extension is run sequentially. This affects
the order that rules appear in generated build files. Metadata may be produced
by an earlier extension and consumed by a later extension. For example, the
proto extension stores metadata in hidden attributes of generated
`proto_library` rules. The Go extension uses this metadata to generate
`go_proto_library` rules.
""",
    "attrs": {
        "languages": attr.label_list(
            doc = """A list of language extensions the Gazelle binary will use.

            Each extension must be a [go_library] or something compatible. Each extension
            must export a function named `NewLanguage` with no parameters that returns
            a value assignable to [Language].""",
            providers = [GoArchive],
            mandatory = True,
            allow_empty = False,
        ),
        "version": attr.int(
            default = 0,
            values = [0, 1, 2],
            doc = """The major version of Gazelle to build for

            - 0 (default): the version is chosen automatically, based on the
              module version.
            - 1: legacy CLI behavior. Includes update-repos subcommand for Go.
            - 2: new CLI behavior.
            """,
        ),
        "_go_context_data": attr.label(default = "@io_bazel_rules_go//:go_context_data"),
        # _stdlib is needed for rules_go versions before v0.23.0. After that,
        # _go_context_data includes a dependency on stdlib.
        "_stdlib": attr.label(default = "@io_bazel_rules_go//:stdlib"),
        "_srcs_v1": attr.label(
            default = "//cmd/gazelle:gazelle_lib",
        ),
        "_srcs_v2": attr.label(
            default = "//v2/cmd/gazelle:gazelle_lib",
        ),
    },
    "executable": True,
    "toolchains": ["@io_bazel_rules_go//go:toolchain"],
}

gazelle_binary = rule(**_gazelle_binary_kwargs)

def gazelle_binary_wrapper(**kwargs):
    for key in ("goos", "goarch", "static", "msan", "race", "pure", "strip", "debug", "linkmode", "gotags"):
        if key in kwargs:
            fail("gazelle_binary attribute '%s' is no longer supported (https://github.com/bazelbuild/bazel-gazelle/issues/803)" % key)
    gazelle_binary(**kwargs)

def _import_alias(importpath):
    return importpath.replace("/", "_").replace(".", "_").replace("-", "_") + "_"

def format_import(importpath):
    return '{} "{}"'.format(_import_alias(importpath), importpath)

def format_call(importpath):
    return _import_alias(importpath) + ".NewLanguage()"

def _get_gazelle_major_version():
    """Parses a version string and returns either 1 or 2.

    Returns 1 if the major version is 0 or 1 or when building in WORKSPACE mode.
    Very little difference in behavior is expected between these versions.

    Returns 2 if the major version is 2 or unset. When the major version is not
    set, it's assumed to be a development version.
    """
    if not GAZELLE_IS_BAZEL_MODULE:
        return 1
    if not GAZELLE_MODULE_VERSION:
        return 2
    parts = GAZELLE_MODULE_VERSION.split(".", 1)
    if not parts:
        fail("Invalid version format: '{}'".format(GAZELLE_MODULE_VERSION))
    major = parts[0]
    if major == "0" or major == "1":
        return 1
    elif major == "2":
        return 2
    else:
        fail("Unsupported Gazelle major version: {}. Only versions 0, 1, and 2 are supported.".format(major))
