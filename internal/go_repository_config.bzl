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

load("//internal:common.bzl", "env_execute", "executable_extension", "watch")
load("//internal:go_repository_cache.bzl", "read_cache_env")

def _go_repository_config_impl(ctx):
    # Locate and resolve configuration files. Gazelle reads directives and
    # known repositories from these files. Resolving them here forces the
    # go_repository_config rule to be invalidated when they change. Gazelle's cache
    # should NOT be invalidated, so we shouldn't need to download these again.
    config_path = None
    if ctx.attr.config:
        config_path = ctx.path(ctx.attr.config)

    if config_path:
        watch(ctx, config_path)
        env = read_cache_env(ctx, ctx.path(Label("@bazel_gazelle_go_repository_cache//:go.env")))
        generate_repo_config = ctx.path(Label("@bazel_gazelle_go_repository_tools//:bin/generate_repo_config{}".format(executable_extension(ctx))))
        watch(ctx, generate_repo_config)
        list_repos_args = [
            "-config_source=" + str(config_path),
            "-config_dest=" + str(ctx.path("WORKSPACE")),
        ]
        result = env_execute(
            ctx,
            [str(generate_repo_config)] + list_repos_args,
            environment = env,
        )
        if result.return_code:
            fail("generate_repo_config: " + result.stderr)
        if result.stdout:
            for f in result.stdout.splitlines():
                f = f.lstrip()
                if len(f) > 0:
                    # Reuse the repo prefix of the stringified label to use a
                    # canonical label literal on Bazel 6 and higher.
                    config_label = str(ctx.attr.config)
                    macro_label_prefix = config_label[:config_label.find("//")]
                    macro_label_str = macro_label_prefix + "//:" + f
                    watch(ctx, ctx.path(Label(macro_label_str)))

    else:
        ctx.file(
            "WORKSPACE",
            "",
            False,
        )

    # add empty go_env.bzl and go_tools.bzl files for compatibility with bzlmod
    ctx.file(
        "go_env.bzl",
        "GO_ENV = {}",
        False,
    )
    ctx.file(
        "go_tools.bzl",
        "TOOLS = {}",
        False,
    )

    # add an empty build file so Bazel recognizes the config
    ctx.file(
        "BUILD.bazel",
        "exports_files([\"go_env.bzl\", \"go_tools.bzl\", \"WORKSPACE\"])",
        False,
    )

go_repository_config = repository_rule(
    implementation = _go_repository_config_impl,
    attrs = {
        "config": attr.label(),
    },
)
