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
    "@io_bazel_rules_go//go:def.bzl",
    "GoArchive",
    "go_context",
)

def _gazelle_binary_impl(ctx):
    go = go_context(ctx)

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
    languages = ctx.attr.languages
    lang_imports = [format_import(d[GoArchive].data.importpath) for d in languages]
    lang_calls = [format_call(d[GoArchive].data.importpath) for d in languages]
    langs_content = langs_content_tpl.format(
        lang_imports = "\n\t".join(lang_imports),
        lang_calls = ",\n\t".join(lang_calls),
    )
    go.actions.write(langs_file, langs_content)

    # Build the gazelle binary.
    library = go.new_library(go, is_main = True)
    attr = struct(
        srcs = [struct(files = [langs_file])],
        deps = languages,
        embed = [ctx.attr._srcs],
    )
    source = go.library_to_source(go, attr, library, ctx.coverage_instrumented())

    archive, executable, _ = go.binary(
        go,
        name = ctx.label.name,
        source = source,
        version_file = ctx.version_file,
        info_file = ctx.info_file,
    )

    files = [executable]

    return [
        archive,
        OutputGroupInfo(compilation_outputs = [archive.data.file]),
        DefaultInfo(
            files = depset(files),
            runfiles = ctx.runfiles(files).merge_all(
                [lang[DefaultInfo].default_runfiles for lang in languages]
            ),
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
        "_go_context_data": attr.label(default = "@io_bazel_rules_go//:go_context_data"),
        # _stdlib is needed for rules_go versions before v0.23.0. After that,
        # _go_context_data includes a dependency on stdlib.
        "_stdlib": attr.label(default = "@io_bazel_rules_go//:stdlib"),
        "_srcs": attr.label(
            default = "//cmd/gazelle:gazelle_lib",
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
