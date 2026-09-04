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
    export_files = [d[GoArchive].data.export_file for d in ctx.attr.languages]
    args = ctx.actions.args()
    args.add("-o", langs_file)
    args.add_all(export_files)
    ctx.actions.run(
        outputs = [langs_file],
        inputs = export_files,
        executable = ctx.executable._generator,
        arguments = [args],
        mnemonic = "GazelleMain",
    )
    return DefaultInfo(files = depset([langs_file]))

gazelle_main = rule(
    implementation = _gazelle_main_impl,
    attrs = {
        "languages": attr.label_list(
            doc = """A list of language extensions the Gazelle binary will use.

            Each extension must be a [go_library] or something compatible. Each extension
            must export a function named `NewLanguage` with no parameters that returns
            a value assignable to [Language], or for Gazelle v2, a function named `NewV2`
            that returns a value assignable to [v2/language.Language].""",
            providers = [GoArchive],
            mandatory = True,
            allow_empty = False,
        ),
        "_generator": attr.label(
            default = Label("//cmd/generate_gazelle_binary_languages"),
            cfg = "exec",
            executable = True,
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
    binary_deps = list(languages) + [
        "//v2/compat",
        "//v2/language",
    ]
    go_binary(
        name = name,
        srcs = [main_name],
        deps = binary_deps,
        embed = [Label("//v2/cmd/gazelle:gazelle_lib") if version == 2 else Label("//cmd/gazelle:gazelle_lib")],
        **kwargs
    )

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
