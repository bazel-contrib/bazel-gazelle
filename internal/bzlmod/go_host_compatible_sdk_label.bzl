# Copyright 2026 The Bazel Authors. All rights reserved.
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

"""
Re-exports rules_go's host-compatible SDK label with a proper BUILD file.

This is a hack to keep Stardoc working. We would like to load this symbol
directly from go_deps.bzl, but Stardoc generates documentation for go_deps.bzl
and fails with an obscure message because the symbol is loaded from a file
not covered by a bzl_library target. There is no such target of course.
So we create a repo, copy the symbol, and declare a bzl_library, just to
work around Stardoc.
"""

visibility("//")

def _go_host_compatible_sdk_label_impl(repository_ctx):
    repository_ctx.file(
        "def.bzl",
        "HOST_COMPATIBLE_SDK = Label({})\n".format(repr(repository_ctx.attr.toolchain)),
    )
    repository_ctx.file("BUILD.bazel", """\
load("@bazel_skylib//:bzl_library.bzl", "bzl_library")

bzl_library(
    name = "def",
    srcs = ["def.bzl"],
    visibility = ["//visibility:public"],
)
""")

go_host_compatible_sdk_label = repository_rule(
    implementation = _go_host_compatible_sdk_label_impl,
    attrs = {
        "toolchain": attr.string(mandatory = True),
    },
)
