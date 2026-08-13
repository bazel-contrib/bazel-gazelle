# Copyright 2019 The Bazel Authors. All rights reserved.
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

load(":env.bzl", "compute_env", "write_go_env_file")

# Change to trigger cache invalidation: 1

def _go_repository_cache_impl(ctx):
    cache_env = compute_env(
        ctx,
        go_sdk_name = ctx.attr.go_sdk_name,
        go_sdk_info = ctx.attr.go_sdk_info,
        go_env = ctx.attr.go_env,
        go_env_inherit = ctx.attr.go_env_inherit,
    )
    write_go_env_file(ctx, cache_env)

    # HACK: copy HOST_COMPATIBLE_SDK here so that go_deps.bzl doesn't load it.
    # Stardoc generates documentation from go_deps.bzl, and it fails if any
    # symbol is loaded from a .bzl file not provided by a bzl_library. rules_go
    # does not provide a bzl_library for this, so we work around Stardoc's
    # limitations by copying the symbol here and declaring a bzl_library for it.
    # GOROOT is the host Go SDK label, resolved from go_sdk_name or go_sdk_info.
    ctx.file(
        "def.bzl",
        "HOST_COMPATIBLE_SDK = Label({})\n".format(repr(cache_env["GOROOT"])),
    )
    ctx.file("BUILD.bazel", """\
load("@bazel_skylib//:bzl_library.bzl", "bzl_library")

exports_files(["go.env"])

bzl_library(
    name = "def",
    srcs = ["def.bzl"],
    visibility = ["//visibility:public"],
)
""")

go_repository_cache = repository_rule(
    _go_repository_cache_impl,
    attrs = {
        "go_sdk_name": attr.string(),
        "go_sdk_info": attr.string_dict(),
        "go_env": attr.string_dict(),
        "go_env_inherit": attr.string_list(),
    },
)
