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
    "go_binary",
)

def _gazelle_main_impl(ctx):
    langs_file = ctx.actions.declare_file(ctx.label.name + ".go")
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
    ctx.actions.write(langs_file, langs_content)
    return DefaultInfo(files = depset([langs_file]))

gazelle_main = rule(
    implementation = _gazelle_main_impl,
    attrs = {
        "languages": attr.label_list(
            doc = """A list of language extensions the Gazelle binary will use.

            Each extension must be a [go_library] or something compatible. Each extension
            must export a function named `NewLanguage` with no parameters that returns
            a value assignable to [Language].""",
            providers = [GoArchive],
            mandatory = True,
            allow_empty = False,
        ),
    },
)

def gazelle_binary(name, languages, version = 0, **kwargs):
    """Builds a Gazelle binary with the requested language extensions."""

    if version not in (0, 1, 2):
        fail("gazelle_binary version must be 0, 1, or 2")
    if version == 0:
        version = _get_gazelle_major_version()

    main_name = name + "_main"
    gazelle_main(
        name = main_name,
        languages = languages,
        testonly = kwargs.get("testonly", False),
        visibility = ["//visibility:private"],
    )
    go_binary(
        name = name,
        srcs = [main_name],
        deps = languages,
        embed = [Label("//v2/cmd/gazelle:gazelle_lib") if version == 2 else Label("//cmd/gazelle:gazelle_lib")],
        **kwargs
    )

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
