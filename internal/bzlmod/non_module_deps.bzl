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

load(
    "@go_host_compatible_sdk_label//:defs.bzl",
    "HOST_COMPATIBLE_SDK",
)
load(
    "//internal:go_repository_cache.bzl",
    "go_repository_cache",
)
load(
    "//internal:go_repository_tools.bzl",
    "go_repository_tools",
)
load(
    "//internal:is_bazel_module.bzl",
    "is_bazel_module",
)
load(
    "//internal/bzlmod:utils.bzl",
    "extension_metadata",
)
load(
    ":go_host_compatible_sdk_label.bzl",
    "go_host_compatible_sdk_label",
)

visibility("//")

def _non_module_deps_impl(module_ctx):
    gazelle_version = ""
    for module in module_ctx.modules:
        if module.name == "gazelle":
            gazelle_version = module.version
            break

    # Work around Stardoc limitation. See doc comment on repo rule.
    go_host_compatible_sdk_label(
        name = "bazel_gazelle_go_host_compatible_sdk_label",
        toolchain = str(HOST_COMPATIBLE_SDK),
    )

    # Shared location for GOCACHE and GOMODCACHE.
    go_repository_cache(
        name = "bazel_gazelle_go_repository_cache",
        # Label.repo_name is always a canonical name, so use a canonical label.
        go_sdk_name = "@" + HOST_COMPATIBLE_SDK.repo_name,
    )

    # Compile gazelle, fetch_repo, and anything else used within go_repository.
    go_repository_tools(
        name = "bazel_gazelle_go_repository_tools",
        go_cache = Label("@bazel_gazelle_go_repository_cache//:go.env"),
        is_bazel_module = True,
        gazelle_version = gazelle_version,
    )

    # Module metadata.
    is_bazel_module(
        name = "bazel_gazelle_is_bazel_module",
        is_bazel_module = True,
        module_version = gazelle_version,
    )
    return extension_metadata(module_ctx, reproducible = True)

non_module_deps = module_extension(
    _non_module_deps_impl,
)
