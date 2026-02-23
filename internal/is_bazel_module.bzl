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

def _is_bazel_module_impl(repository_ctx):
    repository_ctx.file("WORKSPACE")
    repository_ctx.file("BUILD", """\
load("@bazel_skylib//:bzl_library.bzl", "bzl_library")

bzl_library(
    name = "defs",
    srcs = ["defs.bzl"],
    visibility = ["//visibility:public"],
)
""")
    repository_ctx.file("defs.bzl", """\
GAZELLE_IS_BAZEL_MODULE = {}
GAZELLE_MODULE_VERSION = "{}"
""".format(
        repr(repository_ctx.attr.is_bazel_module),
        repository_ctx.attr.module_version,
    ))

is_bazel_module = repository_rule(
    implementation = _is_bazel_module_impl,
    attrs = {
        "is_bazel_module": attr.bool(mandatory = True),
        "module_version": attr.string(default = ""),
    },
)
