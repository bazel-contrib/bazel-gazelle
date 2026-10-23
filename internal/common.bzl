# Copyright 2021 The Bazel Authors. All rights reserved.
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

def env_execute(ctx, arguments, environment = {}, **kwargs):
    """Executes a command in for a repository rule.
    It prepends "env -i" to "arguments" before calling "ctx.execute".
    Variables that aren't explicitly mentioned in "environment"
    are removed from the environment. This should be preferred to "ctx.execute"
    in most situations.
    """
    if ctx.os.name.startswith("windows"):
        return ctx.execute(arguments, environment = environment, **kwargs)
    env_args = ["env", "-i"]
    environment = dict(environment)
    for var in ["TMP", "TMPDIR"]:
        if var in ctx.os.environ and not var in environment:
            environment[var] = ctx.os.environ[var]
    for k, v in environment.items():
        env_args.append("%s=%s" % (k, v))
    arguments = env_args + arguments
    return ctx.execute(arguments, **kwargs)

def executable_extension(ctx):
    extension = ""
    if ctx.os.name.startswith("windows"):
        extension = ".exe"
    return extension

def path_str(path):
    """Converts a path to a string, compatible with unit testing"""
    return path._name if hasattr(path, "_name") else str(path)

def watch(ctx, path):
    # Bazel 7.0 watches ctx.path(Label(...)) implicitly and lacks ctx.watch.
    # Newer Bazel versions may require explicit watches.
    if hasattr(ctx, "watch"):
        ctx.watch(path)

def repository_config_input_path(prefix):
    """Maps a Go import prefix to its go_repository_config input file."""

    # Escape case as golang.org/x/mod does, for case-insensitive filesystems.
    if prefix != prefix.lower():
        prefix = "".join(["!" + c.lower() if c.isupper() else c for c in prefix.elems()])

    # Keep the input file distinct from a Go module component named "input".
    return "config_inputs/p-" + prefix.replace("/", "/p-") + "/input"
